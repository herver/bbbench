package distassets

import "embed"

// FS contains embedded distribution assets (default.yml + templates).
//
//go:embed default.yml templates/**
var FS embed.FS
