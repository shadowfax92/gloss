package web

import "embed"

//go:embed index.html app.js style.css github-markdown.css
var Static embed.FS
