import Foundation

/// Real LaunchAgent shapes, copied from ~/scripts/launchagents (Library/LaunchAgents and installer-managed).
enum PlistFixtures {
    static func wrap(_ body: String) -> String {
        """
        <?xml version="1.0" encoding="UTF-8"?>
        <!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
        <plist version="1.0">
        <dict>
        \(body)
        </dict>
        </plist>
        """
    }

    /// Calendar array, RunAtLoad, shared stdout/stderr log.
    static let backupLegacyRepo = wrap(#"""
        <key>Label</key>
        <string>com.sglavoie.backup-legacy-repo</string>
        <key>ProgramArguments</key>
        <array>
            <string>/Users/sglavoie/.local/bin/backup-legacy.sh</string>
        </array>
        <key>StartCalendarInterval</key>
        <array>
            <dict><key>Hour</key><integer>10</integer><key>Minute</key><integer>0</integer></dict>
            <dict><key>Hour</key><integer>13</integer><key>Minute</key><integer>0</integer></dict>
            <dict><key>Hour</key><integer>21</integer><key>Minute</key><integer>0</integer></dict>
        </array>
        <key>RunAtLoad</key>
        <true/>
        <key>StandardOutPath</key>
        <string>/Users/sglavoie/Library/Logs/legacy-backup.log</string>
        <key>StandardErrorPath</key>
        <string>/Users/sglavoie/Library/Logs/legacy-backup.log</string>
        """#)

    /// KeepAlive dict with SuccessfulExit false, ThrottleInterval, environment.
    static let brainnotesVaultGuard = wrap(#"""
        <key>Label</key>
        <string>com.sglavoie.brainnotes-vault-guard</string>
        <key>ProgramArguments</key>
        <array>
            <string>/Users/sglavoie/.go/bin/bn-vault-guard</string>
        </array>
        <key>EnvironmentVariables</key>
        <dict>
            <key>BNOTES_VAULT</key>
            <string>/Users/sglavoie/BrainNotesVault</string>
        </dict>
        <key>RunAtLoad</key>
        <true/>
        <key>KeepAlive</key>
        <dict>
            <key>SuccessfulExit</key>
            <false/>
        </dict>
        <key>ThrottleInterval</key>
        <integer>30</integer>
        <key>ProcessType</key>
        <string>Background</string>
        <key>StandardOutPath</key>
        <string>/Users/sglavoie/Library/Logs/brainnotes-vault-guard.log</string>
        <key>StandardErrorPath</key>
        <string>/Users/sglavoie/Library/Logs/brainnotes-vault-guard.log</string>
        """#)

    /// Calendar array with a Weekday.
    static let brewMaintain = wrap(#"""
        <key>Label</key>
        <string>com.sglavoie.brew-maintain</string>
        <key>ProgramArguments</key>
        <array>
            <string>/Users/sglavoie/.local/bin/brew-maintain.sh</string>
        </array>
        <key>StartCalendarInterval</key>
        <array>
            <dict>
                <key>Weekday</key><integer>3</integer>
                <key>Hour</key><integer>12</integer>
                <key>Minute</key><integer>0</integer>
            </dict>
        </array>
        <key>StandardOutPath</key>
        <string>/Users/sglavoie/Library/Logs/brew-maintain.log</string>
        <key>StandardErrorPath</key>
        <string>/Users/sglavoie/Library/Logs/brew-maintain.log</string>
        """#)

    /// KeepAlive true daemon.
    static let ddcBrightnessd = wrap(#"""
        <key>Label</key>
        <string>com.sglavoie.ddc-brightnessd</string>
        <key>ProgramArguments</key>
        <array>
            <string>/Users/sglavoie/.local/bin/ddc-brightnessd</string>
        </array>
        <key>RunAtLoad</key>
        <true/>
        <key>KeepAlive</key>
        <true/>
        <key>ThrottleInterval</key>
        <integer>10</integer>
        <key>ProcessType</key>
        <string>Interactive</string>
        <key>StandardOutPath</key>
        <string>/Users/sglavoie/Library/Logs/ddc-brightnessd.log</string>
        <key>StandardErrorPath</key>
        <string>/Users/sglavoie/Library/Logs/ddc-brightnessd.log</string>
        """#)

    /// StartInterval.
    static let forgejoSync = wrap(#"""
        <key>Label</key>
        <string>com.sglavoie.forgejo-sync</string>
        <key>ProgramArguments</key>
        <array>
            <string>/Users/sglavoie/.local/bin/forgejo-sync.sh</string>
        </array>
        <key>StartInterval</key>
        <integer>900</integer>
        <key>RunAtLoad</key>
        <true/>
        <key>ProcessType</key>
        <string>Background</string>
        <key>LowPriorityIO</key>
        <true/>
        <key>StandardOutPath</key>
        <string>/Users/sglavoie/Library/Logs/forgejo-sync.log</string>
        <key>StandardErrorPath</key>
        <string>/Users/sglavoie/Library/Logs/forgejo-sync.log</string>
        """#)

    /// Bare-dict StartCalendarInterval.
    static let logRotate = wrap(#"""
        <key>Label</key>
        <string>com.sglavoie.log-rotate</string>
        <key>ProgramArguments</key>
        <array>
            <string>/Users/sglavoie/.local/bin/log-rotate.sh</string>
        </array>
        <key>StartCalendarInterval</key>
        <dict>
            <key>Weekday</key>
            <integer>1</integer>
            <key>Hour</key>
            <integer>11</integer>
            <key>Minute</key>
            <integer>0</integer>
        </dict>
        <key>StandardOutPath</key>
        <string>/tmp/log-rotate.log</string>
        <key>StandardErrorPath</key>
        <string>/tmp/log-rotate.log</string>
        """#)

    /// Installer-managed: tab-indented, separate stdout/stderr, RunAtLoad false, multi-argument program.
    static let piBackupFetch = wrap(#"""
        	<key>EnvironmentVariables</key>
        	<dict>
        		<key>HOME</key>
        		<string>/Users/sglavoie</string>
        	</dict>
        	<key>Label</key>
        	<string>com.sglavoie.pi-backup-fetch</string>
        	<key>LowPriorityIO</key>
        	<true/>
        	<key>Nice</key>
        	<integer>10</integer>
        	<key>ProcessType</key>
        	<string>Background</string>
        	<key>ProgramArguments</key>
        	<array>
        		<string>/opt/homebrew/opt/python@3.14/bin/python3.14</string>
        		<string>/Users/sglavoie/Library/Application Support/pi-backup-fetch/runner/install-backup-fetch-mac.py</string>
        		<string>run</string>
        	</array>
        	<key>RunAtLoad</key>
        	<false/>
        	<key>StandardErrorPath</key>
        	<string>/Users/sglavoie/Library/Application Support/pi-backup-fetch/runner.err</string>
        	<key>StandardOutPath</key>
        	<string>/Users/sglavoie/Library/Application Support/pi-backup-fetch/runner.log</string>
        	<key>StartCalendarInterval</key>
        	<array>
        		<dict>
        			<key>Hour</key>
        			<integer>6</integer>
        			<key>Minute</key>
        			<integer>10</integer>
        		</dict>
        		<dict>
        			<key>Hour</key>
        			<integer>12</integer>
        			<key>Minute</key>
        			<integer>10</integer>
        		</dict>
        	</array>
        	<key>Umask</key>
        	<integer>63</integer>
        	<key>WorkingDirectory</key>
        	<string>/Users/sglavoie/Library/Application Support/pi-backup-fetch/runner</string>
        """#)

    /// WatchPaths + ThrottleInterval.
    static let syncLegacy = wrap(#"""
        <key>Label</key>
        <string>com.sglavoie.sync-legacy</string>
        <key>ProgramArguments</key>
        <array>
            <string>/Users/sglavoie/.local/bin/sync-legacy.sh</string>
        </array>
        <key>WatchPaths</key>
        <array>
            <string>/Users/sglavoie/1_dev_projects/sglavoie_life-trail/PARA_MARIANA.md</string>
            <string>/Users/sglavoie/1_dev_projects/sglavoie_life-trail/LEGACY_CHECKLIST.md</string>
            <string>/Users/sglavoie/1_dev_projects/sglavoie_life-trail/NOTA_PARA_MARIANA.md</string>
            <string>/Users/sglavoie/1_dev_projects/sglavoie_life-trail/legacy.env</string>
        </array>
        <key>ThrottleInterval</key>
        <integer>30</integer>
        <key>StandardOutPath</key>
        <string>/Users/sglavoie/Library/Logs/sync-legacy.log</string>
        <key>StandardErrorPath</key>
        <string>/Users/sglavoie/Library/Logs/sync-legacy.log</string>
        """#)

    /// Installer-managed: explicit Disabled false, empty EnvironmentVariables, KeepAlive dict.
    static let brainnotesSyncthing = wrap(#"""
        	<key>Disabled</key>
        	<false/>
        	<key>EnvironmentVariables</key>
        	<dict/>
        	<key>KeepAlive</key>
        	<dict>
        		<key>SuccessfulExit</key>
        		<false/>
        	</dict>
        	<key>Label</key>
        	<string>com.sglavoie.brainnotes.syncthing</string>
        	<key>ProgramArguments</key>
        	<array>
        		<string>/Users/sglavoie/Library/Caches/brainnotes/syncthing/v1.30.0/syncthing</string>
        		<string>serve</string>
        	</array>
        	<key>RunAtLoad</key>
        	<true/>
        	<key>StandardErrorPath</key>
        	<string>/Users/sglavoie/Library/Logs/BrainNotes/com.sglavoie.brainnotes/syncthing.err</string>
        	<key>StandardOutPath</key>
        	<string>/Users/sglavoie/Library/Logs/BrainNotes/com.sglavoie.brainnotes/syncthing.log</string>
        	<key>ThrottleInterval</key>
        	<integer>30</integer>
        """#)

    /// Installer-managed: KeepAlive true with logs sent to /dev/null.
    static let aslCompanion = wrap(#"""
        	<key>AbandonProcessGroup</key>
        	<true/>
        	<key>ExitTimeOut</key>
        	<integer>30</integer>
        	<key>KeepAlive</key>
        	<true/>
        	<key>Label</key>
        	<string>com.sglavoie.asl-companion</string>
        	<key>ProgramArguments</key>
        	<array>
        		<string>/opt/homebrew/bin/python3</string>
        		<string>/Users/sglavoie/.local/bin/asl</string>
        	</array>
        	<key>RunAtLoad</key>
        	<true/>
        	<key>StandardErrorPath</key>
        	<string>/dev/null</string>
        	<key>StandardOutPath</key>
        	<string>/dev/null</string>
        """#)

    static func minimal(label: String) -> String {
        wrap("<key>Label</key><string>\(label)</string><key>Program</key><string>/bin/true</string>")
    }
}
