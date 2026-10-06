import AppKit
import SloppyCore

/// The panel's content view: the SwiftUI content with thin, invisible drag
/// handles along its borders and corners. The panel is not `.resizable`,
/// since AppKit's own resizing keeps the opposite border fixed and the picker
/// resizes symmetrically around its centre instead.
final class PanelResizeContainer: NSView {
    init(content: NSView, resizer: PanelResizer) {
        super.init(frame: NSRect(origin: .zero, size: content.frame.size))
        content.frame = bounds
        content.autoresizingMask = [.width, .height]
        addSubview(content)
        let edges: [PanelSize.Edges] = [
            .left, .right, .top, .bottom,
            [.top, .left], [.top, .right], [.bottom, .left], [.bottom, .right],
        ]
        for edge in edges {
            let handle = ResizeHandleView(edges: edge, resizer: resizer)
            handle.frame = Self.handleFrame(for: edge, in: bounds)
            handle.autoresizingMask = Self.autoresizingMask(for: edge)
            addSubview(handle)
        }
    }

    @available(*, unavailable)
    required init?(coder: NSCoder) { fatalError("init(coder:) is not supported") }

    /// Thin enough to leave most of an overlay scroller clickable.
    private static let thickness: CGFloat = 5
    private static let cornerLength: CGFloat = 14

    private static func handleFrame(for edges: PanelSize.Edges, in bounds: NSRect) -> NSRect {
        let isCorner = edges.intersection([.left, .right]) != [] && edges.intersection([.top, .bottom]) != []
        let width = edges.intersection([.left, .right]).isEmpty
            ? bounds.width - 2 * cornerLength
            : (isCorner ? cornerLength : thickness)
        let height = edges.intersection([.top, .bottom]).isEmpty
            ? bounds.height - 2 * cornerLength
            : (isCorner ? cornerLength : thickness)
        let x = edges.contains(.left) ? 0 : edges.contains(.right) ? bounds.maxX - width : cornerLength
        let y = edges.contains(.bottom) ? 0 : edges.contains(.top) ? bounds.maxY - height : cornerLength
        return NSRect(x: x, y: y, width: width, height: height)
    }

    private static func autoresizingMask(for edges: PanelSize.Edges) -> NSView.AutoresizingMask {
        var mask: NSView.AutoresizingMask = []
        if edges.intersection([.left, .right]).isEmpty { mask.insert(.width) }
        if edges.intersection([.top, .bottom]).isEmpty { mask.insert(.height) }
        if edges.contains(.right) { mask.insert(.minXMargin) }
        if edges.contains(.top) { mask.insert(.minYMargin) }
        return mask
    }
}

/// Receives drags from the border handles.
@MainActor
protocol PanelResizer: AnyObject {
    func beginResize()
    /// `delta` is the pointer's movement since the drag began, in screen coordinates.
    func resize(_ edges: PanelSize.Edges, by delta: CGVector)
    func endResize()
}

private final class ResizeHandleView: NSView {
    let edges: PanelSize.Edges
    private weak var resizer: PanelResizer?
    private var dragStart: NSPoint?

    init(edges: PanelSize.Edges, resizer: PanelResizer) {
        self.edges = edges
        self.resizer = resizer
        super.init(frame: .zero)
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
        cursor.set()
    }

    override func mouseDown(with event: NSEvent) {
        dragStart = NSEvent.mouseLocation
        resizer?.beginResize()
    }

    override func mouseDragged(with event: NSEvent) {
        guard let dragStart else { return }
        let location = NSEvent.mouseLocation
        resizer?.resize(edges, by: CGVector(dx: location.x - dragStart.x, dy: location.y - dragStart.y))
        cursor.set()
    }

    override func mouseUp(with event: NSEvent) {
        guard dragStart != nil else { return }
        dragStart = nil
        resizer?.endResize()
    }

    private var cursor: NSCursor {
        let position: NSCursor.FrameResizePosition = switch edges {
        case .left: .left
        case .right: .right
        case .top: .top
        case .bottom: .bottom
        case [.top, .left]: .topLeft
        case [.top, .right]: .topRight
        case [.bottom, .left]: .bottomLeft
        default: .bottomRight
        }
        return NSCursor.frameResize(position: position, directions: .all)
    }
}
