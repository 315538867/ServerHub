// embed.go bundles the frontend dist into the binary.
// Create web/dist/.gitkeep to satisfy the embed at compile time.
// Production builds replace this with the real frontend dist.
package main

import "embed"

//go:embed web/dist
var staticFiles embed.FS
