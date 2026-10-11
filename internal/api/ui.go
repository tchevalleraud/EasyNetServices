package api

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// uiHandler serves the single-page application. Unknown paths fall back to
// index.html so that client-side routes can be reloaded.
func (s *Server) uiHandler() http.Handler {
	files := http.FileServerFS(s.ui)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}

		index, err := fs.ReadFile(s.ui, "index.html")
		if err != nil {
			http.Error(w, "web interface not built", http.StatusServiceUnavailable)
			return
		}

		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name != "" && name != "index.html" {
			if info, err := fs.Stat(s.ui, name); err == nil && !info.IsDir() {
				if strings.HasPrefix(name, "assets/") {
					// Vite fingerprints asset file names, they can be cached forever.
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(w, r)
				return
			}
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(index)
	})
}
