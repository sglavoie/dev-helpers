import AppKit
import Carbon.HIToolbox

/// A global shortcut: a virtual key code plus Carbon modifier flags.
struct HotKey: Codable, Hashable, Sendable {
    var keyCode: UInt32
    var carbonModifiers: UInt32

    /// ⌃⌥⌘V
    static let `default` = HotKey(
        keyCode: UInt32(kVK_ANSI_V),
        carbonModifiers: UInt32(controlKey | optionKey | cmdKey)
    )
}

/// Registers the app's global shortcuts. Behind a protocol so the Carbon
/// implementation can be swapped later.
@MainActor
protocol HotKeyCenter: AnyObject {
    /// Replaces any existing registration with the given one.
    func register(_ hotKey: HotKey, handler: @escaping @MainActor () -> Void) throws
    func unregister()
}

struct HotKeyRegistrationError: Error, CustomStringConvertible {
    var status: OSStatus
    var description: String { "RegisterEventHotKey failed with status \(status)" }
}

/// `RegisterEventHotKey` needs no Accessibility permission and no dependencies.
@MainActor
final class CarbonHotKeyCenter: HotKeyCenter {
    private static let signature: OSType = 0x534C_5050  // "SLPP"
    /// The C event handler cannot capture context, so it dispatches through this.
    private static var handlers: [UInt32: @MainActor () -> Void] = [:]
    private static var nextID: UInt32 = 1
    private static var eventHandler: EventHandlerRef?

    private var hotKeyRef: EventHotKeyRef?
    private var hotKeyID: UInt32?

    init() {
        Self.installEventHandlerIfNeeded()
    }

    func register(_ hotKey: HotKey, handler: @escaping @MainActor () -> Void) throws {
        unregister()
        let id = Self.nextID
        Self.nextID += 1

        var ref: EventHotKeyRef?
        let status = RegisterEventHotKey(
            hotKey.keyCode,
            hotKey.carbonModifiers,
            EventHotKeyID(signature: Self.signature, id: id),
            GetApplicationEventTarget(),
            0,
            &ref
        )
        guard status == noErr, let ref else {
            throw HotKeyRegistrationError(status: status)
        }
        hotKeyRef = ref
        hotKeyID = id
        Self.handlers[id] = handler
    }

    func unregister() {
        if let hotKeyRef {
            UnregisterEventHotKey(hotKeyRef)
        }
        if let hotKeyID {
            Self.handlers[hotKeyID] = nil
        }
        hotKeyRef = nil
        hotKeyID = nil
    }

    private static func installEventHandlerIfNeeded() {
        guard eventHandler == nil else { return }
        var eventType = EventTypeSpec(
            eventClass: OSType(kEventClassKeyboard),
            eventKind: UInt32(kEventHotKeyPressed)
        )
        InstallEventHandler(
            GetApplicationEventTarget(),
            { _, event, _ -> OSStatus in
                var hotKeyID = EventHotKeyID()
                let status = GetEventParameter(
                    event,
                    EventParamName(kEventParamDirectObject),
                    EventParamType(typeEventHotKeyID),
                    nil,
                    MemoryLayout<EventHotKeyID>.size,
                    nil,
                    &hotKeyID
                )
                guard status == noErr, hotKeyID.signature == CarbonHotKeyCenter.signature else {
                    return OSStatus(eventNotHandledErr)
                }
                let id = hotKeyID.id
                // Carbon delivers application-target events on the main thread.
                return MainActor.assumeIsolated {
                    guard let handler = CarbonHotKeyCenter.handlers[id] else {
                        return OSStatus(eventNotHandledErr)
                    }
                    handler()
                    return noErr
                }
            },
            1,
            &eventType,
            nil,
            &eventHandler
        )
    }
}
