package server

import (
	"io/fs"
	"net/http"
	"strings"
)

func (api *API) index(w http.ResponseWriter, r *http.Request) {
	content, err := fs.ReadFile(api.assets, "index.html")
	if err != nil {
		http.Error(w, "无法读取页面", http.StatusInternalServerError)
		return
	}
	page := strings.Replace(string(content), `data-theme="system"`, `data-theme="`+api.workspace.Theme()+`"`, 1)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(page))
}

func (api *API) setTheme(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Theme string `json:"theme"`
	}
	if !decode(w, r, &request) {
		return
	}
	respond(w, http.StatusNoContent, nil, api.workspace.SetTheme(request.Theme))
}

func (api *API) checkUpdate(w http.ResponseWriter, r *http.Request) {
	status := api.updates.Check(r.Context(), r.Method == http.MethodPost)
	respond(w, http.StatusOK, status, nil)
}

func (api *API) installUpdate(w http.ResponseWriter, r *http.Request) {
	if err := api.updates.Install(r.Context()); err != nil {
		respond(w, http.StatusInternalServerError, nil, err)
		return
	}
	respond(w, http.StatusNoContent, nil, nil)
	api.quit()
}
