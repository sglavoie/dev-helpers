import Foundation
import CoreGraphics

/// Where the picker and the HUD go on a multi-display setup, in AppKit's
/// global screen coordinates (origin at the bottom left of the main display).
public enum PanelPlacement {
    public struct Screen: Sendable, Hashable {
        public var frame: CGRect
        /// The frame minus the menu bar and the Dock.
        public var visibleFrame: CGRect

        public init(frame: CGRect, visibleFrame: CGRect) {
            self.frame = frame
            self.visibleFrame = visibleFrame
        }
    }

    /// Space kept free around a panel that is shrunk to fit a small screen.
    public static let margin: CGFloat = 20

    /// The screen containing `point` (the mouse), using the same edge rule as
    /// `NSMouseInRect` for unflipped coordinates: the left and top edges are
    /// inside, the right and bottom edges are not. A point outside every
    /// screen picks the nearest one; nil only when there are no screens.
    public static func screen(containing point: CGPoint, in screens: [Screen]) -> Screen? {
        if let hit = screens.first(where: { contains($0.frame, point) }) {
            return hit
        }
        return screens.min { distance(from: point, to: $0.frame) < distance(from: point, to: $1.frame) }
    }

    /// `preferred`, shrunk where needed to fit `visible` with `margin` on each side.
    public static func fittedSize(_ preferred: CGSize, in visible: CGRect) -> CGSize {
        CGSize(
            width: min(preferred.width, max(visible.width - 2 * margin, 0)),
            height: min(preferred.height, max(visible.height - 2 * margin, 0)))
    }

    /// Centres `size` horizontally in `visible`, with its bottom edge at
    /// `verticalFraction` of the free height, and keeps it on screen.
    public static func origin(for size: CGSize, in visible: CGRect, verticalFraction: CGFloat) -> CGPoint {
        let x = visible.midX - size.width / 2
        let y = visible.minY + (visible.height - size.height) * verticalFraction
        return CGPoint(
            x: clamp(x, visible.minX, visible.maxX - size.width),
            y: clamp(y, visible.minY, visible.maxY - size.height))
    }

    /// The frame for a panel of `preferred` size on the screen with `mouse`.
    public static func frame(
        preferred: CGSize, mouse: CGPoint, screens: [Screen], verticalFraction: CGFloat
    ) -> CGRect? {
        guard let visible = screen(containing: mouse, in: screens)?.visibleFrame else { return nil }
        let size = fittedSize(preferred, in: visible)
        return CGRect(origin: origin(for: size, in: visible, verticalFraction: verticalFraction), size: size)
    }

    private static func contains(_ rect: CGRect, _ point: CGPoint) -> Bool {
        point.x >= rect.minX && point.x < rect.maxX && point.y > rect.minY && point.y <= rect.maxY
    }

    private static func distance(from point: CGPoint, to rect: CGRect) -> CGFloat {
        let dx = max(rect.minX - point.x, 0, point.x - rect.maxX)
        let dy = max(rect.minY - point.y, 0, point.y - rect.maxY)
        return (dx * dx + dy * dy).squareRoot()
    }

    /// Prefers `low` when the range is empty (a panel larger than the screen).
    private static func clamp(_ value: CGFloat, _ low: CGFloat, _ high: CGFloat) -> CGFloat {
        max(low, min(value, high))
    }
}
