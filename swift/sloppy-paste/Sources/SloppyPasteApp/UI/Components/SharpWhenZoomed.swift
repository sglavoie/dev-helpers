import Observation
import SwiftUI

/// The picker's current zoom level, for views that draw differently when zoomed.
@MainActor
@Observable
final class PickerZoom {
    var level: CGFloat = 1
}

extension View {
    /// Draws the picker's content at its zoom level: it lays out in the
    /// unzoomed size and a scale effect enlarges it to fill the panel.
    /// SwiftUI maps clicks through a scale effect, unlike AppKit bounds
    /// scaling, where clicks landed at unzoomed positions.
    func pickerZoom() -> some View {
        modifier(PickerZoomEffect())
    }

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

private struct PickerZoomEffect: ViewModifier {
    @Environment(PickerZoom.self) private var zoom: PickerZoom?

    func body(content: Content) -> some View {
        let level = zoom?.level ?? 1
        GeometryReader { proxy in
            content
                .frame(width: proxy.size.width / level, height: proxy.size.height / level)
                .scaleEffect(level, anchor: .topLeading)
        }
    }
}
