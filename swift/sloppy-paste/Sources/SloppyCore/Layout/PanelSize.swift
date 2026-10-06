import Foundation
import CoreGraphics

/// The picker's size in points at 100% zoom, set by dragging its borders or
/// in Settings. The panel on screen is this size times the zoom level.
public enum PanelSize {
    public static let defaultSize = CGSize(width: 760, height: 480)
    public static let minimum = CGSize(width: 600, height: 360)
    public static let maximum = CGSize(width: 3000, height: 2000)

    /// A stored size clamped to `minimum...maximum`, each dimension falling
    /// back to the default when missing or not a usable number.
    public static func sanitized(width: Double?, height: Double?) -> CGSize {
        CGSize(
            width: sanitized(width, default: defaultSize.width, min: minimum.width, max: maximum.width),
            height: sanitized(height, default: defaultSize.height, min: minimum.height, max: maximum.height))
    }

    /// The borders being dragged. Opposite borders move together, so the
    /// panel grows or shrinks symmetrically around its centre.
    public struct Edges: OptionSet, Sendable, Hashable {
        public let rawValue: Int
        public init(rawValue: Int) { self.rawValue = rawValue }

        public static let left = Edges(rawValue: 1 << 0)
        public static let right = Edges(rawValue: 1 << 1)
        public static let top = Edges(rawValue: 1 << 2)
        public static let bottom = Edges(rawValue: 1 << 3)
    }

    /// `start` resized by dragging `edges` by `delta` (in AppKit screen
    /// coordinates, y up): the dragged border follows the pointer and the
    /// opposite one mirrors it. The size stays within `minimum...maximum`
    /// (already scaled to the zoom) and the frame inside `visible`.
    public static func dragged(
        _ start: CGRect, edges: Edges, by delta: CGVector,
        minimum: CGSize, maximum: CGSize, in visible: CGRect
    ) -> CGRect {
        var width = start.width
        var height = start.height
        if edges.contains(.right) { width += 2 * delta.dx }
        if edges.contains(.left) { width -= 2 * delta.dx }
        if edges.contains(.top) { height += 2 * delta.dy }
        if edges.contains(.bottom) { height -= 2 * delta.dy }
        let size = CGSize(
            width: max(minimum.width, min(width, maximum.width)),
            height: max(minimum.height, min(height, maximum.height)))
        return PanelZoom.resized(start, to: size, in: visible)
    }

    private static func sanitized(_ value: Double?, default fallback: CGFloat, min low: CGFloat, max high: CGFloat) -> CGFloat {
        guard let value, value.isFinite, value > 0 else { return fallback }
        return Swift.max(low, Swift.min(CGFloat(value.rounded()), high))
    }
}
