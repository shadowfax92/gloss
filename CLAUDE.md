# CLAUDE.md

This file provides guidance to Claude Code when working with code in this repository.

## Project Overview

`gloss` is a Go CLI that opens a folder of markdown files in a browser as a GitHub-style file tree + rendered preview, with a one-click "copy with file:line ref" highlight feature for pasting into AI coding agents. A self-spawning persistent daemon backs every invocation so one URL reuses one browser tab across many `gloss` runs.

## Build & Run

```sh
make build       # builds ./gloss with version stamped
make install     # builds and copies to ~/bin/gloss + codesigns
```

There are no automated tests — manual smoke tests only. The module name is `gloss` (not a URL-style module path), matching the bare-name convention used by `grove`.

## Architecture

**CLI layer** (`cmd/`): Cobra commands — `serve` (default), `recent`, `highlights`, `daemon`, `config`, `version`. Bare `gloss [path]` is equivalent to `gloss serve [path]`. The hidden `_serve` subcommand is the detached daemon entry point that the client spawns.

**Internal packages** (`internal/`):
- `paths/` — XDG-aware helpers for `~/.config/gloss/`, `~/.local/state/gloss/`, `~/.local/share/gloss/`.
- `config/` — YAML config at `~/.config/gloss/config.yaml`. Created with defaults + header comment on first run.
- `highlights/` — NDJSON store at `~/.local/share/gloss/highlights.json`. ULID ids. `flock`-protected writes. `IsStale()` re-reads source to detect line shifts.
- `recent/` — Reads `~/.local/share/recent-files/history.tsv` directly (the existing sibling CLI's data file). Filters to `*.md` / `*.markdown`. 5 s in-process cache.
- `walk/` — Markdown filesystem walker with depth (8) / count (5000) caps and an ignore list (`node_modules`, `.git`, `.next`, `dist`, `vendor`, `target`).
- `render/` — Goldmark + chroma + custom block renderer that stamps every block element with `data-line-start` / `data-line-end`. Used by the server to make text selections in the browser map back to source line ranges.
- `watch/` — `fsnotify` wrapper, 100 ms debounce, falls back to 1 s `os.Stat` polling on watcher errors. Caps watched folders at 32 per daemon.
- `server/` — Long-lived `*http.Server`, in-memory open-folders map, SSE hub, route handlers, the daemon spawn-or-reuse logic for the client. Token-gated API (32-byte hex random per daemon start).
- `reltime/` — Tiny "5m / 2h / 3d" helper, copy of grove's `state.RelativeTime`.

**Frontend** (`web/`): One `index.html`, one `style.css`, one `app.js` (~400 lines vanilla JS), plus a vendored `github-markdown.css`. Embedded via `embed.FS`. No build step, no node, no React.

## Data flow

```
gloss .
   │
   ├─ flock(~/.local/state/gloss/server.lock)
   ├─ read server.json → live? POST /api/open : spawn _serve --detached
   └─ open http://127.0.0.1:<port>/?folder=<id>&file=<rel>&t=<token>

gloss _serve --detached  (one process, lives forever)
   ├─ HTTP @ 127.0.0.1:0
   ├─ goldmark renderer with line-stamp extension
   ├─ fsnotify watchers → SSE → live reload
   ├─ highlights NDJSON store
   └─ recent-files TSV reader
```

## State files

| Path                                             | Owner             | Purpose                                |
|--------------------------------------------------|-------------------|----------------------------------------|
| `~/.config/gloss/config.yaml`                    | gloss             | User config                            |
| `~/.local/state/gloss/server.json`               | gloss daemon      | `{pid, port, started_at, token}`       |
| `~/.local/state/gloss/server.lock`               | gloss client      | flock for safe spawn                   |
| `~/.local/state/gloss/daemon.log`                | gloss daemon      | stdout+stderr                          |
| `~/.local/share/gloss/highlights.json`           | gloss daemon + cli| NDJSON, append store                   |
| `~/.local/share/recent-files/history.tsv`        | recent-files cli  | **read-only** by gloss                 |

## Key patterns

- All highlight mutations go through the daemon, but the file is also `flock`'d for safety in case `gloss highlights rm <id>` runs from a separate process.
- The daemon writes `server.json` atomically (write-temp + rename) before binding `127.0.0.1:0`, so the client polling loop sees a consistent file.
- Every `/api/*` request requires `X-Gloss-Token: <hex>` (or `?t=<hex>` for the SSE EventSource which can't set headers). Constant-time compare. The token is regenerated on each daemon start.
- The custom Goldmark renderer uses `node.Lines()` segments to compute 1-based `[start, end]` for each block element. Inline marks pass through unchanged.
- `gloss serve --foreground` is the escape hatch: runs the server attached to the current terminal, bypassing self-daemonization. Use it when debugging anything daemon-related.

## Don't

- Don't add per-folder highlight stores. Highlights are global. The user explicitly asked for one place.
- Don't expose the server beyond `127.0.0.1`. Single-user, single-machine.
- Don't add a JS framework. The frontend stays vanilla.
- Don't add automated tests for v1 unless something gets fragile (the line-stamping renderer is the candidate).
