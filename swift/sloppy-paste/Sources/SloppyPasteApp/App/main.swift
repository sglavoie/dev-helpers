import AppKit

// AppKit lifecycle instead of the SwiftUI `App` protocol: its Settings scene
// is unreliable in accessory apps.
MainActor.assumeIsolated {
    let app = NSApplication.shared
    let delegate = AppDelegate()
    app.delegate = delegate
    app.setActivationPolicy(.accessory)
    withExtendedLifetime(delegate) {
        app.run()
    }
}
