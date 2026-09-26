import SwiftUI

/// An in-panel confirmation (an NSAlert would take focus from the target
/// app). The host screen routes ↵ and ⎋ to `confirm` and `cancel`.
struct ConfirmationOverlay: View {
    var title: String
    var message: String
    var confirmTitle: String
    var isDestructive = true
    var confirm: @MainActor () -> Void
    var cancel: @MainActor () -> Void

    var body: some View {
        ZStack {
            Color.black.opacity(0.25)
                .onTapGesture(perform: cancel)
            VStack(alignment: .leading, spacing: 10) {
                Text(title).font(.headline)
                Text(message)
                    .foregroundStyle(.secondary)
                    .fixedSize(horizontal: false, vertical: true)
                HStack {
                    Spacer()
                    Button("Cancel  ⎋", action: cancel)
                    Button("\(confirmTitle)  ↵", action: confirm)
                        .tint(isDestructive ? .red : .accentColor)
                        .buttonStyle(.borderedProminent)
                }
            }
            .padding(18)
            .frame(width: 380)
            .background(.regularMaterial, in: RoundedRectangle(cornerRadius: 10))
            .shadow(radius: 12)
        }
    }
}

/// Shows the `ToastCenter`'s current toast at the bottom of the panel.
struct ToastOverlay: View {
    @Environment(ToastCenter.self) private var toasts

    var body: some View {
        VStack {
            Spacer()
            if let toast = toasts.current {
                HStack(spacing: 8) {
                    Image(systemName: toast.style == .success ? "checkmark.circle.fill" : "xmark.octagon.fill")
                        .foregroundStyle(toast.style == .success ? .green : .red)
                    VStack(alignment: .leading, spacing: 1) {
                        Text(toast.title).fontWeight(.semibold)
                        if let message = toast.message {
                            Text(message).font(.caption).foregroundStyle(.secondary).lineLimit(2)
                        }
                    }
                }
                .padding(.horizontal, 14)
                .padding(.vertical, 8)
                .background(.thickMaterial, in: Capsule())
                .shadow(radius: 6)
                .padding(.bottom, 44)
                .transition(.move(edge: .bottom).combined(with: .opacity))
                .onTapGesture { toasts.dismiss() }
                .id(toast.id)
            }
        }
        .frame(maxWidth: .infinity)
        .animation(.easeOut(duration: 0.18), value: toasts.current)
    }
}
