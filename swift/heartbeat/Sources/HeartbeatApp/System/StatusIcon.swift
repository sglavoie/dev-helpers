import AppKit
import HeartbeatCore

/// The menu-bar heart. The shape changes along with the color so the state reads without color vision:
/// ok is the plain template `heart`, amber an orange `heart.fill`, red a red `heart.slash.fill` plus the
/// number of failing agents.
@MainActor
enum StatusIcon {
    struct Appearance: Equatable {
        var symbol: String
        /// `nil` keeps the image a template that follows the menu bar's appearance.
        var color: NSColor?
        var title: String
        var description: String
    }

    static func appearance(_ overall: OverallStatus?, failing: Int) -> Appearance {
        switch overall {
        case nil:
            Appearance(symbol: "heart", color: nil, title: "", description: "Heartbeat: checking")
        case .ok?:
            Appearance(symbol: "heart", color: nil, title: "", description: "Heartbeat: all ok")
        case .warning?:
            Appearance(symbol: "heart.fill", color: .systemOrange, title: "", description: "Heartbeat: warning")
        case .unknown?:
            Appearance(symbol: "heart.fill", color: .systemOrange, title: "", description: "Heartbeat: cannot check")
        case .failing?:
            Appearance(symbol: "heart.slash.fill", color: .systemRed, title: failing > 0 ? "\(failing)" : "",
                       description: "Heartbeat: \(failing) failing")
        }
    }

    static func apply(_ appearance: Appearance, to button: NSStatusBarButton) {
        var image = NSImage(systemSymbolName: appearance.symbol, accessibilityDescription: appearance.description)
        if let color = appearance.color {
            image = image?.withSymbolConfiguration(NSImage.SymbolConfiguration(paletteColors: [color]))
            image?.isTemplate = false
        } else {
            image?.isTemplate = true
        }
        button.image = image
        button.imagePosition = appearance.title.isEmpty ? .imageOnly : .imageLeading
        if let color = appearance.color, !appearance.title.isEmpty {
            button.attributedTitle = NSAttributedString(
                string: appearance.title,
                attributes: [.foregroundColor: color, .font: NSFont.menuBarFont(ofSize: 0)])
        } else {
            button.title = appearance.title
        }
        button.setAccessibilityLabel(appearance.description)
    }
}
