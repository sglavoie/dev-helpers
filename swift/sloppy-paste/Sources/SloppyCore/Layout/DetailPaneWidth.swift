import Foundation
import CoreGraphics

/// The ⌘D detail pane's width in points at 100% zoom, set by dragging the
/// divider between it and the snippet list or in Settings.
public enum DetailPaneWidth {
    public static let defaultsKey = "picker.detailWidth"
    public static let defaultWidth: CGFloat = 330
    public static let minimum: CGFloat = 240
    public static let maximum: CGFloat = 2000
    /// Room always left for the snippet list beside the pane.
    public static let minimumListWidth: CGFloat = 280

    /// A stored width clamped to `minimum...maximum`; the default when
    /// missing or not a usable number.
    public static func sanitized(_ width: Double?) -> CGFloat {
        guard let width, width.isFinite, width > 0 else { return defaultWidth }
        return max(minimum, min(CGFloat(width.rounded()), maximum))
    }

    /// `width` as shown in a row `total` points wide: narrowed to leave the
    /// list `minimumListWidth`, but never below `minimum`.
    public static func displayed(_ width: CGFloat, in total: CGFloat) -> CGFloat {
        max(minimum, min(width, total - minimumListWidth))
    }

    /// The width after dragging the divider `dx` points (positive is right,
    /// which narrows the pane) from a pane that was `start` wide.
    public static func dragged(from start: CGFloat, by dx: CGFloat, in total: CGFloat) -> CGFloat {
        displayed(min(start - dx, maximum), in: total).rounded()
    }
}
