# Beacon UI kit (SCSS + Bootstrap 5.3)

This folder contains the source-of-truth styles for Beacon. Bootstrap 5.3.3 SCSS source is compiled together with Beacon overrides into a single `static/uikit.css`.

## Requirements

- `sass` (Dart Sass) in your PATH
- Bootstrap SCSS source in `uikit/vendor/bootstrap/scss/` (see setup below)

## First-time setup

Download Bootstrap SCSS source (no npm required):

```bash
bash scripts/bootstrap-download.sh
```

Windows:

```bat
scripts\bootstrap-download.bat
```

## Commands

### One-shot build

```bash
bash scripts/uikit-build.sh
```

### Watch mode

```bash
bash scripts/uikit-watch.sh
```

## Dev workflow

1. Run `scripts/bootstrap-download.sh` once (if `uikit/vendor/` is empty).
2. Start the app.
3. In a second terminal start the SCSS watcher.
4. Edit files in `uikit/scss/`.
5. Refresh the page to see changes.

## File structure

```
uikit/
  scss/
    uikit.scss          # Entrypoint: variables → bootstrap → tokens → layers below
    _variables.scss     # Bootstrap $variable overrides (colors, $input-btn-*, card, …)
    _tokens.scss        # CSS variables: --app-* scale, --ui-font-size, html[data-bs-theme] chroma (--c-*)
    _base.scss          # html font-size, body smoothing, scrollbar, @keyframes
    _nav.scss           # .dash-nav + light-theme navbar overrides
    _components.scss    # Bootstrap bridges (.btn, .form-*, .table) + Beacon domain (badges, uptime, .monitor-row)
    _shell.scss         # .beacon-app chrome, .app-page, .app-panel, dashboard layouts
    _pages.scss         # Login page (.login-page)
  vendor/
    bootstrap/scss/     # Bootstrap 5.3.3 SCSS source (not committed, download via script)
```

## Class naming

Standard Bootstrap class names are used wherever possible (`.card`, `.table`, `.btn-primary`, `.btn-outline-*`, `.badge`, `.alert`, etc.). Domain-specific classes (`.dash-nav`, `.monitor-row`, `.empty-state`, `.section-label`, etc.) are kept where Bootstrap has no equivalent.

## Style stack decision

**Current choice (path A):** stay on **Bootstrap 5.3 SCSS + Beacon partials** in `uikit/scss/`, compiled to `static/uikit.css`. Theming and density evolve via CSS variables (`_tokens.scss`, Bootstrap `$variables` overrides) and named domain classes (e.g. `.monitor-title`, `.dash-nav`).

**App theme:** authenticated pages use `data-bs-theme` on `<html>` (`dark` or `light`). Preference is stored in `localStorage` under key `beaconTheme` and applied before paint via [`templates/beacon_head_theme.html`](templates/beacon_head_theme.html). Toggle control lives in the app navbar ([`templates/base.html`](templates/base.html)).

**Dashboard layout:** `localStorage` key `beaconDashboardView` (`cards` | `list` | `table`; legacy `grid` maps to `list`) is mirrored to `data-dashboard-view` on `<html>` in the same early script, so the chosen view paints without a flash. Base font size from Settings (`uiFontSize` → `--ui-font-size` on `<html>` in [`templates/head_common.html`](templates/head_common.html)) scales the UI together with typography tokens `--app-text-*` in [`uikit/scss/_tokens.scss`](scss/_tokens.scss).

**Not planned in-repo right now (path B):** a migration to **Tailwind** would mean replacing all template utility usage, adding a PostCSS/Tailwind build, and replacing Bootstrap’s JS behaviors (navbar collapse, etc.) — a separate project when product goals justify it.
