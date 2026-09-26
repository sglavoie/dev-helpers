import Foundation
import Observation

/// In-panel toasts ("Snippet pinned", "Failed to save snippet"). Unlike the
/// HUD, these draw inside the picker panel, so they never take focus away.
@MainActor
@Observable
final class ToastCenter {
    struct Toast: Equatable {
        enum Style { case success, failure }

        var style: Style
        var title: String
        var message: String?
        var id = UUID()
    }

    private(set) var current: Toast?

    func success(_ title: String, message: String? = nil) {
        show(Toast(style: .success, title: title, message: message))
    }

    func failure(_ title: String, message: String? = nil) {
        show(Toast(style: .failure, title: title, message: message))
    }

    func dismiss() {
        current = nil
    }

    private func show(_ toast: Toast) {
        current = toast
        let duration: Duration = toast.message == nil ? .seconds(1.6) : .seconds(3)
        Task { @MainActor [weak self] in
            try? await Task.sleep(for: duration)
            if self?.current?.id == toast.id { self?.current = nil }
        }
    }
}
