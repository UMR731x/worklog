package web

import "embed"

//go:embed index.html *.css *.js icon.svg
var Assets embed.FS

//go:embed icon.png
var Icon []byte
