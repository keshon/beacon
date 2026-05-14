package web

import (
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// serveStatic serves files under staticDir for paths /static/….
// Sets Content-Type explicitly so CSS/JS work on minimal images (e.g. Alpine)
// where mime sniffing can yield text/plain, which browsers reject for stylesheets.
func (s *Server) serveStatic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sub := strings.TrimPrefix(r.URL.Path, "/static")
	sub = strings.TrimPrefix(sub, "/")
	if sub == "" {
		http.NotFound(w, r)
		return
	}

	cleanURL := path.Clean("/" + sub)
	if cleanURL == "/" || cleanURL == "." {
		http.NotFound(w, r)
		return
	}
	name := strings.TrimPrefix(cleanURL, "/")

	root, err := filepath.Abs(s.staticDir)
	if err != nil {
		http.Error(w, "static root", http.StatusInternalServerError)
		return
	}
	full := filepath.Join(root, filepath.FromSlash(name))
	full, err = filepath.Abs(full)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if full != root && !strings.HasPrefix(full, root+string(filepath.Separator)) {
		http.NotFound(w, r)
		return
	}

	fi, err := os.Stat(full)
	if err != nil || fi.IsDir() {
		http.NotFound(w, r)
		return
	}

	f, err := os.Open(full)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()

	setStaticContentType(w, filepath.Ext(full))
	http.ServeContent(w, r, name, fi.ModTime(), f)
}

func setStaticContentType(w http.ResponseWriter, ext string) {
	switch strings.ToLower(ext) {
	case ".css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case ".js", ".mjs":
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	case ".json", ".map":
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	case ".ico":
		w.Header().Set("Content-Type", "image/x-icon")
	case ".svg":
		w.Header().Set("Content-Type", "image/svg+xml")
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".jpg", ".jpeg":
		w.Header().Set("Content-Type", "image/jpeg")
	case ".webp":
		w.Header().Set("Content-Type", "image/webp")
	case ".woff":
		w.Header().Set("Content-Type", "font/woff")
	case ".woff2":
		w.Header().Set("Content-Type", "font/woff2")
	case ".ttf":
		w.Header().Set("Content-Type", "font/ttf")
	default:
		if t := mime.TypeByExtension(ext); t != "" {
			w.Header().Set("Content-Type", t)
		}
	}
}
