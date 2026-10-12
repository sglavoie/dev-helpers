# Heartbeat: health rules

Part of the [Heartbeat README](../README.md).

## Health rules

Each agent gets a severity: hidden, ok, paused (gray), warning (amber) or
failing (red). Every rule that fires adds a reason, and the worst reason wins.

1. `hidden` in config: the agent is excluded.
2. Not loaded, and either `Disabled` in the plist or unloaded from Heartbeat:
   paused. Paused agents don't color the icon.
3. Not loaded for any other reason, or `launchctl print` output that can't be
   read: amber.
4. Last exit code not 0 (and not in `ignoreExitCodes`), or killed by a signal:
   red. A KeepAlive daemon that is running again after a failure is amber
   ("Restarted after exit 1") for an hour after the restart, then ok.
   `(never exited)` is neither a failure nor a sign that the job ran.
5. KeepAlive expected (`true`, or `SuccessfulExit: false`) but no PID: amber on
   the first poll, red from the second, so a ThrottleInterval respawn doesn't
   raise a false alarm.
6. Overdue, checked only while the agent isn't running:
   - `StartInterval` I: allowed age is `maxAgeSeconds`, or max(2I, I + 5 min)
     without it. Red when the newest evidence is older than that and the Mac
     has been awake, and the agent loaded, for longer than that too.
   - `StartCalendarInterval`: S is the latest slot. Skip it if it came before
     boot or before the agent was loaded, because launchd doesn't run slots
     missed while the Mac was off. If the Mac slept through S, launchd runs the
     job once on wake, so the deadline is max(S, last wake) + grace (15 min by
     default). Red when there is no evidence from S − 60 s on and the deadline
     has passed.
   - `maxAgeSeconds` also applies to agents without an interval, and it replaces
     the calendar check.
   - No evidence source at all (no log files, evidence paths or runs ledger):
     amber "Cannot verify last run".
7. Health command (`health` in config): an argv run without a shell, with a
   fixed GUI PATH, every 300 s by default with a 30 s timeout. Exit 0 is ok,
   any other exit or a signal is red, and an exit listed in `warningExitCodes`,
   a timeout or a command that can't start is amber. The first line of output
   (stderr first) goes into the message. A result older than three intervals
   is stale (amber), once the Mac has been awake that long, because it means
   the checks stopped running.
8. Receipt (`receipt` in config) for jobs that report to Uptime Kuma: a JSON
   file the job writes after each run. A `statusKey` value outside `okValues`
   is red. `reportedKey: false` is amber, "Ran locally but not reported to
   Kuma". A missing or unreadable receipt is amber too.

Rules 7 and 8 judge the job's own output rather than launchd, so they also
apply when an agent isn't loaded. They're skipped only for hidden and paused
agents.

Boot and wake times come from `sysctl kern.boottime` and `kern.waketime`
(`waketime` is 0 until the first sleep). Calendar slots are local wall time.
The missing keys are wildcards, and Day and Weekday match either one, as in
crontab. On the spring-forward day a time that doesn't exist, like 02:30, isn't
a slot, and on the fall-back day a repeated time counts once, at its first
occurrence. The evaluator is pure: the clock, time zone and power timeline are
passed in, so the tests pin them, with `America/Montreal` for the DST cases.

