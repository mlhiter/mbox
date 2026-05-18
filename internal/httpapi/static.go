package httpapi

import (
	"embed"
	"net/http"
	"path"
	"strings"
)

//go:embed static/*
var consoleAssets embed.FS

func (api *API) serveConsole(w http.ResponseWriter, r *http.Request) {
	name := "index.html"
	if strings.HasPrefix(r.URL.Path, "/console/") {
		name = strings.TrimPrefix(path.Clean(r.URL.Path), "/console/")
	}
	switch name {
	case "", ".", "index.html":
		serveConsoleFile(w, "index.html", "text/html; charset=utf-8")
	case "app.css":
		serveConsoleFile(w, "app.css", "text/css; charset=utf-8")
	case "app.js":
		serveConsoleFile(w, "app.js", "text/javascript; charset=utf-8")
	default:
		http.NotFound(w, r)
	}
}

func serveConsoleFile(w http.ResponseWriter, name string, contentType string) {
	data, err := consoleAssets.ReadFile("static/" + name)
	if err != nil {
		http.Error(w, "console asset not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
