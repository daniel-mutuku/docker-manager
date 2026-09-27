// Package web embeds the HTML templates and static assets so the whole
// application ships as a single binary.
package web

import "embed"

//go:embed templates
var TemplatesFS embed.FS

//go:embed static
var StaticFS embed.FS
