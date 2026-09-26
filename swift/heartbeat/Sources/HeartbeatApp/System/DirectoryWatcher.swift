import Foundation

/// Calls `onChange` on the main actor, debounced, when anything under a set of watched paths may have changed.
/// Adapted from Sloppy Paste's `FileWatcher`: a directory sees entries added, removed and atomically replaced
/// (a new inode), while an in-place write only touches the file itself. So callers pass both the directories
/// and the files they care about, and every source is re-opened after each change.
@MainActor
final class DirectoryWatcher {
    private let debounce: TimeInterval
    private let onChange: @MainActor () -> Void
    private var paths: Set<String> = []
    private var sources: [String: DispatchSourceFileSystemObject] = [:]
    private var pending: DispatchWorkItem?

    init(debounce: TimeInterval = 0.5, onChange: @escaping @MainActor () -> Void) {
        self.debounce = debounce
        self.onChange = onChange
    }

    isolated deinit {
        pending?.cancel()
        sources.values.forEach { $0.cancel() }
    }

    /// Replaces the watched set. Paths that don't exist yet are retried on the next change or call.
    func watch(_ newPaths: Set<String>) {
        guard newPaths != paths else {
            openMissing()
            return
        }
        for path in paths.subtracting(newPaths) {
            sources.removeValue(forKey: path)?.cancel()
        }
        paths = newPaths
        openMissing()
    }

    private func openMissing() {
        for path in paths where sources[path] == nil {
            sources[path] = makeSource(path: path)
        }
    }

    /// Replaced files and recreated directories have new inodes; watch those instead of the old ones.
    private func reopenAll() {
        sources.values.forEach { $0.cancel() }
        sources.removeAll()
        openMissing()
    }

    private func makeSource(path: String) -> DispatchSourceFileSystemObject? {
        let descriptor = open(path, O_EVTONLY)
        guard descriptor >= 0 else { return nil }
        let source = DispatchSource.makeFileSystemObjectSource(
            fileDescriptor: descriptor, eventMask: [.write, .extend, .rename, .delete, .attrib, .link], queue: .main)
        source.setEventHandler { [weak self] in
            MainActor.assumeIsolated { self?.scheduleChange() }
        }
        source.setCancelHandler { close(descriptor) }
        source.resume()
        return source
    }

    /// Editors and stow write in several steps; wait for them to settle.
    private func scheduleChange() {
        pending?.cancel()
        let item = DispatchWorkItem { [weak self] in
            MainActor.assumeIsolated {
                guard let self else { return }
                self.reopenAll()
                self.onChange()
            }
        }
        pending = item
        DispatchQueue.main.asyncAfter(deadline: .now() + debounce, execute: item)
    }
}
