# AI Sloppy Paste

A powerful Raycast extension for storing and quickly pasting text snippets with multi-tag support, placeholders, and usage tracking.

## Features

- 🏷️ **Hierarchical Tag System**: Organize snippets with hierarchical tags (e.g., `work/projects/client-a`)
  - Quick Add dropdown for selecting existing tags
  - TextField for creating new tags with full hierarchy support
  - Automatic lowercase normalization
  - Parent/child tag filtering
  - Visual hierarchy in all tag dropdowns and management view
- 🔍 **Smart Search & Filter**: Find snippets instantly with search across title, content, and tags
- 📊 **Usage Tracking**: See how often you use each snippet and when it was last used
- ⌨️ **Keyboard Shortcuts**: Lightning-fast actions for all operations
- 🎯 **Dynamic Placeholders**: Create snippet templates with `{{variable}}` syntax
- 👁️ **Detail View**: Preview long snippets with full metadata
- 💾 **Export/Import**: Backup and restore your snippets with merge support
- ✅ **Input Validation**: Character limits and real-time validation
- 💿 **Storage Monitoring**: Track storage usage with built-in indicator
- ⭐ **Favorites**: Quick access to your most important snippets
- 📦 **Archive System**: Hide unused snippets without deleting them
- ⏰ **Comprehensive Timestamps**: Track creation, modification, and last use

## Keyboard Shortcuts

- `⌘ + N`: Create new snippet
- `⌘ + E`: Edit selected snippet
- `⌘ + Shift + D`: Duplicate selected snippet
- `⌘ + Shift + A`: Archive/unarchive snippet
- `⌘ + Enter`: Copy content to clipboard (closes window)
- `⌘ + Option + Enter`: Paste content to frontmost app
- `⌘ + Shift + Enter`: Copy content (keep window open)
- `⌃ + X`: Delete snippet
- `⌘ + C`: Copy title
- `⌘ + D`: Toggle detail view
- `⌘ + A`: Toggle archived snippets view
- `⌘ + T`: Manage tags
- `⌘ + Shift + E`: Export all snippets
- `⌘ + Shift + I`: Import snippets
- `⌘ + Shift + S`: View storage info

## Using Tags

Tags allow you to organize snippets in multiple categories simultaneously with powerful hierarchy support:

- **Creating snippets**: Use the "Quick Add Tag" dropdown to select existing tags, or type directly in the Tags field
  - **Quick Add dropdown**: Click to add existing tags instantly (includes hierarchical tags)
  - **Tags field**: Type comma-separated tags for full control (e.g., `work/projects, personal, urgent`)
  - Create new hierarchical tags by typing them directly
- **Hierarchy**: Use slashes for hierarchical tags (e.g., `work/projects/client-a`)
  - Parent tag filters show all child snippets (selecting `work` shows `work`, `work/projects`, etc.)
  - Hierarchical display in tag dropdown and management view with indentation
  - Maximum 5 levels of hierarchy depth
- **Normalization**: Tags are automatically converted to lowercase for consistency
- **Validation**: Tags can contain letters, numbers, hyphens, underscores, and slashes (no spaces)
  - Use dashes instead of spaces (e.g., `my-project` not `my project`)
  - Examples: `work`, `work/projects`, `dev/backend/api`, `client-a`
- **Filtering**: Use the dropdown to filter by a specific tag (includes child tags)
- **Searching**: Search works across all tags automatically
- **Untagged**: Snippets without tags are labeled as "untagged"
- **Management**: Use ⌘+T to view, rename, merge, and delete tags with hierarchical tree view

## Using Placeholders

Create dynamic snippet templates with placeholder syntax:

- **Basic**: `{{name}}` - will prompt for value when copying
- **With default**: `{{name|John Doe}}` - optional with default value
- **Example**: `Hello {{name}}, your order {{order_id}} is ready!`

When you copy a snippet with placeholders, you'll be prompted to fill in the values.

## Usage Statistics

Every time you copy a snippet, the extension tracks:
- **Use Count**: How many times you've used this snippet
- **Last Used**: When you last copied this snippet

View these statistics in the detail view (⌘+D) to understand which snippets are most valuable.

## Storage Management

- Press ⌘+Shift+S to view storage usage
- The extension uses LocalStorage with ~5MB typical limit
- You'll see warnings when approaching storage limits
- Export/import feature helps manage large collections

## Setup

1. Install dependencies: `npm install`
2. Run development mode: `npm run dev`
3. Build for production: `npm run build`

## Development

Built with:

- React + TypeScript
- Raycast API v1.103.4
- Versioned LocalStorage for persistence
- Comprehensive TypeScript validation

## Attributions

<a href="https://www.flaticon.com/free-icons/paste-clipboard" title="paste clipboard icons">Paste clipboard icons created by Arkinasi - Flaticon</a>
