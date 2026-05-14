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
    uikit.scss          # Entrypoint: variables -> bootstrap -> tokens -> custom
    _variables.scss     # Bootstrap $variable overrides (colors, fonts, radii, card)
    _tokens.scss        # CSS custom properties not covered by Bootstrap
    _base.scss          # Scrollbar, animations
    _layout.scss        # Container, layout wrappers, page-title
    _nav.scss           # .dash-nav navbar override
    _components.scss    # Card, table, badge, button, form overrides + domain classes
    _pages.scss         # Login page
  vendor/
    bootstrap/scss/     # Bootstrap 5.3.3 SCSS source (not committed, download via script)
```

## Class naming

Standard Bootstrap class names are used wherever possible (`.card`, `.table`, `.btn-primary`, `.btn-outline-*`, `.badge`, `.alert`, etc.). Domain-specific classes (`.dash-nav`, `.monitor-row`, `.empty-state`, `.section-label`, etc.) are kept where Bootstrap has no equivalent.

## Style stack decision

**Current choice (path A):** stay on **Bootstrap 5.3 SCSS + Beacon partials** in `uikit/scss/`, compiled to `static/uikit.css`. Theming and density evolve via CSS variables (`_tokens.scss`, Bootstrap `$variables` overrides) and named domain classes (e.g. `.monitor-title`, `.dash-nav`).

**Not planned in-repo right now (path B):** a migration to **Tailwind** would mean replacing all template utility usage, adding a PostCSS/Tailwind build, and replacing Bootstrap’s JS behaviors (navbar collapse, etc.) — a separate project when product goals justify it.
