ext_dir_map = {
    # No name
    ("noname",): ("Other", "Uncategorized"),
    # Audio
    (
        ".aif",
        ".cda",
        ".m3u",
        ".mid",
        ".midi",
        ".mp3",
        ".mpa",
        ".ogg",
        ".opus",
        ".wav",
        ".wma",
        ".wpl",
    ): ("Media", "Audio"),
    # Text
    (".txt", ".rtf", ".tex", ".wks", ".wps", ".wpd", ".odt", ".wiki"): (
        "Text",
        "TextFiles",
    ),
    (".doc", ".docx"): ("Text", "Word"),
    (".pdf",): ("Text", "PDF"),
    (".sla",): ("Text", "Resume"),
    # Video
    (
        ".3g2",
        ".3gp",
        ".avi",
        ".flv",
        ".h264",
        ".m4v",
        ".mkv",
        ".mov",
        ".mp4",
        ".mpg",
        ".mpeg",
        ".rm",
        ".swf",
        ".vob",
        ".wmv",
    ): ("Media", "Video"),
    # Images
    (
        ".CR2",
        ".ai",
        ".bmp",
        ".gif",
        ".ico",
        ".jpeg",
        ".jpg",
        ".png",
        ".ps",
        ".psd",
        ".svg",
        ".tif",
        ".tiff",
        ".xcf",
    ): ("Media", "Images"),
    # Internet
    (
        ".asp",
        ".aspx",
        ".cer",
        ".cfm",
        ".cgi",
        ".pl",
        ".css",
        ".htm",
        ".js",
        ".jsp",
        ".part",
        ".php",
        ".rss",
        ".xhtml",
    ): ("Other", "Internet"),
    # Compressed
    (".7z", ".arj", ".deb", ".pkg", ".rar", ".rpm", ".tar.gz", ".z", ".zip"): (
        "Compressed",
    ),
    # Disc
    (".bin", ".dmg", ".iso", ".toast", ".vcd"): ("Other", "Disc"),
    # Data
    (
        ".csv",
        ".dat",
        ".db",
        ".dbf",
        ".log",
        ".mdb",
        ".sav",
        ".sql",
        ".tar",
        ".xml",
        ".json",
    ): ("Programming", "Database"),
    # Executables
    (".apk", ".bat", ".com", ".exe", ".gadget", ".jar", ".wsf"): (
        "Executables",
    ),
    # Fonts
    (".fnt", ".fon", ".otf", ".ttf", ".woff", ".woff2"): ("Fonts"),
    # Presentations
    (".key", ".odp", ".pps", ".ppt", ".pptx"): ("Text", "Presentations"),
    # Programming
    (".c", ".class", ".dart", ".py", ".sh", ".swift", ".html", ".h"): (
        "Programming",
        "SourceCode",
    ),
    # Spreadsheets
    (".ods", ".xlr", ".xls", ".xlsx"): ("Text", "Spreadsheets"),
    # System
    (
        ".bak",
        ".cab",
        ".cfg",
        ".cpl",
        ".cur",
        ".dll",
        ".dmp",
        ".drv",
        ".icns",
        ".ico",
        ".ini",
        ".lnk",
        ".msi",
        ".sys",
        ".tmp",
    ): ("Text", "Other", "System"),
    # Security
    (".gpg", "kdb", "kdbx", ".asc", ".kbx", ".key", ".pub"): ("Security",),
}
