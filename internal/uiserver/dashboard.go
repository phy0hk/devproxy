package server

import (
	"bytes"
	"embed"
	"io/fs"
	"net/http"
	"time"
)

//go:embed static/*
var staticFiles embed.FS

func handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	serveStaticFile(w, r, "static/index.html")
}

func handleStaticAssets() http.Handler {
	assets, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}

	return http.StripPrefix("/static/", http.FileServer(http.FS(assets)))
}

func serveStaticFile(w http.ResponseWriter, r *http.Request, name string) {
	data, err := staticFiles.ReadFile(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
}
