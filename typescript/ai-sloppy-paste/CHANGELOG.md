# Changelog

All notable changes to the AI Sloppy Paste extension will be documented in this file.

## [2.1.0] - Paste-First Update

### Breaking Changes

- **Default Action Changed**: Primary action (⌘ + Return) now pastes content to frontmost app instead of copying to clipboard
  - **⌘ + Return**: Now pastes to frontmost app (was copy to clipboard)
  - **⌘ + Option + Return**: Now copies to clipboard (was paste to frontmost app)
- **Removed Action**: "Copy (Keep Window Open)" action has been removed to simplify the interface

### Rationale

This change makes the extension more intuitive for its primary use case: quickly pasting snippets into applications. The most common action is now just one keystroke away (⌘ + Return), while copying to clipboard remains available as a secondary action.

## [2.0.0] - Major Update

### Breaking Changes

- **Multi-Tag System**: Migrated from single category to multiple tags per snippet
    - Existing snippets automatically migrated (category → single tag)
    - Storage format updated to v2 with versioned schema

### New Features

- **Multi-Tag Support**: Organize snippets with multiple tags for flexible categorization
- **Dynamic Placeholders**: Create snippet templates with `{{variable}}` or `{{variable|default}}` syntax
- **Usage Tracking**: Track how often each snippet is used and when it was last used
- **Favorites System**: Mark important snippets as favorites with ⭐
- **Storage Monitoring**: View storage usage and get warnings when approaching limits
- **Enhanced Copy Actions**: Copy with window close or keep window open for multiple operations
- **Placeholder Form**: Interactive form to fill placeholder values before copying
- **Improved Validation**: Character limits with real-time feedback and visual indicators

### Enhanced Features

- **Better Search**: Search across title, content, AND tags simultaneously
- **Tag Management**: View and delete tags (with cascade to all snippets)
- **Export/Import**: Enhanced with merge support and better error handling
- **Detail View**: Shows comprehensive metadata including usage statistics
- **Timestamps**: Now tracks creation, modification, and last use times
- **Placeholder Pre-selection**: Last-used value now pre-selected in placeholder forms for improved workflow continuity (dropdown list still uses smart ranking)

### Keyboard Shortcuts (Updated)

- `⌘ + N`: Create new snippet
- `⌘ + E`: Edit selected snippet
- `⌘ + Enter`: Copy content to clipboard (closes window)
- `⌘ + Shift + Enter`: Copy content (keep window open)
- `⌃ + X`: Delete snippet
- `⌘ + C`: Copy title
- `⌘ + D`: Toggle detail view
- `⌘ + F`: Toggle favorites view
- `⌘ + Shift + F`: Toggle favorite status
- `⌘ + T`: Manage tags
- `⌘ + Shift + E`: Export all snippets
- `⌘ + Shift + I`: Import snippets
- `⌘ + Shift + S`: View storage info

### Technical Details

- Built with React + TypeScript
- Uses Raycast API v1.103.4 (upgraded from v1.83.2)
- Versioned LocalStorage with automatic migrations
- Comprehensive TypeScript validation
- Character limits: Title (200), Content (100KB), Tags (50)
- Placeholder syntax support with validation
- Migration system for future schema changes

### Data Migration

- Automatic migration from v1 to v2 on first load
- Legacy storage automatically converted and cleaned up
- All existing snippets preserved with enhanced features
- Migration logged to console for verification

## [1.0.0] - Initial Release

### Features

- **Snippet Management**: Create, edit, and delete text snippets
- **Category Organization**: Organize snippets with custom categories/tags
- **Smart Search**: Fuzzy search across all snippets with real-time filtering
- **Category Filter**: Quick category dropdown in search bar
- **Detail View**: Preview long snippets with full metadata
- **Timestamps**: Automatic tracking of creation and modification times
- **Export/Import**: Backup and restore your snippets to/from JSON files
- **Keyboard Shortcuts**: Comprehensive keyboard shortcuts for all actions

### Keyboard Shortcuts

- `⌘ + N`: Create new snippet
- `⌘ + E`: Edit selected snippet
- `⌘ + Enter`: Copy content to clipboard
- `⌃ + X`: Delete snippet
- `⌘ + C`: Copy title
- `⌘ + Shift + C`: Copy full content
- `⌘ + D`: Toggle detail view
- `⌘ + T`: Manage categories
- `⌘ + Shift + E`: Export all snippets
- `⌘ + Shift + I`: Import snippets

### Technical Details

- Built with React + TypeScript
- Uses Raycast API v1.83.2
- LocalStorage for data persistence
- Full form validation
- Error handling and user feedback
- Merge or replace options for imports
- Automatic category management
