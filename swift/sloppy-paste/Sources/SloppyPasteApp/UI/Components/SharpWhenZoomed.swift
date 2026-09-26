import Observation
import SwiftUI

/// The picker's current zoom level, for views that draw differently when zoomed.
@MainActor
@Observable
final class PickerZoom {
    var level: CGFloat = 1
}

extension View {
    /// SwiftUI rasterises SF Symbols at the screen's scale, ignoring the
    /// panel's zoom, so they blur when the picker is zoomed in. A drawing group
    /// renders them at the panel's zoomed backing scale instead. Unzoomed,
    /// symbols draw as usual and keep their vibrancy.
    func sharpWhenZoomed() -> some View {
        modifier(SharpWhenZoomed())
    }
}

private struct SharpWhenZoomed: ViewModifier {
    @Environment(PickerZoom.self) private var zoom: PickerZoom?

    func body(content: Content) -> some View {
        if let zoom, zoom.level != 1 {
            content.drawingGroup()
        } else {
            content
        }
    }
}
