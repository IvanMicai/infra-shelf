package assets

import "embed"

//go:embed templates/*.html static/*
var Files embed.FS
