import Foundation
import CoreGraphics

/// The picker's zoom factor, stepped with ⌘+ / ⌘- and reset with ⌘0 like a
/// browser. The panel grows and shrinks with it, so a zoomed picker shows the
/// same layout at a larger size rather than clipping bigger text.
public enum PanelZoom {
    public static let levels: [CGFloat] = [0.75, 0.85, 1, 1.1, 1.25, 1.5, 1.75, 2]
    public static let defaultLevel: CGFloat = 1

    /// The next level above `zoom`, or the largest one.
    public static func zoomedIn(from zoom: CGFloat) -> CGFloat {
        levels.first { $0 > zoom + tolerance } ?? levels[levels.count - 1]
    }

    /// The next level below `zoom`, or the smallest one.
    public static func zoomedOut(from zoom: CGFloat) -> CGFloat {
        levels.last { $0 < zoom - tolerance } ?? levels[0]
    }

    /// A stored value snapped to the nearest level; the default when missing
    /// or not a usable number.
    public static func sanitized(_ zoom: Double?) -> CGFloat {
        guard let zoom, zoom.isFinite, zoom > 0 else { return defaultLevel }
        return levels.min { abs($0 - zoom) < abs($1 - zoom) } ?? defaultLevel
    }

    /// `frame` resized to `size` around its centre, then moved (and shrunk
    /// where needed) to stay inside `visible` with `PanelPlacement.margin`.
    public static func resized(_ frame: CGRect, to size: CGSize, in visible: CGRect) -> CGRect {
        let fitted = PanelPlacement.fittedSize(size, in: visible)
        let inset = visible.insetBy(dx: PanelPlacement.margin, dy: PanelPlacement.margin)
        let x = frame.midX - fitted.width / 2
        let y = frame.midY - fitted.height / 2
        return CGRect(
            x: max(inset.minX, min(x, inset.maxX - fitted.width)),
            y: max(inset.minY, min(y, inset.maxY - fitted.height)),
            width: fitted.width, height: fitted.height)
    }

    private static let tolerance: CGFloat = 0.001
}
