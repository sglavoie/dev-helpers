import AppKit
import SloppyCore
import SwiftUI

/// A floating panel that takes key status without activating the app, so the
/// app the user was typing in stays frontmost and can be pasted into.
final class PickerPanel: NSPanel {
    /// Called when the panel loses key status (a click outside, another app).
    var onResignKey: (() -> Void)?

    init(contentRect: NSRect) {
        super.init(
            contentRect: contentRect,
            styleMask: [.nonactivatingPanel, .titled, .fullSizeContentView],
            backing: .buffered,
            defer: true
        )
        level = .floating
        collectionBehavior = [.canJoinAllSpaces, .fullScreenAuxiliary, .transient, .ignoresCycle]
        isFloatingPanel = true
        hidesOnDeactivate = false
        becomesKeyOnlyIfNeeded = false
        titleVisibility = .hidden
        titlebarAppearsTransparent = true
        isMovableByWindowBackground = true
        isReleasedWhenClosed = false
        animationBehavior = .utilityWindow
        for button in [NSWindow.ButtonType.closeButton, .miniaturizeButton, .zoomButton] {
            standardWindowButton(button)?.isHidden = true
        }
    }

    override var canBecomeKey: Bool { true }
    override var canBecomeMain: Bool { false }

    override func resignKey() {
        super.resignKey()
        onResignKey?()
    }

    // Esc is routed by KeyRouter; stop NSPanel from closing on its own.
    override func cancelOperation(_ sender: Any?) {}
}

/// App-level commands the picker can run; they leave the panel (modal
/// panels, the Settings window), so the app delegate provides them.
@MainActor
struct PickerCommands {
    var importData: @MainActor () -> Void = {}
    var exportData: @MainActor () -> Void = {}
    var openSettings: @MainActor () -> Void = {}
    var storageDescription: @MainActor () -> String = { "" }
}

/// Owns the picker panel and its navigation state, and shows or hides it.
@MainActor
final class PickerPanelController {
    static let panelSize = NSSize(width: 760, height: 480)

    let navigator = Navigator()
    private(set) lazy var keyRouter = KeyRouter(navigator: navigator) { [weak self] in
        self?.hide()
    }
    let accessibility: AccessibilityPermission
    let toasts = ToastCenter()
    var commands = PickerCommands()
    private let store: SnippetStore
    private let panel: PickerPanel
    private let paster: Paster
    private let hud = HUD()
    private lazy var snippetUse = SnippetUse(
        record: { [store] id, values in
            try await store.recordUse(id: id, placeholderValues: values)
        },
        log: { NSLog("Sloppy Paste: %@", $0) }
    )
    /// The app that was frontmost when the panel opened; the paste target.
    private(set) var previousApp: NSRunningApplication?
    /// While positive, losing key status does not hide the panel (menus, popovers).
    private var hideSuppressionCount = 0
    /// Menus that began tracking while the panel was visible and have not ended.
    private var menuTrackingDepth = 0

    init(store: SnippetStore, accessibility: AccessibilityPermission) {
        self.store = store
        self.accessibility = accessibility
        paster = Paster(accessibility: accessibility)
        panel = PickerPanel(contentRect: NSRect(origin: .zero, size: Self.panelSize))

        let root = PickerContentView()
            .environment(store)
            .environment(navigator)
            .environment(accessibility)
            .environment(toasts)
            .environment(keyRouter.actionMenu)
            .environment(\.keyRouter, keyRouter)
            .environment(\.pickerPanel, self)
        let hostingView = NSHostingView(rootView: root)
        hostingView.sizingOptions = []
        panel.contentView = hostingView

        panel.onResignKey = { [weak self] in
            MainActor.assumeIsolated {
                guard let self, self.hideSuppressionCount == 0 else { return }
                self.hide()
            }
        }
        navigator.willChange = { [weak self, weak panel] in
            self?.keyRouter.closeActionMenu(restoringFocus: false)
            panel?.makeFirstResponder(nil)
        }
        keyRouter.start(for: panel)
        observeMenuTracking()
        observeScreenChanges()
    }

    /// Menus inside the panel (the form's dropdowns, the ⌘P filters) can take
    /// key status from the non-activating panel while they track. Suppress
    /// hide-on-resign for that time and take key status back afterwards.
    private func observeMenuTracking() {
        let center = NotificationCenter.default
        center.addObserver(forName: NSMenu.didBeginTrackingNotification, object: nil, queue: .main) { [weak self] _ in
            MainActor.assumeIsolated {
                guard let self, self.panel.isVisible else { return }
                self.menuTrackingDepth += 1
                self.beginSuppressingHide()
            }
        }
        center.addObserver(forName: NSMenu.didEndTrackingNotification, object: nil, queue: .main) { [weak self] _ in
            MainActor.assumeIsolated {
                guard let self, self.menuTrackingDepth > 0 else { return }
                self.menuTrackingDepth -= 1
                self.endSuppressingHide()
                guard self.menuTrackingDepth == 0, self.panel.isVisible, !self.panel.isKeyWindow else { return }
                // Another app came forward while the menu was open: behave
                // like a click outside. Otherwise the menu only borrowed key.
                let frontmost = NSWorkspace.shared.frontmostApplication?.processIdentifier
                let ownPID = ProcessInfo.processInfo.processIdentifier
                if frontmost == ownPID || frontmost == self.previousApp?.processIdentifier {
                    self.panel.makeKey()
                } else {
                    self.hide()
                }
            }
        }
    }

    var isVisible: Bool { panel.isVisible }

    func toggle() {
        if panel.isVisible {
            hide()
        } else {
            show()
        }
    }

    func show() {
        let frontmost = NSWorkspace.shared.frontmostApplication
        if frontmost?.processIdentifier != ProcessInfo.processInfo.processIdentifier {
            previousApp = frontmost
        }
        store.reloadIfChanged()
        accessibility.refresh()
        keyRouter.closeActionMenu(restoringFocus: false)
        navigator.popToRoot()
        position()
        panel.orderFrontRegardless()
        panel.makeKey()
    }

    /// Shows the panel on `route` (above the root), e.g. New Snippet from the menu bar.
    func show(_ route: Route) {
        show()
        if route != .root {
            navigator.push(route)
        }
    }

    func hide() {
        guard panel.isVisible else { return }
        panel.orderOut(nil)
    }

    enum DeliveryMode {
        /// Paste into the previous app (copy-only without Accessibility or under Secure Input).
        case paste
        /// Copy to the clipboard only.
        case copy
        /// Copy to the clipboard and keep the panel open for more copies.
        case copyAndStay

        init(_ mode: PlaceholderFormMode) {
            switch mode {
            case .paste: self = .paste
            case .copy: self = .copy
            case .copyAndStay: self = .copyAndStay
            }
        }

        /// The placeholder form mode that ends in this delivery.
        var formMode: PlaceholderFormMode {
            switch self {
            case .paste: .paste
            case .copy: .copy
            case .copyAndStay: .copyAndStay
            }
        }
    }

    /// Pastes or copies the prepared content, reports the outcome in a HUD,
    /// and then records the use. The panel hides first unless the mode is
    /// `.copyAndStay`.
    func deliver(_ prepared: PreparedSnippet, snippetID: String, mode: DeliveryMode) {
        let target = previousApp
        if mode != .copyAndStay {
            hide()
        }
        Task { @MainActor in
            _ = await snippetUse.run(
                snippetID: snippetID,
                placeholderValues: prepared.placeholderValues,
                prepare: { prepared },
                primaryOperation: { [weak self] prepared in
                    try await self?.performDelivery(prepared.content, mode: mode, target: target)
                },
                onPreparationFailure: { _ in },
                onPrimaryFailure: { [weak self] error in
                    await self?.hud.show("Could Not Copy", message: String(describing: error), symbol: "xmark.octagon")
                }
            )
        }
    }

    private func performDelivery(_ content: String, mode: DeliveryMode, target: NSRunningApplication?) async throws {
        let outcome: PasteOutcome
        switch mode {
        case .paste:
            outcome = try await paster.paste(content, into: target)
        case .copy:
            try Pasteboard.copy(content)
            outcome = .copied
        case .copyAndStay:
            try Pasteboard.copy(content)
            hud.show("Copied to Clipboard", message: "Picker stays open for more copies")
            return
        }
        if let hud = outcome.hud {
            self.hud.show(hud.title, message: hud.message)
        }
    }

    /// Shows a failure notice, e.g. when Paste with Last Values has no history.
    func showFailure(_ title: String, message: String) {
        hud.show(title, message: message, symbol: "xmark.octagon")
    }

    /// Runs `body` without the panel hiding when it loses key status, e.g.
    /// while a menu or popover inside the panel is open.
    func withHideSuppressed<T>(_ body: () throws -> T) rethrows -> T {
        hideSuppressionCount += 1
        defer { hideSuppressionCount -= 1 }
        return try body()
    }

    func beginSuppressingHide() { hideSuppressionCount += 1 }
    func endSuppressingHide() { hideSuppressionCount = max(0, hideSuppressionCount - 1) }

    /// Centres the panel horizontally, a little above the middle, on the
    /// screen with the mouse pointer, shrinking it on a screen too small for it.
    private func position() {
        let preferred = panel.frameRect(forContentRect: NSRect(origin: .zero, size: Self.panelSize)).size
        guard let frame = PanelPlacement.frame(
            preferred: preferred, mouse: NSEvent.mouseLocation,
            screens: NSScreen.placementScreens, verticalFraction: 0.6)
        else { return }
        panel.setFrame(frame, display: false)
    }

    /// A display was added, removed or rearranged: move a visible panel back
    /// onto a screen if its own went away or it no longer fits.
    private func observeScreenChanges() {
        NotificationCenter.default.addObserver(
            forName: NSApplication.didChangeScreenParametersNotification, object: nil, queue: .main
        ) { [weak self] _ in
            MainActor.assumeIsolated {
                guard let self, self.panel.isVisible else { return }
                let frame = self.panel.frame
                let fits = NSScreen.screens.contains { $0.visibleFrame.contains(frame) }
                if !fits {
                    self.position()
                }
            }
        }
    }
}

extension NSScreen {
    /// The connected screens, for `PanelPlacement`.
    static var placementScreens: [PanelPlacement.Screen] {
        screens.map { PanelPlacement.Screen(frame: $0.frame, visibleFrame: $0.visibleFrame) }
    }
}

private struct KeyRouterKey: EnvironmentKey {
    static let defaultValue: KeyRouter? = nil
}

private struct PickerPanelKey: EnvironmentKey {
    static let defaultValue: PickerPanelController? = nil
}

extension EnvironmentValues {
    var keyRouter: KeyRouter? {
        get { self[KeyRouterKey.self] }
        set { self[KeyRouterKey.self] = newValue }
    }

    var pickerPanel: PickerPanelController? {
        get { self[PickerPanelKey.self] }
        set { self[PickerPanelKey.self] = newValue }
    }
}
