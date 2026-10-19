// Package web embeds the frontend.
package web

import "embed"

//go:embed static
// FS holds the embedded frontend files under static/.
var FS embed.FS
