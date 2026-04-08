<div align="center">

# ✨ gloss

**Browser markdown viewer with one-click highlight-to-clipboard for AI agent feedback loops.**

*One daemon. All your markdown, rendered, highlighted, agent-ready.*

</div>

You're iterating on a design doc with Claude / Codex / Cursor. You want to point at a specific paragraph and say "fix this" with a real `path:line` ref — not a screenshot, not "look at the third bullet." `gloss` renders a folder of markdown in a clean GitHub-styled browser UI, and one click on any selection puts `file:start-end` + the quoted text on your clipboard, ready to paste.

A self-spawning persistent daemon backs every invocation, so `gloss .` from any folder reuses one browser tab across many runs.

- 🎨 **GitHub-style render** — goldmark + chroma syntax highlighting, vendored `github-markdown.css`
- 📋 **Selection → clipboard with file:line ref** — built for the "ask my agent about this paragraph" loop
- ⭐ **Saved highlights** — one global NDJSON store, browseable in-app, exportable as markdown
- 🕘 **Cross-session recent** — every `.md` you've touched in any nvim session, one click away (via sibling [`recent-files`](../recent-files) CLI)
- 🔁 **Self-spawning daemon** — never run `daemon start`. First invocation spawns it, every later one reuses it, crashes self-heal on the next call
- 🔄 **Live reload** — fsnotify pushes file changes over SSE, the open page refreshes in place
- 👁️ **Read-only viewer** — no editing in the browser. Editing belongs in your editor
- 🔒 **localhost-only, token-gated** — random per-daemon hex token, never bound beyond `127.0.0.1`
- 🪶 **Vanilla frontend** — one HTML, one CSS, ~400 lines of plain JS embedded in the binary. No node, no React, no build step

---

## Install

Requires Go 1.24+ and macOS or Linux.

```sh
git clone <repo-url> gloss
cd gloss
make install    # builds and copies to ~/bin/ + codesigns on macOS
```

Make sure `~/bin` is on your `$PATH`.

## Quick Start

```sh
gloss .                       # serve current folder, open browser
gloss path/to/folder          # serve a specific folder
gloss path/to/file.md         # serve its parent folder and focus the file
```

The first invocation spawns a detached daemon and opens a browser tab. Every subsequent invocation reuses the same daemon and refocuses the same tab.

## Config

Location: `~/.config/gloss/config.yaml` (auto-created on first run)

```yaml
port: 0                     # 0 = random free port; pin a number for a stable URL
open_browser: true
copy_path_style: tilde      # tilde | absolute | relative

recent:
  days: 30
  max: 100

ignore:
  - node_modules
  - .git
  - .next
  - dist
  - vendor
  - target

highlights:
  path: ~/.local/share/gloss/highlights.json
```

## CLI

```sh
gloss [path]                   # default = serve current folder, open browser
gloss serve [path]             # explicit
gloss serve --foreground       # don't detach — run in current terminal (debug)
gloss serve --port 8765        # pin port for a stable URL
gloss serve --no-open          # don't open browser
gloss serve --quiet            # suppress status output
gloss serve --file <abs>       # focus this file in the opened folder

gloss recent                   # list recent .md files cross-session, fzf-friendly
gloss recent --json
gloss recent --days 7

gloss highlights list          # all saved highlights
gloss highlights show <id>
gloss highlights rm <id>
gloss highlights export        # all highlights as markdown to stdout

gloss daemon status            # pid + port + log path
gloss daemon stop              # graceful shutdown
gloss daemon log               # tail -f the daemon log

gloss config                   # open config in $EDITOR
gloss config --path            # print config file path

gloss --version
```

## Browser Keybindings

| Key          | Action                                       |
|--------------|----------------------------------------------|
| `j` / `k`    | Move file selection                          |
| `Enter`      | Open selected file                           |
| `g` / `G`    | Top / bottom of current file                 |
| `/`          | Focus filter input                           |
| `y`          | Copy current selection with `path:line` ref  |
| `Y`          | Copy current selection (plain text)          |
| `*`          | Save current selection as a highlight        |
| `H`          | Toggle highlights view                       |
| `?`          | Help dialog                                  |

## How Highlights Work

Select any rendered text. A floating bar pops up with three actions:

- **⎘ Copy w/ ref** — clipboard contains:

  ```
  ~/code/foo/ARCHITECTURE.md:42-58

  > selected line 1
  > selected line 2
  > …
  ```

  Paste into Claude / Codex / Cursor — the agent sees both the location and the content.
- **⎘ Copy** — plain text only.
- **★ Save** — appends to `~/.local/share/gloss/highlights.json`. Browseable from the `★` button in the top bar, survives daemon restarts, export with `gloss highlights export`.

The line range comes from `data-line-start` / `data-line-end` attributes that the server-side goldmark renderer stamps onto every block-level element. Selecting the middle of a code span rounds up to the enclosing block's range — exactly what you want for an agent paste.

## nvim Integration

Drop-in replacement for any existing `gh markdown-preview` keymap. Add to `~/.config/nvim/after/ftplugin/markdown.lua`:

```lua
vim.keymap.set("n", "<leader>mP", function()
  local file = vim.fn.expand("%:p")
  if file == "" then
    vim.notify("No file to preview", vim.log.levels.WARN)
    return
  end
  vim.cmd("silent write")
  vim.fn.jobstart(
    { "gloss", "serve", vim.fn.expand("%:p:h"), "--file", file, "--quiet" },
    { detach = true }
  )
end, { buffer = true, silent = true, desc = "Preview markdown (gloss)" })
```

First press spawns the daemon. Subsequent presses are instant — they just refocus the existing browser tab on the new file.

## How It Works

```
gloss .
   │
   ├─ flock(~/.local/state/gloss/server.lock)
   ├─ read server.json → live? POST /api/open : spawn `_serve --detached`
   └─ open http://127.0.0.1:<port>/?folder=<id>&file=<rel>&t=<token>

gloss _serve --detached  (one process, lives forever)
   ├─ HTTP @ 127.0.0.1:0
   ├─ goldmark renderer with line-stamp extension
   ├─ fsnotify watchers → SSE → live reload
   ├─ highlights NDJSON store
   └─ recent-files TSV reader
```

**Client** grabs an flock, reads `server.json`, and either POSTs to the live daemon or spawns a fresh `_serve --detached`. Either way it ends with an `open http://127.0.0.1:<port>/…` call carrying the folder id, optional file, and the per-daemon token.

**Daemon** is one long-lived `*http.Server` with an in-memory open-folders map, an SSE hub for live reload, fsnotify watchers (debounced 100ms, capped at 32 folders), and the highlights store. Every `/api/*` route requires `X-Gloss-Token: <hex>` — constant-time compared.

## State Files

| Path                                             | Owner             | Purpose                                |
|--------------------------------------------------|-------------------|----------------------------------------|
| `~/.config/gloss/config.yaml`                    | gloss             | User config                            |
| `~/.local/state/gloss/server.json`               | gloss daemon      | `{pid, port, started_at, token}`       |
| `~/.local/state/gloss/server.lock`               | gloss client      | flock for safe spawn                   |
| `~/.local/state/gloss/daemon.log`                | gloss daemon      | stdout + stderr                        |
| `~/.local/share/gloss/highlights.json`           | gloss             | NDJSON append store                    |
| `~/.local/share/recent-files/history.tsv`        | recent-files cli  | **read-only** by gloss                 |

## Troubleshooting

```sh
gloss daemon status          # is it running?
gloss daemon log             # tail the log
gloss daemon stop            # nuke it; the next `gloss .` will respawn

gloss serve --foreground .   # bypass self-daemon, run attached for debugging
```

If something looks broken, `--foreground` is the easiest way to see what's happening.

---

> Personal tool built for the "iterate on a markdown design doc with an agent" loop. Sharing it in case it's useful — not actively seeking feature requests, but feel free to fork.
