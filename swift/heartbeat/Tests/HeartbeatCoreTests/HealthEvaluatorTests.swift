import Foundation
import Testing
@testable import HeartbeatCore

@Suite struct HealthEvaluatorTests {
    static func local(_ year: Int, _ month: Int, _ day: Int, _ hour: Int, _ minute: Int, _ second: Int = 0) -> Date {
        TimeFixtures.local(year, month, day, hour, minute, second)
    }
    static func utc(_ text: String) -> Date { TimeFixtures.utc(text) }

    /// Noon on Saturday 2026-09-26; the Mac booted six days earlier and has not slept.
    static let now = local(2026, 9, 26, 12, 0)
    static let boot = local(2026, 9, 20, 8, 0)
    static let minute: TimeInterval = 60
    static let hour: TimeInterval = 3600

    static func ago(_ seconds: TimeInterval) -> Date { now.addingTimeInterval(-seconds) }

    static func agent(_ schedule: Schedule = .none, keepAlive: KeepAlivePolicy = .none, disabled: Bool = false) -> AgentDefinition {
        AgentDefinition(label: "com.sglavoie.test", plistPath: "/tmp/com.sglavoie.test.plist", schedule: schedule,
                        keepAlive: keepAlive, disabled: disabled, standardOutPath: "/tmp/test.log")
    }

    static func loaded(pid: Int32? = nil, lastExit: LastExit? = .exited(code: 0), runs: Int = 3) -> ServiceStatus {
        .loaded(ServiceRuntime(state: pid == nil ? .notRunning : .running, pid: pid, runs: runs, lastExit: lastExit))
    }

    static func evidence(_ date: Date?) -> Evidence { Evidence(hasSource: true, latest: date) }

    static let daily10 = Schedule.calendar([CalendarEntry(minute: 0, hour: 10)])
    static let every15 = Schedule.interval(seconds: 900)
    static let daemon = KeepAlivePolicy.always
    static let untilSuccess = KeepAlivePolicy.conditional(successfulExit: false, otherKeys: [])

    struct Case: CustomTestStringConvertible, Sendable {
        var name: String
        var input: HealthInput
        var now: Date = HealthEvaluatorTests.now
        var power = PowerTimeline(bootTime: HealthEvaluatorTests.boot)
        var severity: Severity
        /// Reason codes, worst first.
        var codes: [HealthReasonCode]
        var summary: String? = nil
        var testDescription: String { name }
    }

    static func input(
        _ agent: AgentDefinition = agent(), _ status: ServiceStatus = loaded(), evidence: Evidence = .none,
        options: AgentHealthOptions = AgentHealthOptions(), history: AgentHistory = AgentHistory()
    ) -> HealthInput {
        HealthInput(agent: agent, status: status, evidence: evidence, options: options, history: history)
    }

    // MARK: Rules 1-3: visibility and load state

    static let loadCases: [Case] = [
        Case(name: "1: hidden wins over everything", input: input(agent(), loaded(lastExit: .exited(code: 1)),
             options: AgentHealthOptions(hidden: true)), severity: .hidden, codes: [.hidden]),
        Case(name: "2: not loaded + Disabled in plist is paused", input: input(agent(disabled: true), .notLoaded),
             severity: .paused, codes: [.paused], summary: "Disabled in plist"),
        Case(name: "2: not loaded + paused by Heartbeat is paused", input: input(agent(), .notLoaded,
             options: AgentHealthOptions(pausedByHeartbeat: true)),
             severity: .paused, codes: [.paused], summary: "Paused (unloaded by Heartbeat)"),
        Case(name: "3: not loaded otherwise is amber", input: input(agent(every15), .notLoaded),
             severity: .warning, codes: [.notLoaded], summary: "Not loaded"),
        Case(name: "3: unparseable print output is amber", input: input(agent(), .unknown(reason: "missing state"),
             evidence: evidence(ago(1))), severity: .warning, codes: [.unreadableState],
             summary: "Cannot read launchctl state: missing state"),
        Case(name: "loaded, Disabled key ignored", input: input(agent(disabled: true)), severity: .ok, codes: []),
    ]

    // MARK: Rule 4: last exit

    static let exitCases: [Case] = [
        Case(name: "4: exit 0 is ok", input: input(), severity: .ok, codes: []),
        Case(name: "4: exit 1 is red", input: input(agent(), loaded(lastExit: .exited(code: 1))),
             severity: .failing, codes: [.exitCode], summary: "Exited with status 1"),
        Case(name: "4: ignored exit code is ok", input: input(agent(), loaded(lastExit: .exited(code: 2)),
             options: AgentHealthOptions(ignoreExitCodes: [2])), severity: .ok, codes: []),
        Case(name: "4: never exited is neither failure nor evidence", input: input(agent(), loaded(lastExit: .neverExited, runs: 0)),
             severity: .ok, codes: []),
        Case(name: "4: no exit line at all", input: input(agent(), loaded(lastExit: nil)), severity: .ok, codes: []),
        Case(name: "4: signal is red", input: input(agent(), loaded(lastExit: .signaled(signal: 15, name: "Terminated"))),
             severity: .failing, codes: [.signal], summary: "Killed by signal Terminated (15)"),
        Case(name: "4: running non-KeepAlive job with a failed last run is red",
             input: input(agent(every15), loaded(pid: 42, lastExit: .exited(code: 1)), evidence: evidence(ago(minute))),
             severity: .failing, codes: [.exitCode]),
        Case(name: "4: KeepAlive daemon restarted 10 min ago is amber",
             input: input(agent(keepAlive: daemon), loaded(pid: 42, lastExit: .exited(code: 1)),
                          history: AgentHistory(lastRestartAt: ago(10 * minute))),
             severity: .warning, codes: [.restarted], summary: "Restarted after exit 1"),
        Case(name: "4: KeepAlive daemon restarted after a signal is amber",
             input: input(agent(keepAlive: daemon), loaded(pid: 42, lastExit: .signaled(signal: 9, name: "Killed")),
                          history: AgentHistory(lastRestartAt: ago(59 * minute))),
             severity: .warning, codes: [.restarted], summary: "Restarted after signal Killed (9)"),
        Case(name: "4: KeepAlive daemon restarted over an hour ago is ok",
             input: input(agent(keepAlive: daemon), loaded(pid: 42, lastExit: .exited(code: 1)),
                          history: AgentHistory(lastRestartAt: ago(61 * minute))),
             severity: .ok, codes: []),
        Case(name: "4: KeepAlive daemon with unknown restart time is ok",
             input: input(agent(keepAlive: daemon), loaded(pid: 42, lastExit: .exited(code: 1))),
             severity: .ok, codes: []),
        Case(name: "4+5: KeepAlive daemon down after a failure: red exit, then amber not running",
             input: input(agent(keepAlive: daemon), loaded(lastExit: .exited(code: 1))),
             severity: .failing, codes: [.exitCode, .notRunning]),
    ]

    // MARK: Rule 5: KeepAlive without a PID

    static let keepAliveCases: [Case] = [
        Case(name: "5: KeepAlive running is ok", input: input(agent(keepAlive: daemon), loaded(pid: 42, lastExit: .neverExited)),
             severity: .ok, codes: []),
        Case(name: "5: KeepAlive without PID, first poll, is amber", input: input(agent(keepAlive: daemon), loaded()),
             severity: .warning, codes: [.notRunning], summary: "Not running (KeepAlive)"),
        Case(name: "5: KeepAlive without PID, second poll, is red", input: input(agent(keepAlive: daemon), loaded(),
             history: AgentHistory(previousNotRunningPolls: 1)), severity: .failing, codes: [.notRunning]),
        Case(name: "5: SuccessfulExit false that exited 0 is expected running",
             input: input(agent(keepAlive: untilSuccess), loaded(lastExit: .exited(code: 0))),
             severity: .warning, codes: [.notRunning]),
        Case(name: "5: SuccessfulExit true is not expected running",
             input: input(agent(keepAlive: .conditional(successfulExit: true, otherKeys: [])), loaded()),
             severity: .ok, codes: []),
        Case(name: "5: config can turn the expectation off", input: input(agent(keepAlive: daemon), loaded(),
             options: AgentHealthOptions(expectsRunning: false)), severity: .ok, codes: []),
        Case(name: "5: config can turn the expectation on", input: input(agent(), loaded(),
             options: AgentHealthOptions(expectsRunning: true), history: AgentHistory(previousNotRunningPolls: 3)),
             severity: .failing, codes: [.notRunning]),
    ]

    // MARK: Rule 6: interval / maxAgeSeconds

    static let intervalCases: [Case] = [
        Case(name: "6i: recent evidence is ok", input: input(agent(every15), evidence: evidence(ago(10 * minute))),
             severity: .ok, codes: []),
        Case(name: "6i: just inside max(2I, I+300) = 30 min", input: input(agent(every15), evidence: evidence(ago(30 * minute))),
             severity: .ok, codes: []),
        Case(name: "6i: older than 30 min while awake is red", input: input(agent(every15), evidence: evidence(ago(31 * minute))),
             severity: .failing, codes: [.overdue], summary: "Overdue: last run 31 min ago (allowed 30 min)"),
        Case(name: "6i: woke 20 min ago, not awake long enough yet", input: input(agent(every15), evidence: evidence(ago(5 * hour))),
             power: PowerTimeline(bootTime: boot, lastWake: ago(20 * minute)), severity: .ok, codes: []),
        Case(name: "6i: woke 40 min ago and still nothing is red", input: input(agent(every15), evidence: evidence(ago(5 * hour))),
             power: PowerTimeline(bootTime: boot, lastWake: ago(40 * minute)), severity: .failing, codes: [.overdue]),
        Case(name: "6i: booted 20 min ago", input: input(agent(every15), evidence: evidence(ago(3 * 86400))),
             power: PowerTimeline(bootTime: ago(20 * minute)), severity: .ok, codes: []),
        Case(name: "6i: loaded 10 min ago", input: input(agent(every15), evidence: evidence(ago(5 * hour)),
             history: AgentHistory(loadedObservedAt: ago(10 * minute))), severity: .ok, codes: []),
        Case(name: "6i: running now is never overdue", input: input(agent(every15), loaded(pid: 7), evidence: evidence(ago(5 * hour))),
             severity: .ok, codes: []),
        Case(name: "6i: sources but no timestamp yet is red", input: input(agent(every15), evidence: evidence(nil)),
             severity: .failing, codes: [.overdue], summary: "Overdue: no run recorded"),
        Case(name: "6i: no evidence source is amber", input: input(agent(every15)),
             severity: .warning, codes: [.cannotVerify], summary: "Cannot verify last run"),
        Case(name: "6i: hourly allows 2I = 2 h", input: input(agent(.interval(seconds: 3600)), evidence: evidence(ago(119 * minute))),
             severity: .ok, codes: []),
        Case(name: "6i: hourly red after 2 h", input: input(agent(.interval(seconds: 3600)), evidence: evidence(ago(121 * minute))),
             severity: .failing, codes: [.overdue]),
        Case(name: "6i: every minute allows I+300 = 6 min", input: input(agent(.interval(seconds: 60)), evidence: evidence(ago(5 * minute))),
             severity: .ok, codes: []),
        Case(name: "6i: every minute red after 6 min", input: input(agent(.interval(seconds: 60)), evidence: evidence(ago(7 * minute))),
             severity: .failing, codes: [.overdue]),
        Case(name: "6i: maxAgeSeconds overrides the interval", input: input(agent(every15), evidence: evidence(ago(5 * minute)),
             options: AgentHealthOptions(maxAgeSeconds: 60)), severity: .failing, codes: [.overdue]),
        Case(name: "6i: maxAgeSeconds on a KeepAlive-only agent", input: input(agent(), evidence: evidence(ago(2 * hour)),
             options: AgentHealthOptions(maxAgeSeconds: 3600)), severity: .failing, codes: [.overdue]),
        Case(name: "6i: maxAgeSeconds replaces the calendar check", input: input(agent(daily10), evidence: evidence(ago(1 * hour)),
             options: AgentHealthOptions(maxAgeSeconds: 2700)), severity: .failing, codes: [.overdue]),
        Case(name: "6: no schedule and no maxAge has no overdue rule", input: input(agent()), severity: .ok, codes: []),
        Case(name: "6: WatchPaths has no overdue rule", input: input(agent(.watchPaths(["/tmp/x"], throttleSeconds: 30))),
             severity: .ok, codes: []),
        Case(name: "4+6: worst wins, rule order breaks ties",
             input: input(agent(every15), loaded(lastExit: .exited(code: 1)), evidence: evidence(ago(2 * hour))),
             severity: .failing, codes: [.exitCode, .overdue], summary: "Exited with status 1"),
    ]

    // MARK: Rule 6: calendar (daily 10:00, now 12:00)

    static let calendarCases: [Case] = [
        Case(name: "6c: ran at the slot", input: input(agent(daily10), evidence: evidence(local(2026, 9, 26, 10, 0, 30))),
             severity: .ok, codes: []),
        Case(name: "6c: evidence within 60 s before the slot counts", input: input(agent(daily10),
             evidence: evidence(local(2026, 9, 26, 9, 59, 30))), severity: .ok, codes: []),
        Case(name: "6c: evidence from yesterday is red after grace", input: input(agent(daily10),
             evidence: evidence(local(2026, 9, 25, 10, 0, 5))), severity: .failing, codes: [.overdue],
             summary: "Overdue: missed the Sep 26 10:00 run"),
        Case(name: "6c: still within the 15 min grace", input: input(agent(daily10), evidence: evidence(local(2026, 9, 25, 10, 0, 5))),
             now: local(2026, 9, 26, 10, 14), severity: .ok, codes: []),
        Case(name: "6c: grace is configurable", input: input(agent(daily10), evidence: evidence(local(2026, 9, 25, 10, 0, 5)),
             options: AgentHealthOptions(graceSeconds: 3 * 3600)), severity: .ok, codes: []),
        Case(name: "6c: powered off at the slot (booted after it)", input: input(agent(daily10),
             evidence: evidence(local(2026, 9, 24, 10, 0, 5))),
             power: PowerTimeline(bootTime: local(2026, 9, 26, 11, 0)), severity: .ok, codes: []),
        Case(name: "6c: loaded after the slot", input: input(agent(daily10), evidence: evidence(local(2026, 9, 25, 10, 0, 5)),
             history: AgentHistory(loadedObservedAt: local(2026, 9, 26, 11, 0))), severity: .ok, codes: []),
        Case(name: "6c: slept across the slot, woke 10 min ago", input: input(agent(daily10),
             evidence: evidence(local(2026, 9, 25, 10, 0, 5))),
             power: PowerTimeline(bootTime: boot, lastWake: local(2026, 9, 26, 11, 50)), severity: .ok, codes: []),
        Case(name: "6c: slept across the slot, no run 15 min after wake is red", input: input(agent(daily10),
             evidence: evidence(local(2026, 9, 25, 10, 0, 5))), now: local(2026, 9, 26, 12, 6),
             power: PowerTimeline(bootTime: boot, lastWake: local(2026, 9, 26, 11, 50)), severity: .failing, codes: [.overdue]),
        Case(name: "6c: slept across the slot, coalesced run on wake is ok", input: input(agent(daily10),
             evidence: evidence(local(2026, 9, 26, 11, 50, 20))), now: local(2026, 9, 26, 12, 30),
             power: PowerTimeline(bootTime: boot, lastWake: local(2026, 9, 26, 11, 50)), severity: .ok, codes: []),
        Case(name: "6c: a wake before the slot doesn't extend the deadline", input: input(agent(daily10),
             evidence: evidence(local(2026, 9, 25, 10, 0, 5))),
             power: PowerTimeline(bootTime: boot, lastWake: local(2026, 9, 26, 8, 0)), severity: .failing, codes: [.overdue]),
        Case(name: "6c: running now", input: input(agent(daily10), loaded(pid: 9), evidence: evidence(local(2026, 9, 20, 10, 0))),
             severity: .ok, codes: []),
        Case(name: "6c: never ran, sources exist", input: input(agent(daily10), evidence: evidence(nil)),
             severity: .failing, codes: [.overdue]),
        Case(name: "6c: no evidence source is amber", input: input(agent(daily10)), severity: .warning, codes: [.cannotVerify]),
        Case(name: "6c: never-run job loaded after its only slot", input: input(agent(.calendar([CalendarEntry(minute: 0, hour: 3, weekday: 0)])),
             loaded(lastExit: .neverExited, runs: 0), evidence: evidence(nil),
             history: AgentHistory(loadedObservedAt: local(2026, 9, 21, 9, 0))), severity: .ok, codes: []),
        Case(name: "6c: impossible calendar has nothing to miss", input: input(agent(.calendar([CalendarEntry(minute: 0, hour: 0, day: 30, month: 2)])),
             evidence: evidence(nil)), severity: .ok, codes: []),

        // America/Montreal DST, 2026-03-08 (spring forward) and 2026-11-01 (fall back).
        Case(name: "6c DST: skipped 02:30 on spring-forward day is not missed",
             input: input(agent(.calendar([CalendarEntry(minute: 30, hour: 2)])), evidence: evidence(utc("2026-03-07T07:30:10Z"))),
             now: local(2026, 3, 8, 9, 0), power: PowerTimeline(bootTime: local(2026, 3, 1, 8, 0)), severity: .ok, codes: []),
        Case(name: "6c DST: next day's 02:30 is expected again",
             input: input(agent(.calendar([CalendarEntry(minute: 30, hour: 2)])), evidence: evidence(utc("2026-03-07T07:30:10Z"))),
             now: local(2026, 3, 9, 9, 0), power: PowerTimeline(bootTime: local(2026, 3, 1, 8, 0)),
             severity: .failing, codes: [.overdue], summary: "Overdue: missed the Mar 9 02:30 run"),
        Case(name: "6c DST: repeated 01:30 is satisfied by the first run",
             input: input(agent(.calendar([CalendarEntry(minute: 30, hour: 1)])), evidence: evidence(utc("2026-11-01T05:30:10Z"))),
             now: utc("2026-11-01T07:00:00Z"), power: PowerTimeline(bootTime: local(2026, 10, 25, 8, 0)), severity: .ok, codes: []),
        Case(name: "6c DST: daily 10:00 after spring forward is at 14:00Z",
             input: input(agent(daily10), evidence: evidence(utc("2026-03-08T14:00:30Z"))),
             now: local(2026, 3, 8, 12, 0), power: PowerTimeline(bootTime: local(2026, 3, 1, 8, 0)), severity: .ok, codes: []),
        Case(name: "6c DST: evidence at the EST time after spring forward is an hour late but still inside grace",
             input: input(agent(daily10), evidence: evidence(utc("2026-03-07T15:00:30Z"))),
             now: utc("2026-03-08T14:10:00Z"), power: PowerTimeline(bootTime: local(2026, 3, 1, 8, 0)), severity: .ok, codes: []),
    ]

    static func run(_ c: Case) -> HealthVerdict {
        HealthEvaluator.evaluate(c.input, context: HealthContext(now: c.now, power: c.power, calendar: TimeFixtures.montreal))
    }

    static func check(_ c: Case) {
        let verdict = run(c)
        #expect(verdict.severity == c.severity)
        #expect(verdict.reasons.map(\.code) == c.codes)
        if let summary = c.summary { #expect(verdict.summary == summary) }
    }

    @Test(arguments: loadCases) func loadRules(_ c: Case) { Self.check(c) }
    @Test(arguments: exitCases) func exitRule(_ c: Case) { Self.check(c) }
    @Test(arguments: keepAliveCases) func keepAliveRule(_ c: Case) { Self.check(c) }
    @Test(arguments: intervalCases) func intervalRule(_ c: Case) { Self.check(c) }
    @Test(arguments: calendarCases) func calendarRule(_ c: Case) { Self.check(c) }

    @Test func keepAliveMissingIsReportedForTheStreak() {
        let down = Self.run(Case(name: "", input: Self.input(Self.agent(keepAlive: Self.daemon), Self.loaded()), severity: .warning, codes: []))
        #expect(down.keepAliveMissing)
        let up = Self.run(Case(name: "", input: Self.input(Self.agent(keepAlive: Self.daemon), Self.loaded(pid: 1)), severity: .ok, codes: []))
        #expect(!up.keepAliveMissing)
        let plain = Self.run(Case(name: "", input: Self.input(), severity: .ok, codes: []))
        #expect(!plain.keepAliveMissing)
    }

    @Test func calendarTraceShowsItsWorking() throws {
        let wake = Self.local(2026, 9, 26, 11, 50)
        let verdict = Self.run(Case(name: "", input: Self.input(Self.agent(Self.daily10), evidence: Self.evidence(Self.ago(Self.hour * 26))),
                                    power: PowerTimeline(bootTime: Self.boot, lastWake: wake), severity: .ok, codes: []))
        let trace = try #require(verdict.overdue)
        #expect(trace.kind == .calendar)
        #expect(trace.latestSlot == Self.local(2026, 9, 26, 10, 0))
        #expect(trace.deadline == wake.addingTimeInterval(900))
        #expect(trace.nextExpected == Self.local(2026, 9, 27, 10, 0))
        #expect(trace.skipped == nil)
    }

    @Test func intervalTraceShowsItsWorking() throws {
        let loadedAt = Self.local(2026, 9, 26, 9, 0)
        let lastRun = Self.ago(10 * Self.minute)
        let verdict = Self.run(Case(name: "", input: Self.input(Self.agent(Self.every15), evidence: Self.evidence(lastRun),
                                                               history: AgentHistory(loadedObservedAt: loadedAt)),
                                    severity: .ok, codes: []))
        let trace = try #require(verdict.overdue)
        #expect(trace.kind == .maxAge)
        #expect(trace.allowedAge == 1800)
        #expect(trace.t0 == loadedAt)
        #expect(trace.nextExpected == lastRun.addingTimeInterval(900))
    }

    @Test func skippedChecksSayWhy() {
        let beforeBoot = Self.run(Case(name: "", input: Self.input(Self.agent(Self.daily10), evidence: Self.evidence(nil)),
                                       power: PowerTimeline(bootTime: Self.local(2026, 9, 26, 11, 0)), severity: .ok, codes: []))
        #expect(beforeBoot.overdue?.skipped == "latest slot is before boot or load")
        let running = Self.run(Case(name: "", input: Self.input(Self.agent(Self.every15), Self.loaded(pid: 3)), severity: .ok, codes: []))
        #expect(running.overdue?.skipped == "running now")
        #expect(Self.run(Case(name: "", input: Self.input(), severity: .ok, codes: [])).overdue == nil)
    }

    @Test func severityOrderIsWorstWins() {
        #expect(Severity.allCases.sorted() == [.hidden, .ok, .paused, .warning, .failing])
        #expect([Severity.paused, .failing, .warning].max() == .failing)
    }
}

@Suite struct PowerTimelineTests {
    @Test func zeroWakeTimeMeansNoWakeSinceBoot() {
        let timeline = PowerTimeline(bootTime: timeval(tv_sec: 1_790_000_000, tv_usec: 500_000), wakeTime: timeval(tv_sec: 0, tv_usec: 0))
        #expect(timeline.bootTime == Date(timeIntervalSince1970: 1_790_000_000.5))
        #expect(timeline.lastWake == nil)
        #expect(timeline.awakeSince == timeline.bootTime)
    }

    @Test func wakeAfterBootIsUsed() {
        let timeline = PowerTimeline(bootTime: timeval(tv_sec: 1_790_000_000, tv_usec: 0), wakeTime: timeval(tv_sec: 1_790_050_000, tv_usec: 0))
        #expect(timeline.lastWake == Date(timeIntervalSince1970: 1_790_050_000))
        #expect(timeline.awakeSince == Date(timeIntervalSince1970: 1_790_050_000))
    }

    @Test func wakeBeforeBootIsStaleAndIgnored() {
        let timeline = PowerTimeline(bootTime: Date(timeIntervalSince1970: 1_790_000_000), lastWake: Date(timeIntervalSince1970: 1_789_000_000))
        #expect(timeline.lastWake == nil)
    }

    /// Reads sysctl on this Mac; checks shape only, not the clock.
    @Test func liveReadIsConsistent() throws {
        let timeline = try #require(PowerTimeline.current())
        #expect(timeline.bootTime.timeIntervalSince1970 > 0)
        if let wake = timeline.lastWake { #expect(wake > timeline.bootTime) }
    }
}

@Suite struct AgeDescriptionTests {
    @Test(arguments: [(0, "0 s"), (59, "59 s"), (60, "1 min"), (3599, "59 min"), (3600, "1 h"), (86399, "23 h"), (86400, "1 d"), (-5, "0 s")])
    func age(_ seconds: Int, _ text: String) {
        #expect(ScheduleDescription.age(seconds) == text)
    }
}
