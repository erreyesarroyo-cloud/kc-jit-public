package ui

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:static
var staticFS embed.FS

func Handler() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/", "/app", "/index.html":
			// Serve bytes directly — http.FileServer redirects /index.html <-> /
			// which causes ERR_TOO_MANY_REDIRECTS (-310) in browsers.
			b, err := staticFS.ReadFile("static/index.html")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(b)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
