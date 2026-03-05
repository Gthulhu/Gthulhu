package rest

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestRegisterFrontend_WithIndex(t *testing.T) {
	e := echo.New()

	// Create a mock filesystem that simulates embedded dist contents.
	mockFS := fstest.MapFS{
		"index.html":           {Data: []byte("<html>app</html>")},
		"assets/main.js":       {Data: []byte("console.log('ok')")},
		"assets/style.css":     {Data: []byte("body{}")},
	}

	registerFrontendFromFS(e, mockFS)

	// Requesting an existing asset should return it directly.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/assets/main.js", nil)
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "console.log")

	// Requesting a non-existent path should fall back to index.html (SPA).
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/some/spa/route", nil)
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "<html>app</html>")
}

func TestRegisterFrontend_WithoutIndex(t *testing.T) {
	e := echo.New()

	// Filesystem without index.html should not register any routes.
	mockFS := fstest.MapFS{
		".gitkeep": {Data: []byte{}},
	}

	registerFrontendFromFS(e, mockFS)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/anything", nil)
	e.ServeHTTP(rec, req)

	// Should get 404/405 because no wildcard route was registered.
	assert.NotEqual(t, http.StatusOK, rec.Code)
}

// registerFrontendFromFS is a test helper that mirrors RegisterFrontend logic
// but accepts an arbitrary fs.FS instead of the embedded one.
func registerFrontendFromFS(e *echo.Echo, distFS fs.FS) {
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
		path := r.URL.Path
		if len(path) > 0 && path[0] == '/' {
			path = path[1:]
		}

		if path != "" {
			if f, err := distFS.Open(path); err == nil {
				f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})))
}
