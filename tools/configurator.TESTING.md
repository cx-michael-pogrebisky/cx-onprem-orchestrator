# Configuration builder — testing & browser support

The builder ([configurator.html](configurator.html)) is a single self-contained
HTML file (vanilla JS, no dependencies, no network). It is **generated** from the
live CLI schema by `go run ./hack/gen-configurator`; never edit `configurator.html`
by hand — edit `configurator.template.html` and regenerate.

## How to run the tests

```bash
# 1. Logic tests (V8 — the Chrome/Edge engine), no browser needed:
node --test tools/configurator.test.mjs

# 2. Cross-engine: logic + headless Chrome/Edge (Blink) + headless Firefox (Gecko):
./tools/run-browser-tests.sh

# 3. Any browser, by hand — open the page with the #selftest fragment:
#    file:///…/tools/configurator.html#selftest
#    A green "SELFTEST PASS n/n" banner = good; red = failure details.
```

`run-browser-tests.sh` skips (does not fail) any browser that isn't installed, so
CI runs the subset available. CI runs stages 1–2 on every push (see
`.github/workflows/ci.yml`).

## Automated coverage

| Layer | Engine | What it checks | Where |
|---|---|---|---|
| Logic tests | V8 (Chrome/Edge) | 18 cases: every runtime, every CI target (linux + windows), flag wiring, secret-by-name-only, validations | `configurator.test.mjs` |
| In-page self-test | **Blink** (Chrome, Edge) | 10 scenarios in the real DOM via `--dump-dom #selftest` | `run-browser-tests.sh` |
| In-page self-test | **Gecko** (Firefox) | same 10 scenarios; pass/fail read from the banner colour in a headless screenshot | `run-browser-tests.sh` |

> The 10 in-page scenarios and the 18 Node cases overlap by design — the Node suite
> is the detailed contract; the in-page self-test proves the **real DOM/CSS/JS**
> works in each browser engine.

## Browser support

Verified passing: **Chrome**, **Microsoft Edge** (both Chromium/Blink), and
**Firefox** (Gecko) — desktop and mobile widths (responsive single column below
920 px; tested at 390 px). **Safari / iOS Safari** (WebKit) can't be automated in
this repo's CI image; it is covered by the manual checklist below — and because the
page uses only Baseline web features (see audit), it behaves identically.

### Feature/compatibility audit

Every API and CSS feature used is **Baseline / widely available** (Chrome, Edge,
Firefox, Safari, and their mobile builds):

- **JS:** ES2018+ only — `const`/`let`, arrow functions, template literals, spread,
  `Set`, `Array.filter/map/reduce`, object getters/setters, `async/await`. No
  optional chaining is required at runtime; no modules; no top-level await.
- **Clipboard:** uses the async Clipboard API **only in a secure context**, with a
  `<textarea>` + `document.execCommand('copy')` **fallback** for `file://` and older
  Safari/Edge — so Copy works whether opened locally or from GitHub Pages.
- **CSS:** Flexbox, CSS Grid, custom properties (`--vars`), `position: sticky`,
  `@media` — all Baseline. Layout collapses to one column under 920 px.
- **No** WebComponents, no fetch/XHR, no service workers, no external fonts/CDNs.

## Manual cross-browser / mobile checklist

Open `configurator.html` (locally or the Pages URL) and confirm:

- [ ] **Chrome / Edge / Firefox / Safari (desktop):** page loads; changing Target /
      OS / Runtime updates the output live; `#selftest` shows a green PASS banner.
- [ ] **iOS Safari / Android Chrome:** single-column layout; all controls usable;
      the output `<pre>` scrolls horizontally; **Copy** works (tap Copy → paste).
- [ ] **Project & team** fields appear at the top; setting them adds
      `--project-name` / `--sast-team` to the output.
- [ ] **Windows + Docker/Podman** shows the WSL2 / Server 2016 caveat note.
- [ ] **Download** saves the snippet with the CI's filename.
- [ ] No secret **values** ever appear — only env-var **names**.
