# site

The GitHub Pages site: <https://ivanmicai.github.io/infra-shelf/>

A landing page plus the documentation, deployed by
[`.github/workflows/pages.yml`](../.github/workflows/pages.yml) on every push to
`main` that touches `site/`, `docs/`, `CONTRIBUTING.md` or the workflow itself.

## How it fits together

`build.mjs` renders `site/dist`:

- **Landing page** — `src/index.html`, hand-written. The hero is the shelf
  itself: three projects attached by hairlines to one dark board carrying six
  service units. Each unit is tinted by service, and that same hue reappears as
  the spine on the isolation table — colour identifies a service and is never
  decoration.
- **Docs pages** — rendered from the markdown that already lives in the
  repository, so there is one copy of each document and the site cannot drift
  from what shipped. The page list and sidebar order live in `PAGES` in
  `build.mjs`.

Relative links inside those markdown files are rewritten on the way out:
documents published here become site pages, and everything else points at the
file on GitHub. Heading anchors use GitHub's slug rules, so the tables of
contents already written into the markdown keep resolving.

The build fails on a broken internal link — a published doc leaking out as a
GitHub blob URL, a target that does not exist, or an anchor with no matching
`id`.

## Working on it

```bash
pnpm install
pnpm build
python3 -m http.server 8099 --directory dist
```

Then open <http://127.0.0.1:8099/>.

Only one dependency: `marked`, for markdown rendering. The landing page ships no
JavaScript — the load sequence is CSS animation, and it is disabled under
`prefers-reduced-motion`.

## Adding a docs page

1. Add the markdown under `docs/` in the repository root.
2. Add an entry to `PAGES` in `build.mjs` (order sets the sidebar).
3. Add the same file to `ROUTES` so links to it from other documents resolve to
   the site page rather than to GitHub.
