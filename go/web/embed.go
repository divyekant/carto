package web

import "embed"

// DistFS contains the built React SPA files from web/dist/.
// Build with: cd web && npm run build
//
//go:embed all:dist
var DistFS embed.FS
