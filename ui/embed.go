// Package ui provides the embedded frontend assets.
package ui

import "embed"

//go:embed all:dist
var Assets embed.FS
