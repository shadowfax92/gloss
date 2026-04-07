<div align="center">

# gloss

**Browser markdown viewer with one-shot highlight-to-clipboard for AI agent feedback loops.**

</div>

`gloss` opens a folder of markdown files in a clean, GitHub-styled browser UI with a nested file tree. Highlight any rendered text and one click puts it on your clipboard with `path:line_start-line_end` plus the selection as a markdown blockquote — ready to paste into Claude / Codex / Cursor for feedback.

A single self-spawning daemon backs every invocation, so `gloss .` from any folder reuses the same browser tab. The sidebar surfaces every `.md` file you've recently opened in any nvim session via `recent-files`.

> Personal tool. Built for the "iterate on a markdown design doc with an agent" loop.

## Install

```sh
git clone <repo-url> gloss
cd gloss
make install
```

Installs to `~/bin/gloss` and codesigns it.

## Quick start

```sh
gloss .                       # serve current folder, open browser
gloss path/to/folder          # serve a specific folder
gloss path/to/file.md         # serve its parent and focus the file
```

The first invocation spawns a detached daemon. Every subsequent invocation reuses it.

See [`docs/`](./docs) and the [PRD](../.llm/0407-gloss/prd.md) for the full design.
