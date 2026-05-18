package docs

import "embed"

//go:embed *.md assets/*
var FS embed.FS
