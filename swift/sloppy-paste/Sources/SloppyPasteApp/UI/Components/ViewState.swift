import SwiftUI

/// Use `@ViewState` instead of `@State`. The Command Line Tools SDK declares a
/// `State` macro that shadows the property wrapper, but ships no SwiftUIMacros
/// plugin, so `@State` fails to build. Going through a typealias reaches the
/// property wrapper directly.
typealias ViewState<Value> = SwiftUI.State<Value>
