import AppKit
import SloppyCore
import SwiftUI

/// A short-lived, click-through notice near the bottom of the screen with
/// the mouse, shown after the picker hides (e.g. "Copied to Clipboard").
@MainActor
final class HUD {
    private var panel: NSPanel?
    private var generation = 0

    func show(_ title: String, message: String? = nil, symbol: String = "doc.on.clipboard") {
        generation += 1
        let current = generation

        let panel = self.panel ?? makePanel()
        self.panel = panel
        let hosting = NSHostingView(rootView: HUDView(title: title, message: message, symbol: symbol))
        panel.contentView = hosting
        let size = hosting.fittingSize
        panel.setContentSize(size)
        position(panel, size: size)
        panel.alphaValue = 1
        panel.orderFrontRegardless()

        let duration: Duration = message == nil ? .seconds(1.2) : .seconds(3)
        Task { @MainActor [weak self] in
            try? await Task.sleep(for: duration)
            guard let self, self.generation == current else { return }
            NSAnimationContext.runAnimationGroup { context in
                context.duration = 0.25
                panel.animator().alphaValue = 0
            } completionHandler: {
                MainActor.assumeIsolated {
                    if self.generation == current { panel.orderOut(nil) }
                }
            }
        }
    }

    private func makePanel() -> NSPanel {
        let panel = NSPanel(
            contentRect: .zero, styleMask: [.borderless, .nonactivatingPanel], backing: .buffered, defer: true)
        panel.level = .statusBar
        panel.collectionBehavior = [.canJoinAllSpaces, .fullScreenAuxiliary, .transient, .ignoresCycle]
        panel.isOpaque = false
        panel.backgroundColor = .clear
        panel.hasShadow = true
        panel.ignoresMouseEvents = true
        panel.isReleasedWhenClosed = false
        return panel
    }

    private func position(_ panel: NSPanel, size: NSSize) {
        guard let visible = PanelPlacement.screen(
            containing: NSEvent.mouseLocation, in: NSScreen.placementScreens)?.visibleFrame
        else { return }
        panel.setFrameOrigin(NSPoint(x: visible.midX - size.width / 2, y: visible.minY + visible.height * 0.18))
    }
}

private struct HUDView: View {
    var title: String
    var message: String?
    var symbol: String

    var body: some View {
        HStack(alignment: .top, spacing: 10) {
            Image(systemName: symbol)
                .font(.title3)
            VStack(alignment: .leading, spacing: 3) {
                Text(title).font(.headline)
                if let message {
                    // A fixed width gives the wrapped text a definite fitting height.
                    Text(message)
                        .font(.callout)
                        .foregroundStyle(.secondary)
                        .frame(width: 300, alignment: .leading)
                        .fixedSize(horizontal: false, vertical: true)
                }
            }
        }
        .padding(.horizontal, 18)
        .padding(.vertical, 12)
        .background(.regularMaterial, in: RoundedRectangle(cornerRadius: 12))
    }
}
