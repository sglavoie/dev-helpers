// ddc-brightnessd -- resident DDC/CI brightness applier for Apple Silicon.
//
// The ddc-brightness shell script enqueues one small file per keypress; this
// daemon drains that queue and writes the summed level to the monitor. It
// exists purely to remove per-keypress overhead: invoking m1ddc costs ~74ms,
// of which only ~30ms is the DDC write itself. The rest is process startup and
// re-enumerating the IORegistry to find the display. Holding the IOAVService
// handle open across keypresses removes all of it.
//
// The write policy deliberately matches m1ddc's (two sends, 10ms apart): this
// panel drops writes that are issued closer together, and its DDC reads are
// unreliable enough -- they intermittently fail outright or return a stale
// value -- that no faster policy could be verified from software. Redundancy
// is cheap here and a dropped brightness write is not.
//
// The DDC and IORegistry specifics are adapted from m1ddc (MIT, waydabber).
//
// Queue protocol: each entry is an empty file whose name ends in ".<tag>",
//   pNN  relative +NN     mNN  relative -NN     aNN  absolute NN
// The name carries the payload so an entry is never observed half-written.

@import Foundation;
@import IOKit;
@import CoreGraphics;

typedef CFTypeRef IOAVServiceRef;
extern IOAVServiceRef IOAVServiceCreateWithService(CFAllocatorRef, io_service_t);
extern IOReturn IOAVServiceWriteI2C(IOAVServiceRef, uint32_t, uint32_t, void *, uint32_t);
extern CFDictionaryRef CoreDisplay_DisplayCreateInfoDictionary(CGDirectDisplayID);

#define CHIP_ADDR       0x37
#define INPUT_ADDR      0x51
#define VCP_LUMINANCE   0x10
#define WRITE_ITERS     2
#define WRITE_WAIT_US   10000
#define MIN_WRITE_GAP_NS (40ull * NSEC_PER_MSEC)
#define SETTLE_DELAY_S  0.20
#define SAFETY_POLL_S   2.0
#define MAX_DISPLAYS    8
#define STR_EQ(a, b)    (strcmp(a, b) == 0)

static NSString *gDisplayMatch = @"PA27JCV";
static NSString *gStateDir;
static NSString *gQueueDir;
static NSString *gLevelFile;
static NSString *gPidFile;
static IOAVServiceRef gService;
static int gLevel = -1;
static uint64_t gLastWriteNs = 0;
static uint64_t gWriteSeq = 0;

static void logmsg(NSString *fmt, ...) {
    va_list ap;
    va_start(ap, fmt);
    NSString *msg = [[NSString alloc] initWithFormat:fmt arguments:ap];
    va_end(ap);
    fprintf(stderr, "%s\n", msg.UTF8String);
    fflush(stderr);
}

#pragma mark - display

static IOAVServiceRef findService(NSString *want) {
    CGDirectDisplayID list[MAX_DISPLAYS];
    CGDisplayCount count = 0;
    CGGetOnlineDisplayList(MAX_DISPLAYS, list, &count);

    for (CGDisplayCount i = 0; i < count; i++) {
        CFDictionaryRef info = CoreDisplay_DisplayCreateInfoDictionary(list[i]);
        if (!info) continue;
        NSString *loc = (__bridge NSString *)CFDictionaryGetValue(info, CFSTR("IODisplayLocation"));
        if (!loc) { CFRelease(info); continue; }
        io_service_t adapter = IORegistryEntryCopyFromPath(kIOMainPortDefault,
                                                           (__bridge CFStringRef)loc);
        CFRelease(info);
        if (adapter == MACH_PORT_NULL) continue;

        CFTypeRef attrs = IORegistryEntrySearchCFProperty(adapter, kIOServicePlane,
            CFSTR("DisplayAttributes"), kCFAllocatorDefault, kIORegistryIterateRecursively);
        NSString *name = attrs ? [[(__bridge NSDictionary *)attrs
            objectForKey:@"ProductAttributes"] objectForKey:@"ProductName"] : nil;
        BOOL match = name && [name rangeOfString:want].location != NSNotFound;
        if (attrs) CFRelease(attrs);
        if (!match) { IOObjectRelease(adapter); continue; }

        uint64_t adapterID = 0;
        IORegistryEntryGetRegistryEntryID(adapter, &adapterID);
        IOObjectRelease(adapter);

        // The IOAVService lives on the DCPAVServiceProxy under the framebuffer
        // belonging to this display, so track which framebuffer we are inside.
        io_iterator_t iter;
        if (IORegistryEntryCreateIterator(IORegistryGetRootEntry(kIOMainPortDefault),
                kIOServicePlane, kIORegistryIterateRecursively, &iter) != KERN_SUCCESS) {
            return NULL;
        }
        BOOL inMatchingFramebuffer = NO;
        io_service_t svc;
        while ((svc = IOIteratorNext(iter)) != MACH_PORT_NULL) {
            if (IOObjectConformsTo(svc, "IOMobileFramebuffer")) {
                uint64_t fbID = 0;
                inMatchingFramebuffer =
                    IORegistryEntryGetRegistryEntryID(svc, &fbID) == KERN_SUCCESS &&
                    fbID == adapterID;
                IOObjectRelease(svc);
                continue;
            }
            io_name_t nm;
            IORegistryEntryGetName(svc, nm);
            if (!inMatchingFramebuffer || !STR_EQ(nm, "DCPAVServiceProxy")) {
                IOObjectRelease(svc);
                continue;
            }
            IOAVServiceRef av = IOAVServiceCreateWithService(kCFAllocatorDefault, svc);
            IOObjectRelease(svc);
            if (av) { IOObjectRelease(iter); return av; }
        }
        IOObjectRelease(iter);
    }
    return NULL;
}

static BOOL ensureService(void) {
    if (gService) return YES;
    gService = findService(gDisplayMatch);
    if (gService) logmsg(@"attached to display matching \"%@\"", gDisplayMatch);
    return gService != NULL;
}

static void dropService(void) {
    if (gService) { CFRelease(gService); gService = NULL; }
}

static BOOL gDebug = NO;
static uint64_t nowNs(void);

static BOOL writeLuminance(int value) {
    if (!ensureService()) return NO;
    if (gDebug) logmsg(@"%8.3f write %d", (double)nowNs() / 1e9, value);
    uint8_t d[6] = {0x84, 0x03, VCP_LUMINANCE, 0x00, (uint8_t)value, 0x00};
    d[5] = 0x6E ^ INPUT_ADDR ^ d[0] ^ d[1] ^ d[2] ^ d[3] ^ d[4];
    for (int i = 0; i < WRITE_ITERS; i++) {
        usleep(WRITE_WAIT_US);
        if (IOAVServiceWriteI2C(gService, CHIP_ADDR, INPUT_ADDR, d, sizeof(d))) {
            // The handle goes stale when the display sleeps or is replugged;
            // drop it so the next attempt re-resolves.
            logmsg(@"write failed, dropping service handle");
            dropService();
            return NO;
        }
    }
    return YES;
}

#pragma mark - state

static void loadLevel(void) {
    NSString *s = [NSString stringWithContentsOfFile:gLevelFile
                                            encoding:NSUTF8StringEncoding error:NULL];
    int v = s ? s.intValue : -1;
    gLevel = (v >= 0 && v <= 100) ? v : 50;
}

static void saveLevel(void) {
    [[NSString stringWithFormat:@"%d\n", gLevel] writeToFile:gLevelFile atomically:YES
                                                    encoding:NSUTF8StringEncoding error:NULL];
}

#pragma mark - queue

// Drains every queued entry, folding it into `base`. Returns YES if anything
// was pending, and reports the resulting target level through *target.
static BOOL drainQueueFrom(int base, int *target) {
    NSFileManager *fm = NSFileManager.defaultManager;
    NSArray<NSString *> *names = [fm contentsOfDirectoryAtPath:gQueueDir error:NULL];
    if (names.count == 0) return NO;

    // Sort so absolute requests and relative deltas apply in creation order;
    // the shell script prefixes every name with a monotonic counter.
    names = [names sortedArrayUsingSelector:@selector(compare:)];

    int level = base;
    BOOL any = NO;
    for (NSString *name in names) {
        NSString *tag = name.pathExtension;
        if (tag.length < 2) {
            [fm removeItemAtPath:[gQueueDir stringByAppendingPathComponent:name] error:NULL];
            continue;
        }
        int n = [tag substringFromIndex:1].intValue;
        switch ([tag characterAtIndex:0]) {
            case 'p': level += n; any = YES; break;
            case 'm': level -= n; any = YES; break;
            case 'a': level = n;  any = YES; break;
            default: break;
        }
        [fm removeItemAtPath:[gQueueDir stringByAppendingPathComponent:name] error:NULL];
    }
    if (!any) return NO;
    if (level < 0) level = 0;
    if (level > 100) level = 100;
    *target = level;
    return YES;
}

static uint64_t nowNs(void) {
    return clock_gettime_nsec_np(CLOCK_MONOTONIC);
}

// Re-sends the current level once the bus has been quiet for a moment.
//
// This panel drops writes issued in quick succession -- measured worst during
// a direction change mid-slide, where roughly one run in three ended a few
// steps off target. Sending the settled value again once nothing else is
// competing for the bus is invisible to the user and fixes those misses. The
// sequence check makes a settle that was overtaken by new input a no-op.
static void scheduleSettle(void) {
    uint64_t seq = gWriteSeq;
    dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(SETTLE_DELAY_S * NSEC_PER_SEC)),
                   dispatch_get_main_queue(), ^{
        @autoreleasepool {
            if (gWriteSeq != seq || gLevel < 0) return;
            writeLuminance(gLevel);
        }
    });
}

// Applies pending work, then keeps draining while more arrives, so a slide
// collapses into as few writes as the DDC bus allows.
static void pump(void) {
    int target;
    BOOL wrote = NO;
    while (drainQueueFrom(gLevel, &target)) {
        if (target == gLevel) continue;

        // Pace consecutive writes. Ticks arriving during the pause are folded
        // into the same write, so pacing costs no accuracy -- it just makes a
        // fast slide move in fewer, larger steps.
        uint64_t since = nowNs() - gLastWriteNs;
        if (gLastWriteNs && since < MIN_WRITE_GAP_NS) {
            usleep((useconds_t)((MIN_WRITE_GAP_NS - since) / NSEC_PER_USEC));
            int newer;
            if (drainQueueFrom(target, &newer)) target = newer;
            if (target == gLevel) continue;
        }

        if (!writeLuminance(target) && !writeLuminance(target)) {
            // Retried once with a freshly resolved handle; the queue is
            // already drained, so there is nothing left to retry from.
            logmsg(@"giving up on level %d", target);
            continue;
        }
        gLastWriteNs = nowNs();
        gWriteSeq++;
        gLevel = target;
        saveLevel();
        wrote = YES;
    }
    if (wrote) scheduleSettle();
}

#pragma mark - main

static void onDisplayReconfig(CGDirectDisplayID d, CGDisplayChangeSummaryFlags flags, void *ctx) {
    (void)d; (void)ctx;
    if (flags & (kCGDisplayAddFlag | kCGDisplayRemoveFlag | kCGDisplayDisabledFlag)) {
        logmsg(@"display reconfigured, releasing service handle");
        dropService();
    }
}

int main(int argc, char **argv) {
    @autoreleasepool {
        gDebug = getenv("DDC_DEBUG") != NULL;
        const char *env = getenv("DDC_DISPLAY");
        if (env && *env) gDisplayMatch = @(env);
        if (argc > 1) gDisplayMatch = @(argv[1]);

        NSString *cache = NSProcessInfo.processInfo.environment[@"XDG_CACHE_HOME"];
        if (!cache.length) cache = [NSHomeDirectory() stringByAppendingPathComponent:@".cache"];
        gStateDir = [[cache stringByAppendingPathComponent:@"ddc-brightness"]
                        stringByAppendingPathComponent:gDisplayMatch];
        gQueueDir = [gStateDir stringByAppendingPathComponent:@"queue"];
        gLevelFile = [gStateDir stringByAppendingPathComponent:@"level"];
        gPidFile = [gStateDir stringByAppendingPathComponent:@"daemon.pid"];
        [NSFileManager.defaultManager createDirectoryAtPath:gQueueDir
                                withIntermediateDirectories:YES attributes:nil error:NULL];

        // The shell script checks this to decide whether it must run its own
        // fallback applier. A stale file left by a crash is harmless: the pid
        // will not be live, so the script falls back.
        [[NSString stringWithFormat:@"%d\n", getpid()] writeToFile:gPidFile atomically:YES
                                                         encoding:NSUTF8StringEncoding error:NULL];
        atexit_b(^{ [NSFileManager.defaultManager removeItemAtPath:gPidFile error:NULL]; });

        loadLevel();
        ensureService();
        CGDisplayRegisterReconfigurationCallback(onDisplayReconfig, NULL);

        int dirFD = open(gQueueDir.fileSystemRepresentation, O_EVTONLY);
        if (dirFD < 0) { logmsg(@"cannot watch %@", gQueueDir); return 1; }

        dispatch_queue_t q = dispatch_get_main_queue();
        dispatch_source_t watch = dispatch_source_create(
            DISPATCH_SOURCE_TYPE_VNODE, dirFD,
            DISPATCH_VNODE_WRITE | DISPATCH_VNODE_EXTEND, q);
        dispatch_source_set_event_handler(watch, ^{ @autoreleasepool { pump(); } });
        dispatch_source_set_cancel_handler(watch, ^{ close(dirFD); });
        dispatch_resume(watch);

        // kqueue coalesces rapid directory changes and can miss an entry that
        // lands mid-notification, so sweep periodically as a backstop.
        dispatch_source_t timer = dispatch_source_create(DISPATCH_SOURCE_TYPE_TIMER, 0, 0, q);
        dispatch_source_set_timer(timer, dispatch_time(DISPATCH_TIME_NOW, 0),
                                  (uint64_t)(SAFETY_POLL_S * NSEC_PER_SEC),
                                  (uint64_t)(0.5 * NSEC_PER_SEC));
        dispatch_source_set_event_handler(timer, ^{ @autoreleasepool { pump(); } });
        dispatch_resume(timer);

        logmsg(@"watching %@", gQueueDir);
        dispatch_main();
    }
    return 0;
}
