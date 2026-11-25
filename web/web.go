package web

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
)

//go:embed index.html style.css app.js
var Assets embed.FS

// Handler returns an http.Handler that serves the MeshStore web dashboard.
// It checks if local ./web/index.html exists on disk (useful for live local development),
// and seamlessly falls back to embedded binary assets.
func Handler() http.Handler {
	if _, err := os.Stat("web/index.html"); err == nil {
		return http.FileServer(http.Dir("web"))
	}
	sub, err := fs.Sub(Assets, ".")
	if err != nil {
		return http.NotFoundHandler()
	}
	return http.FileServer(http.FS(sub))
}
