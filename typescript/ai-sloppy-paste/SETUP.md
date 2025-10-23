# Setup Instructions for AI Sloppy Paste

## Installation

1. Navigate to the extension directory:

   ```bash
   cd typescript/ai-sloppy-paste
   ```

2. Install dependencies:

   ```bash
   npm install
   ```

3. Start development mode:

   ```bash
   npm run dev
   ```

This will open Raycast and allow you to test the extension locally.

## Building for Production

To build the extension for distribution:

```bash
npm run build
```

## Usage

Once installed, you can:

1. Open Raycast (default: `⌘ + Space`)
2. Type "Manage Snippets" or "AI Sloppy Paste"
3. Start creating and managing your text snippets!

### First Time Setup

The first time you run the extension, it will have no snippets. Press `⌘+N` to create your first snippet.

### Keyboard Shortcuts

See README.md for the full list of keyboard shortcuts.

## Troubleshooting

If you encounter any issues:

1. Make sure all dependencies are installed: `npm install`
2. Try clearing Raycast cache and rebuilding
3. Check that the icon.png file exists in the root directory
4. Verify that package.json has the correct structure

## Publishing to Raycast Store

If you want to publish this extension:

```bash
npm run publish
```

Follow Raycast's extension publishing guidelines.
