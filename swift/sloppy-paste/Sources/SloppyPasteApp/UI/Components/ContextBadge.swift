import AppKit
import SwiftUI
import SloppyCore

/// The round `ASL`-style badge for a title context, coloured like the extension.
struct ContextBadge: View {
    var context: String
    var size: CGFloat = 22

    var body: some View {
        let abbreviation = TitleContext.abbreviation(context)
        Circle()
            .fill(Color(TitleContext.color(context)))
            .frame(width: size, height: size)
            .overlay {
                Text(abbreviation)
                    .font(.system(size: TitleContext.badgeFontSize(forAbbreviation: abbreviation) * size / 32, weight: .bold))
                    .foregroundStyle(.white)
                    .lineLimit(1)
                    .minimumScaleFactor(0.5)
            }
            .help(context)
    }
}

/// A small rounded label, used for tags and filter chips.
struct Chip: View {
    var text: String
    var color: Color = .blue

    var body: some View {
        Text(text)
            .font(.caption)
            .lineLimit(1)
            .padding(.horizontal, 6)
            .padding(.vertical, 1)
            .foregroundStyle(color)
            .background(color.opacity(0.15), in: Capsule())
    }
}

extension Color {
    /// A colour that follows the appearance, from the core's light/dark hex pair.
    init(_ pair: TitleContext.Color) {
        let light = NSColor(hex: pair.light)
        let dark = NSColor(hex: pair.dark)
        self.init(nsColor: NSColor(name: nil) { appearance in
            appearance.bestMatch(from: [.aqua, .darkAqua]) == .darkAqua ? dark : light
        })
    }
}

extension NSColor {
    /// `#RRGGBB`; anything unparsable is grey.
    convenience init(hex: String) {
        let digits = hex.hasPrefix("#") ? String(hex.dropFirst()) : hex
        guard digits.count == 6, let value = UInt32(digits, radix: 16) else {
            self.init(white: 0.5, alpha: 1)
            return
        }
        self.init(
            srgbRed: CGFloat((value >> 16) & 0xFF) / 255,
            green: CGFloat((value >> 8) & 0xFF) / 255,
            blue: CGFloat(value & 0xFF) / 255,
            alpha: 1)
    }
}
