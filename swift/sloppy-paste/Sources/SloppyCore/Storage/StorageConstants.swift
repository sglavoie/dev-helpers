public enum StorageConstants {
    public static let currentVersion = 7

    /// Hard limit on values stored per placeholder key, separate from the UI display preference.
    public static let maxStoredValuesPerKey = 100

    /// Default number of ranked history values a placeholder field shows (Settings allows 5–100).
    public static let defaultMaxDisplayedHistoryValues = 20
}
