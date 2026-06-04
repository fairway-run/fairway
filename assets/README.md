# Brand assets

This directory holds the repo's visual identity. The mark, lockup, and social card are also embedded in or referenced from product surfaces — when you change one, check the others.

| File | Purpose | Dimensions |
|---|---|---|
| `logo.svg` | Canonical mark. Monochrome `currentColor`. Use everywhere a small mark is needed. | 24 × 24 |
| `logo-lockup.svg` | Mark + wordmark for README headers. Monochrome `currentColor`. | 280 × 56 |
| `social-card.svg` | GitHub social preview source. Color (`#0d1117` bg, `#c9d1d9` mark, `#58a6ff` middle-dot accent, `#e6edf3` wordmark, `#7d8590` tagline). | 1280 × 640 |
| `social-card.png` | Rendered social card for upload to GitHub. | 1280 × 640 |

## Where the mark is also used

- `internal/dashboard/assets/logo.svg` — embedded into the binary via `//go:embed`, served by the dashboard.
- `README.md` — references `assets/logo-lockup.svg`.

Keep `assets/logo.svg` and `internal/dashboard/assets/logo.svg` byte-identical until we have a reason to diverge.

## Regenerating the social card PNG

The SVG is source; the PNG is a build artifact suitable for GitHub upload. The renderer must preserve the 1280×640 aspect — `qlmanage` produces a square thumbnail, so don't use it.

```bash
# Preferred path 1 — headless Chrome (already installed on most dev macs)
CHROME="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
"$CHROME" --headless=new --disable-gpu --hide-scrollbars \
  --window-size=1280,640 \
  --screenshot=assets/social-card.png \
  "file://$PWD/assets/social-card.svg"

# Preferred path 2 — librsvg via Homebrew
brew install librsvg
rsvg-convert -w 1280 -h 640 assets/social-card.svg -o assets/social-card.png
```

## Design rationale

The mark is two vertical rails (the fairway channel walls) with three dots between them (tasks in transit through coordinated lanes). The social card preserves the same composition at scale, with the middle dot rendered in the GitHub-style accent blue (`#58a6ff`) to telegraph "active lane" — the only colored element on an otherwise monochrome composition. The wordmark uses the system sans-serif stack so it renders consistently across GitHub, browsers, and Slack/Twitter previews without shipping a font.

See [docs/design/sketches/logo/preview.html](../docs/design/sketches/logo/preview.html) for the original candidate sketches that led to this direction.
