package rest

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/Gthulhu/api/web"
	"github.com/labstack/echo/v4"
)

// RegisterFrontend serves the embedded React SPA from web/dist.
// It serves static assets directly and falls back to index.html
// for client-side routing.
func RegisterFrontend(e *echo.Echo) {
	distFS, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		return
	}

	// Check whether the frontend was actually built (more than just .gitkeep).
	hasIndex := false
	if f, err := distFS.Open("index.html"); err == nil {
		f.Close()
		hasIndex = true
	}
	if !hasIndex {
		return
	}

	fileServer := http.FileServer(http.FS(distFS))

	e.GET("/*", echo.WrapHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")

		// Try to open the requested file. If it exists, serve it directly.
		if path != "" {
			if f, err := distFS.Open(path); err == nil {
				f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		// Fallback to index.html for SPA client-side routing.
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})))
}
