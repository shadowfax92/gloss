<div align="center">

# gloss

**Browser markdown viewer with one-shot highlight-to-clipboard for AI agent feedback loops.**

</div>

`gloss` opens a folder of markdown files in a clean, GitHub-styled browser UI with a nested file tree on the left. Highlight any rendered text and one click puts it on your clipboard with `path:line_start-line_end` plus the selection as a markdown blockquote — ready to paste into Claude / Codex / Cursor for feedback.

A self-spawning persistent daemon backs every invocation: `gloss .` from any folder reuses the same browser tab. The sidebar surfaces every `.md` file you've recently opened in any nvim session via the sibling [`recent-files`](../recent-files) CLI.

- **GitHub-style render** — goldmark + chroma syntax highlighting
- **Selection → clipboard with file:line ref** — built for the "ask my agent for feedback on this paragraph" loop
- **Saved highlights** — one global store at `~/.local/share/gloss/highlights.json`, browseable in-app, exportable as markdown
- **Cross-session recent** — see every `.md` you've touched in any nvim session, jump to it in one click
- **Self-spawning daemon** — never run `gloss daemon start`. The first invocation spawns it, every later invocation reuses it, crashes self-heal on the next call
- **Live reload** — fsnotify pushes file changes over SSE; the open page refreshes in place
- **Read-only viewer** — no editing in the browser. Editing belongs in your editor
- **localhost-only, token-gated** — random per-daemon token; never bound to anything but `127.0.0.1`
- **Vanilla frontend** — one HTML, one CSS, ~400 lines of plain JS embedded in the binary. No node, no React, no build step

---

## Install

Requires Go 1.24+ and macOS or Linux.

```sh
git clone <repo-url> gloss
cd gloss
make install
```

Installs to `~/bin/gloss` and codesigns it on macOS. Make sure `~/bin` is on your `$PATH`.

## Quick start

```sh
gloss .                       # serve current folder, open browser
gloss path/to/folder          # serve a specific folder
gloss path/to/file.md         # serve its parent folder and focus the file
```

The first invocation spawns a detached daemon and opens a browser tab. Every subsequent invocation reuses the same daemon and refocuses the same tab.

## Commands

```sh
gloss [path]                  # default = serve current folder, open browser
gloss serve [path]            # explicit
gloss serve --foreground      # don't detach — run in current terminal (debug)
gloss serve --port 8765       # pin port for stable URL
gloss serve --no-open         # don't open browser
gloss serve --quiet           # suppress status output
gloss serve --file <abs>      # focus this file in the opened folder

gloss recent                  # list recent .md files cross-session, fzf-friendly
gloss recent --json
gloss recent --days 7

gloss highlights list         # all saved highlights
gloss highlights show <id>
gloss highlights rm <id>
gloss highlights export       # all highlights as markdown to stdout

gloss daemon status           # pid + port + log path
gloss daemon stop             # graceful shutdown
gloss daemon log              # tail -f the daemon log

gloss config                  # open config in $EDITOR
gloss config --path           # print config path

gloss --version
```

## Keyboard shortcuts (in the browser)

| Key                | Action                                          |
|--------------------|-------------------------------------------------|
| `j` / `k`          | Move file selection                             |
| `Enter`            | Open selected file                              |
| `g` / `G`          | Top / bottom of current file                    |
| `/`                | Focus filter input                              |
| `y`                | Copy current selection with `path:line` ref     |
| `Y`                | Copy current selection (plain text)             |
| `*`                | Save current selection as a highlight           |
| `H`                | Toggle highlights view                          |
| `?`                | Help dialog                                     |

## How highlights work

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
- **★ Save** — appends to `~/.local/share/gloss/highlights.json`. Browseable from the `★` button in the top bar; survives daemon restarts; export with `gloss highlights export`.

The line range comes from `data-line-start` / `data-line-end` attributes that the server-side goldmark renderer stamps onto every block-level element. Selecting the middle of a code span rounds up to the enclosing block's range — exactly what you want for an agent paste.

## nvim integration (drop-in for `<leader>mP`)

Replace any existing `gh markdown-preview` keymap in your `~/.config/nvim/after/ftplugin/markdown.lua`:

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

The first press spawns the daemon. Subsequent presses are instant — they just refocus the existing browser tab on the new file.

## Config

Location: `~/.config/gloss/config.yaml` (auto-created on first run).

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

## Files

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
gloss daemon status     # is it running?
gloss daemon log        # tail the log
gloss daemon stop       # nuke it; the next gloss . will respawn

gloss serve --foreground .   # bypass self-daemon, run attached for debugging
```

If something looks broken, the foreground escape hatch is the easiest way to see what's happening.

---

> Personal tool. Built for the "iterate on a markdown design doc with an agent" loop. Not actively seeking feature requests, but feel free to fork.
