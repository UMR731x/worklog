package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/url"
	"os"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"

	"worklog/internal/update"
	"worklog/internal/workspace"
)

type API struct {
	workspace *workspace.Workspace
	markdown  goldmark.Markdown
	quit      func()
	assets    fs.FS
	updates   *update.Updater
}

func New(work *workspace.Workspace, assets fs.FS, quit func(), updates *update.Updater) http.Handler {
	api := &API{workspace: work, markdown: goldmark.New(goldmark.WithExtensions(extension.GFM)), quit: quit, assets: assets, updates: updates}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/workspace", api.snapshot)
	mux.HandleFunc("POST /api/workspace", api.switchDirectory)
	mux.HandleFunc("GET /api/directories", api.directories)
	mux.HandleFunc("POST /api/projects", api.createProject)
	mux.HandleFunc("GET /api/projects/{id}", api.project)
	mux.HandleFunc("DELETE /api/projects/{id}", api.deleteProject)
	mux.HandleFunc("PUT /api/projects/{id}/status", api.setStatus)
	mux.HandleFunc("POST /api/projects/{id}/entries", api.addEntry)
	mux.HandleFunc("PUT /api/projects/{id}/entries/{key}", api.saveEntry)
	mux.HandleFunc("PUT /api/projects/{id}/entries/{key}/name", api.renameEntry)
	mux.HandleFunc("DELETE /api/projects/{id}/entries/{key}", api.deleteEntry)
	mux.HandleFunc("POST /api/projects/{id}/images", api.uploadImage)
	mux.HandleFunc("GET /images/{token}/{id}/{name}", api.image)
	mux.HandleFunc("POST /api/preview", api.preview)
	mux.HandleFunc("POST /api/quit", api.shutdown)
	mux.HandleFunc("PUT /api/theme", api.setTheme)
	mux.HandleFunc("GET /api/update", api.checkUpdate)
	mux.HandleFunc("POST /api/update/check", api.checkUpdate)
	mux.HandleFunc("POST /api/update/install", api.installUpdate)
	mux.HandleFunc("GET /{$}", api.index)
	mux.Handle("GET /", http.FileServerFS(assets))
	protected := http.NewCrossOriginProtection().Handler(mux)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Loopback binding alone does not prevent a foreign domain rebinding to it.
		host := (&url.URL{Host: r.Host}).Hostname()
		if host != "127.0.0.1" && host != "localhost" {
			http.Error(w, "仅允许通过本机地址访问", http.StatusForbidden)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data: https: http:; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		protected.ServeHTTP(w, r)
	})
}

func (api *API) snapshot(w http.ResponseWriter, r *http.Request) {
	snapshot, err := api.workspace.Snapshot()
	respond(w, http.StatusOK, snapshot, err)
}

func (api *API) switchDirectory(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Directory string `json:"directory"`
	}
	if !decode(w, r, &request) {
		return
	}
	snapshot, err := api.workspace.Switch(r.Header.Get("X-Workspace"), request.Directory)
	respond(w, http.StatusOK, snapshot, err)
}

func (api *API) directories(w http.ResponseWriter, r *http.Request) {
	listing, err := workspace.ListDirectories(r.URL.Query().Get("path"))
	respond(w, http.StatusOK, listing, err)
}

func (api *API) preview(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Content string `json:"content"`
	}
	if !decode(w, r, &request) {
		return
	}
	var html bytes.Buffer
	err := api.markdown.Convert([]byte(request.Content), &html)
	respond(w, http.StatusOK, map[string]string{"html": html.String()}, err)
}

func (api *API) shutdown(w http.ResponseWriter, r *http.Request) {
	respond(w, http.StatusNoContent, nil, nil)
	api.quit()
}

func decode(w http.ResponseWriter, r *http.Request, value any) bool {
	if err := json.NewDecoder(r.Body).Decode(value); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": "请求内容不是有效的 JSON"}, nil)
		return false
	}
	return true
}

func respond(w http.ResponseWriter, status int, value any, err error) {
	if err != nil {
		status = http.StatusInternalServerError
		switch {
		case errors.Is(err, workspace.ErrChanged):
			status = http.StatusConflict
		case errors.Is(err, workspace.ErrInput):
			status = http.StatusBadRequest
		case errors.Is(err, os.ErrNotExist):
			status = http.StatusNotFound
		case errors.Is(err, os.ErrPermission):
			status = http.StatusForbidden
		}
		value = map[string]string{"error": err.Error()}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if status != http.StatusNoContent {
		_ = json.NewEncoder(w).Encode(value)
	}
}
