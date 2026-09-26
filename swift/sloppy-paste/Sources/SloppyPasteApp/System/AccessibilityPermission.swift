import AppKit
import ApplicationServices
import Observation

/// Tracks the Accessibility (TCC) grant needed to post ⌘V. Until it is
/// granted the app works in copy-only mode, and it polls so the grant is
/// picked up without a restart.
@MainActor
@Observable
final class AccessibilityPermission {
    private(set) var isTrusted: Bool

    @ObservationIgnored private var pollTimer: Timer?

    static let settingsURL = URL(
        string: "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility")!

    init() {
        isTrusted = AXIsProcessTrusted()
    }

    /// Shows the system prompt when the grant is missing, and starts polling.
    func requestIfNeeded() {
        refresh()
        guard !isTrusted else { return }
        // A literal rather than kAXTrustedCheckOptionPrompt, a mutable C global under Swift 6.
        let options = ["AXTrustedCheckOptionPrompt": true] as CFDictionary
        isTrusted = AXIsProcessTrustedWithOptions(options)
        startPolling()
    }

    func refresh() {
        isTrusted = AXIsProcessTrusted()
        if isTrusted { stopPolling() }
    }

    func openSystemSettings() {
        NSWorkspace.shared.open(Self.settingsURL)
        startPolling()
    }

    private func startPolling() {
        guard !isTrusted, pollTimer == nil else { return }
        pollTimer = Timer.scheduledTimer(withTimeInterval: 2, repeats: true) { [weak self] _ in
            MainActor.assumeIsolated { self?.refresh() }
        }
    }

    private func stopPolling() {
        pollTimer?.invalidate()
        pollTimer = nil
    }
}
