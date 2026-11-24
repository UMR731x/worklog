package server

import "net/http"

func (api *API) createProject(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	if !decode(w, r, &request) {
		return
	}
	project, err := api.workspace.Create(r.Header.Get("X-Workspace"), request.Name, request.Status)
	respond(w, http.StatusCreated, project, err)
}

func (api *API) project(w http.ResponseWriter, r *http.Request) {
	project, err := api.workspace.Project(r.Header.Get("X-Workspace"), r.PathValue("id"))
	respond(w, http.StatusOK, project, err)
}

func (api *API) deleteProject(w http.ResponseWriter, r *http.Request) {
	err := api.workspace.Delete(r.Header.Get("X-Workspace"), r.PathValue("id"))
	respond(w, http.StatusNoContent, nil, err)
}

func (api *API) setStatus(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Status string `json:"status"`
	}
	if !decode(w, r, &request) {
		return
	}
	project, err := api.workspace.SetStatus(r.Header.Get("X-Workspace"), r.PathValue("id"), request.Status)
	respond(w, http.StatusOK, project, err)
}

func (api *API) addEntry(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Date string `json:"date"`
	}
	if !decode(w, r, &request) {
		return
	}
	project, key, err := api.workspace.AddEntry(r.Header.Get("X-Workspace"), r.PathValue("id"), request.Date)
	respond(w, http.StatusCreated, map[string]any{"project": project, "key": key}, err)
}

func (api *API) saveEntry(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Content *string `json:"content"`
	}
	if !decode(w, r, &request) {
		return
	}
	if request.Content == nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": "缺少 content 字段"}, nil)
		return
	}
	project, err := api.workspace.SaveEntry(
		r.Header.Get("X-Workspace"), r.PathValue("id"), r.PathValue("key"), *request.Content,
	)
	respond(w, http.StatusOK, project.Summary(), err)
}

func (api *API) uploadImage(w http.ResponseWriter, r *http.Request) {
	path, err := api.workspace.SaveImage(
		r.Header.Get("X-Workspace"), r.PathValue("id"), r.Header.Get("Content-Type"), r.Body,
	)
	respond(w, http.StatusCreated, map[string]string{"path": path}, err)
}

func (api *API) image(w http.ResponseWriter, r *http.Request) {
	file, err := api.workspace.Image(r.PathValue("token"), r.PathValue("id"), r.PathValue("name"))
	if err != nil {
		respond(w, http.StatusOK, nil, err)
		return
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		respond(w, http.StatusOK, nil, err)
		return
	}
	// Uploaded SVGs may be viewed directly as well as through an <img> element.
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'; style-src 'unsafe-inline'")
	http.ServeContent(w, r, stat.Name(), stat.ModTime(), file)
}
