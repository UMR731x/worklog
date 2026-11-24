package server

import (
	"encoding/json"
	"net/http"
	"testing"

	"worklog/internal/workspace"
)

func TestEntryManagementThroughHTTP(t *testing.T) {
	handler, token := testAPI(t)
	created := callAPI(handler, token, "POST", "/api/projects", `{"name":"进度管理","status":"Doing"}`)
	var project workspace.Project
	if err := json.Unmarshal(created.Body.Bytes(), &project); err != nil {
		t.Fatal(err)
	}
	path := "/api/projects/" + project.ID + "/entries/2026-11-24"
	added := callAPI(handler, token, "POST", "/api/projects/"+project.ID+"/entries", `{"date":"2026-11-24"}`)
	if added.Code != http.StatusCreated {
		t.Fatalf("add entry: %d %s", added.Code, added.Body)
	}
	saved := callAPI(handler, token, "PUT", path, `{"content":"保留正文"}`)
	var result struct {
		workspace.Summary
		Entry workspace.EntryMetadata `json:"entry"`
	}
	if err := json.Unmarshal(saved.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if saved.Code != http.StatusOK || result.ID != project.ID || result.EntryCount != 1 || result.Entry.CreatedAt.IsZero() || result.Entry.UpdatedAt.IsZero() {
		t.Fatalf("save must retain summary and supply entry times: %d %s", saved.Code, saved.Body)
	}
	renamed := callAPI(handler, token, "PUT", path+"/name", `{"name":"接口 / 联调 📝"}`)
	if renamed.Code != http.StatusOK {
		t.Fatalf("rename: %d %s", renamed.Code, renamed.Body)
	}
	if err := json.Unmarshal(renamed.Body.Bytes(), &project); err != nil {
		t.Fatal(err)
	}
	metadata := project.EntryMetadata["2026-11-24"]
	if project.Entries["2026-11-24"] != "保留正文" || metadata.Name != "接口 / 联调 📝" || !metadata.CreatedAt.Equal(result.Entry.CreatedAt) {
		t.Fatalf("rename changed record identity, content or creation time: %#v", project)
	}
	if response := callAPI(handler, token, "PUT", path+"/name", `{}`); response.Code != http.StatusBadRequest {
		t.Fatalf("missing name must not erase the title: %d %s", response.Code, response.Body)
	}
	if response := callAPI(handler, "previous-workspace", "DELETE", path, ""); response.Code != http.StatusConflict {
		t.Fatalf("stale directory token accepted: %d %s", response.Code, response.Body)
	}
	deleted := callAPI(handler, token, "DELETE", path, "")
	var remaining workspace.Project
	if err := json.Unmarshal(deleted.Body.Bytes(), &remaining); err != nil {
		t.Fatal(err)
	}
	if deleted.Code != http.StatusOK || remaining.ID != project.ID || len(remaining.Entries) != 0 {
		t.Fatalf("delete must keep the project and return its remaining entries: %d %s", deleted.Code, deleted.Body)
	}
	late := callAPI(handler, token, "PUT", path, `{"content":"迟到保存"}`)
	if late.Code != http.StatusNotFound {
		t.Fatalf("deleted entry was recreated: %d %s", late.Code, late.Body)
	}
}
