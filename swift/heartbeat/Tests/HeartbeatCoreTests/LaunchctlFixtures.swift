import Foundation

/// Real `launchctl print gui/501/<label>` outputs captured 2026-09-26 on macOS 27.0, plus
/// constructed cases (signal, garbage). Whitespace is significant: fields are tab-indented.
enum LaunchctlFixtures {
    /// WatchPaths job whose last run exited 1; nested blocks repeat `state = active`.
    static let syncLegacy = #"""
        gui/501/com.sglavoie.sync-legacy = {
        	active count = 0
        	path = /Users/sglavoie/scripts/launchagents/Library/LaunchAgents/com.sglavoie.sync-legacy.plist
        	type = LaunchAgent
        	state = not running

        	program = /Users/sglavoie/.local/bin/sync-legacy.sh
        	arguments = {
        		/Users/sglavoie/.local/bin/sync-legacy.sh
        	}

        	stdout path = /Users/sglavoie/Library/Logs/sync-legacy.log
        	stderr path = /Users/sglavoie/Library/Logs/sync-legacy.log
        	inherited environment = {
        		SSH_AUTH_SOCK => /var/run/com.apple.launchd.vjczDioG91/Listeners
        	}

        	default environment = {
        		PATH => /usr/bin:/bin:/usr/sbin:/sbin
        	}

        	environment = {
        		OSLogRateLimit => 64
        		XPC_SERVICE_NAME => com.sglavoie.sync-legacy
        	}

        	domain = gui/501 [100020]
        	asid = 100020
        	minimum runtime = 30
        	exit timeout = 5
        	runs = 4
        	last exit code = 1

        	event triggers = {
        		com.apple.launchd.WatchPaths => {
        			keepalive = 0
        			service = com.sglavoie.sync-legacy
        			stream = com.apple.fsevents.matching
        			monitor = com.apple.UserEventAgent-Aqua
        			descriptor = {
        				"WatchPaths" => [
        					0 = "/Users/sglavoie/1_dev_projects/sglavoie_life-trail/PARA_MARIANA.md"
        					1 = "/Users/sglavoie/1_dev_projects/sglavoie_life-trail/LEGACY_CHECKLIST.md"
        					2 = "/Users/sglavoie/1_dev_projects/sglavoie_life-trail/NOTA_PARA_MARIANA.md"
        					3 = "/Users/sglavoie/1_dev_projects/sglavoie_life-trail/legacy.env"
        				]
        			}
        		}
        	}

        	event channels = {
        		"com.apple.fsevents.matching" = {
        			port = 0xdb937
        			active = 0
        			managed = 1
        			reset = 0
        			hide = 0
        			watching = 1
        		}
        	}

        	resource coalition = {
        		ID = 1262
        		type = resource
        		state = active
        		active count = 1
        		name = com.sglavoie.sync-legacy
        	}

        	jetsam coalition = {
        		ID = 1263
        		type = jetsam
        		state = active
        		active count = 1
        		name = com.sglavoie.sync-legacy
        	}

        	spawn type = daemon (3)
        	jetsam priority = 40
        	jetsam memory limit (active) = (unlimited)
        	jetsam memory limit (inactive) = (unlimited)
        	jetsamproperties category = daemon
        	jetsam thread limit = 32
        	cpumon = default
        	job state = exited
        	sanitizer flags = 0x0

        	properties = inferred program | managed LWCR
        }
        """#

    /// KeepAlive daemon running on its first run: pid, `last exit code = (never exited)`.
    static let ddcBrightnessd = #"""
        gui/501/com.sglavoie.ddc-brightnessd = {
        	active count = 1
        	path = /Users/sglavoie/scripts/launchagents/Library/LaunchAgents/com.sglavoie.ddc-brightnessd.plist
        	type = LaunchAgent
        	state = running

        	program = /Users/sglavoie/.local/bin/ddc-brightnessd
        	arguments = {
        		/Users/sglavoie/.local/bin/ddc-brightnessd
        	}

        	stdout path = /Users/sglavoie/Library/Logs/ddc-brightnessd.log
        	stderr path = /Users/sglavoie/Library/Logs/ddc-brightnessd.log
        	inherited environment = {
        		SSH_AUTH_SOCK => /var/run/com.apple.launchd.vjczDioG91/Listeners
        	}

        	default environment = {
        		PATH => /usr/bin:/bin:/usr/sbin:/sbin
        	}

        	environment = {
        		OSLogRateLimit => 64
        		DDC_DISPLAY => PA27JCV
        		XPC_SERVICE_NAME => com.sglavoie.ddc-brightnessd
        	}

        	LWCR = {
        		"reqs" => {
        			"cdhash" => {
        				"$in" => [
        					0 = 				]
        			}
        		}
        		"vers" => 1
        		"comp" => 1
        		"ccat" => 0
        	}

        	domain = gui/501 [100020]
        	asid = 100020
        	minimum runtime = 10
        	exit timeout = 5
        	runs = 1
        	pid = 1076
        	immediate reason = speculative
        	forks = 0
        	execs = 1
        	initialized = 1
        	trampolined = 1
        	started suspended = 0
        	proxy started suspended = 0
        	checked allocations = 0 (queried = 1)
        	checked allocations reason = no host
        	checked allocations flags = 0x0
        	last exit code = (never exited)

        	resource coalition = {
        		ID = 1140
        		type = resource
        		state = active
        		active count = 1
        		name = com.sglavoie.ddc-brightnessd
        	}

        	jetsam coalition = {
        		ID = 1141
        		type = jetsam
        		state = active
        		active count = 1
        		name = com.sglavoie.ddc-brightnessd
        	}

        	spawn type = interactive (4)
        	jetsam priority = 40
        	jetsam memory limit (active) = (unlimited)
        	jetsam memory limit (inactive) = (unlimited)
        	jetsamproperties category = daemon
        	jetsam thread limit = 32
        	cpumon = default
        	job state = running
        	sanitizer flags = 0x0

        	properties = keepalive | runatload | inferred program | managed LWCR | has LWCR
        }
        """#

    /// Second running daemon (KeepAlive SuccessfulExit:false).
    static let brainnotesVaultGuard = #"""
        gui/501/com.sglavoie.brainnotes-vault-guard = {
        	active count = 1
        	path = /Users/sglavoie/scripts/launchagents/Library/LaunchAgents/com.sglavoie.brainnotes-vault-guard.plist
        	type = LaunchAgent
        	state = running

        	program = /Users/sglavoie/.go/bin/bn-vault-guard
        	arguments = {
        		/Users/sglavoie/.go/bin/bn-vault-guard
        	}

        	stdout path = /Users/sglavoie/Library/Logs/brainnotes-vault-guard.log
        	stderr path = /Users/sglavoie/Library/Logs/brainnotes-vault-guard.log
        	inherited environment = {
        		SSH_AUTH_SOCK => /var/run/com.apple.launchd.vjczDioG91/Listeners
        	}

        	default environment = {
        		PATH => /usr/bin:/bin:/usr/sbin:/sbin
        	}

        	environment = {
        		OSLogRateLimit => 64
        		GIT_TERMINAL_PROMPT => 0
        		PATH => /opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin
        		BNOTES_VAULT => /Users/sglavoie/BrainNotesVault
        		XPC_SERVICE_NAME => com.sglavoie.brainnotes-vault-guard
        	}

        	LWCR = {
        		"reqs" => {
        			"cdhash" => {
        				"$in" => [
        					0 = 				]
        			}
        		}
        		"vers" => 1
        		"comp" => 1
        		"ccat" => 0
        	}

        	domain = gui/501 [100020]
        	asid = 100020
        	minimum runtime = 30
        	exit timeout = 5
        	runs = 1
        	pid = 1082
        	immediate reason = speculative
        	forks = 12675
        	execs = 1
        	initialized = 1
        	trampolined = 1
        	started suspended = 0
        	proxy started suspended = 0
        	checked allocations = 0 (queried = 1)
        	checked allocations reason = no host
        	checked allocations flags = 0x0
        	last exit code = (never exited)

        	semaphores = {
        		successful exit => 0
        	}

        	resource coalition = {
        		ID = 1152
        		type = resource
        		state = active
        		active count = 1
        		name = com.sglavoie.brainnotes-vault-guard
        	}

        	jetsam coalition = {
        		ID = 1153
        		type = jetsam
        		state = active
        		active count = 1
        		name = com.sglavoie.brainnotes-vault-guard
        	}

        	spawn type = background (5)
        	jetsam priority = 40
        	jetsam memory limit (active) = (unlimited)
        	jetsam memory limit (inactive) = (unlimited)
        	jetsamproperties category = daemon
        	jetsam thread limit = 32
        	cpumon = default
        	job state = running
        	sanitizer flags = 0x0

        	properties = runatload | inferred program | managed LWCR | has LWCR
        }
        """#

    /// Interval job, last exit 0, runs = 18.
    static let forgejoSync = #"""
        gui/501/com.sglavoie.forgejo-sync = {
        	active count = 0
        	path = /Users/sglavoie/scripts/launchagents/Library/LaunchAgents/com.sglavoie.forgejo-sync.plist
        	type = LaunchAgent
        	state = not running

        	program = /Users/sglavoie/.local/bin/forgejo-sync.sh
        	arguments = {
        		/Users/sglavoie/.local/bin/forgejo-sync.sh
        	}

        	stdout path = /Users/sglavoie/Library/Logs/forgejo-sync.log
        	stderr path = /Users/sglavoie/Library/Logs/forgejo-sync.log
        	inherited environment = {
        		SSH_AUTH_SOCK => /var/run/com.apple.launchd.vjczDioG91/Listeners
        	}

        	default environment = {
        		PATH => /usr/bin:/bin:/usr/sbin:/sbin
        	}

        	environment = {
        		OSLogRateLimit => 64
        		XPC_SERVICE_NAME => com.sglavoie.forgejo-sync
        	}

        	domain = gui/501 [100020]
        	asid = 100020
        	minimum runtime = 10
        	exit timeout = 5
        	runs = 18
        	last exit code = 0

        	resource coalition = {
        		ID = 1150
        		type = resource
        		state = active
        		active count = 1
        		name = com.sglavoie.forgejo-sync
        	}

        	jetsam coalition = {
        		ID = 1151
        		type = jetsam
        		state = active
        		active count = 1
        		name = com.sglavoie.forgejo-sync
        	}

        	spawn type = background (5)
        	jetsam priority = 40
        	jetsam memory limit (active) = (unlimited)
        	jetsam memory limit (inactive) = (unlimited)
        	jetsamproperties category = daemon
        	jetsam thread limit = 32
        	cpumon = default
        	run interval = 900 seconds
        	job state = exited
        	sanitizer flags = 0x0

        	properties = runatload | low priority i/o | inferred program | managed LWCR
        }
        """#

    /// Calendar job loaded but never run: runs = 0, never exited.
    static let brewMaintain = #"""
        gui/501/com.sglavoie.brew-maintain = {
        	active count = 0
        	path = /Users/sglavoie/scripts/launchagents/Library/LaunchAgents/com.sglavoie.brew-maintain.plist
        	type = LaunchAgent
        	state = not running

        	program = /Users/sglavoie/.local/bin/brew-maintain.sh
        	arguments = {
        		/Users/sglavoie/.local/bin/brew-maintain.sh
        	}

        	stdout path = /Users/sglavoie/Library/Logs/brew-maintain.log
        	stderr path = /Users/sglavoie/Library/Logs/brew-maintain.log
        	inherited environment = {
        		SSH_AUTH_SOCK => /var/run/com.apple.launchd.vjczDioG91/Listeners
        	}

        	default environment = {
        		PATH => /usr/bin:/bin:/usr/sbin:/sbin
        	}

        	environment = {
        		OSLogRateLimit => 64
        		XPC_SERVICE_NAME => com.sglavoie.brew-maintain
        	}

        	domain = gui/501 [100020]
        	asid = 100020
        	minimum runtime = 10
        	exit timeout = 5
        	runs = 0
        	last exit code = (never exited)

        	event triggers = {
        		com.sglavoie.brew-maintain.268435472 => {
        			keepalive = 0
        			service = com.sglavoie.brew-maintain
        			stream = com.apple.launchd.calendarinterval
        			monitor = com.apple.UserEventAgent-Aqua
        			descriptor = {
        				"Minute" => 0
        				"Hour" => 12
        				"Weekday" => 3
        			}
        		}
        	}

        	event channels = {
        		"com.apple.launchd.calendarinterval" = {
        			port = 0x0
        			active = 0
        			managed = 1
        			reset = 0
        			hide = 0
        			watching = 1
        		}
        	}

        	spawn type = daemon (3)
        	jetsam priority = 40
        	jetsam memory limit (active) = (unlimited)
        	jetsam memory limit (inactive) = (unlimited)
        	jetsamproperties category = daemon
        	jetsam thread limit = 32
        	cpumon = default
        	job state = uninitialized
        	sanitizer flags = 0x0

        	properties = inferred program | needs LWCR update | managed LWCR
        }
        """#

    /// Constructed: forgejo-sync as if its last run was killed by SIGTERM.
    static let forgejoSyncSignaled = forgejoSync.replacingOccurrences(
        of: "\tlast exit code = 0", with: "\tlast terminating signal = Terminated: 15")

    /// stderr of `launchctl print gui/501/com.sglavoie.nope` (exit 113).
    static let notFoundStderr = """
        Bad request.
        Could not find service "com.sglavoie.nope" in domain for user gui: 501
        """
}
