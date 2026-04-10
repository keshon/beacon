# Beacon UI kit (SCSS)

This folder contains the source-of-truth styles for Beacon.

The application still serves CSS from `static/uikit.css`, but that file should be treated as a generated artifact from these SCSS sources.

## Requirements
- `sass` (Dart Sass) available in your `PATH`

## Commands

### One-shot build (SCSS → CSS)

Linux:

```bash
bash scripts/uikit-build.sh
```

Windows:

```bat
scripts\uikit-build.bat
```

### Watch mode (rebuild on changes)

Linux:

```bash
bash scripts/uikit-watch.sh
```

Windows:

```bat
scripts\uikit-watch.bat
```

## Dev workflow
1. Start the app (as you do now).
2. In a second terminal start the SCSS watcher.
3. Edit files in `uikit/scss/`.
4. Refresh the page to see changes.

## Files
- `uikit/scss/uikit.scss`: SCSS entrypoint
- `uikit/scss/_*.scss`: partials (tokens, base, layout, nav, components, pages)
- `static/uikit.css`: generated output consumed by templates

