import AppKit
import SwiftUI

/// A vertical divider that can be dragged sideways. The line stays 1pt; a
/// wider invisible strip over it takes the drag. Double-clicking calls
/// `onReset`.
///
/// The drag is tracked in AppKit, like the panel's border handles: the panel
/// moves by its background, and a view that refuses to move the window is
/// the reliable way to keep a drag here from dragging the whole picker.
struct PaneDivider: View {
    @Environment(PickerZoom.self) private var zoom: PickerZoom?
    var onBegin: () -> Void
    /// The pointer's horizontal movement since the drag began, in content points.
    var onDrag: (CGFloat) -> Void
    var onEnd: () -> Void
    var onReset: () -> Void

    var body: some View {
        let level = zoom?.level ?? 1
        Divider()
            .overlay {
                DividerHandle(
                    onBegin: onBegin,
                    // Screen points are zoomed; the content lays out unzoomed.
                    onDrag: { onDrag($0 / level) },
                    onEnd: onEnd,
                    onReset: onReset)
                    .frame(width: 8)
            }
    }
}

private struct DividerHandle: NSViewRepresentable {
    var onBegin: () -> Void
    var onDrag: (CGFloat) -> Void
    var onEnd: () -> Void
    var onReset: () -> Void

    func makeNSView(context: Context) -> DividerHandleView {
        let view = DividerHandleView()
        update(view)
        return view
    }

    func updateNSView(_ view: DividerHandleView, context: Context) {
        update(view)
    }

    private func update(_ view: DividerHandleView) {
        view.onBegin = onBegin
        view.onDrag = onDrag
        view.onEnd = onEnd
        view.onReset = onReset
    }
}

private final class DividerHandleView: NSView {
    var onBegin: () -> Void = {}
    var onDrag: (CGFloat) -> Void = { _ in }
    var onEnd: () -> Void = {}
    var onReset: () -> Void = {}
    private var dragStartX: CGFloat?

    override init(frame: NSRect) {
        super.init(frame: frame)
        // The panel is non-activating, so cursor rects alone would not
        // update while another app is active.
        addTrackingArea(NSTrackingArea(
            rect: .zero, options: [.cursorUpdate, .activeAlways, .inVisibleRect], owner: self))
    }

    @available(*, unavailable)
    required init?(coder: NSCoder) { fatalError("init(coder:) is not supported") }

    override var mouseDownCanMoveWindow: Bool { false }
    override func acceptsFirstMouse(for event: NSEvent?) -> Bool { true }

    override func cursorUpdate(with event: NSEvent) {
        NSCursor.columnResize.set()
    }

    override func mouseDown(with event: NSEvent) {
        if event.clickCount == 2 {
            dragStartX = nil
            onReset()
            return
        }
        dragStartX = NSEvent.mouseLocation.x
        onBegin()
    }

    override func mouseDragged(with event: NSEvent) {
        guard let dragStartX else { return }
        onDrag(NSEvent.mouseLocation.x - dragStartX)
        NSCursor.columnResize.set()
    }

    override func mouseUp(with event: NSEvent) {
        guard dragStartX != nil else { return }
        dragStartX = nil
        onEnd()
    }
}
