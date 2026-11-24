package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"worklog/internal/update"
	"worklog/internal/workspace"
	"worklog/web"
)

func testAPI(t *testing.T) (http.Handler, string) {
	t.Helper()
	work, err := workspace.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { work.Close() })
	snapshot, err := work.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	return New(work, web.Assets, func() {}, update.New(filepath.Join(t.TempDir(), "worklog"), update.Version)), snapshot.Token
}

func callAPI(handler http.Handler, token, method, path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, "http://127.0.0.1:8080"+path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Workspace", token)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestProjectEditingThroughHTTP(t *testing.T) {
	handler, token := testAPI(t)
	created := callAPI(handler, token, "POST", "/api/projects", `{"name":"版本发布 / v2","status":"Pending"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", created.Code, created.Body)
	}
	var project workspace.Project
	if err := json.Unmarshal(created.Body.Bytes(), &project); err != nil {
		t.Fatal(err)
	}
	path := "/api/projects/" + project.ID
	added := callAPI(handler, token, "POST", path+"/entries", `{"date":"2026-11-24"}`)
	if added.Code != http.StatusCreated {
		t.Fatalf("add: %d %s", added.Code, added.Body)
	}
	saved := callAPI(handler, token, "PUT", path+"/entries/2026-11-24", `{"content":"## 完成\n- [x] 确认发布"}`)
	if saved.Code != http.StatusOK {
		t.Fatalf("save: %d %s", saved.Code, saved.Body)
	}
	missing := callAPI(handler, token, "PUT", path+"/entries/2026-11-24", `{}`)
	if missing.Code != http.StatusBadRequest {
		t.Fatal("missing content must not erase a saved entry")
	}
	changed := callAPI(handler, token, "PUT", path+"/status", `{"status":"Done"}`)
	if changed.Code != http.StatusOK {
		t.Fatalf("status: %d %s", changed.Code, changed.Body)
	}
	loaded := callAPI(handler, token, "GET", path, "")
	if err := json.Unmarshal(loaded.Body.Bytes(), &project); err != nil {
		t.Fatal(err)
	}
	if project.Status != "Done" || project.Entries["2026-11-24"] != "## 完成\n- [x] 确认发布" {
		t.Fatalf("unexpected saved project: %#v", project)
	}
	deleted := callAPI(handler, token, "DELETE", path, "")
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", deleted.Code, deleted.Body)
	}
	if response := callAPI(handler, token, "GET", path, ""); response.Code != http.StatusNotFound {
		t.Fatal("deleted project is still readable")
	}
}

func TestMarkdownPreviewRendersDocumentsWithoutExecutableHTML(t *testing.T) {
	handler, token := testAPI(t)
	markdown := "## 今日进展\n\n- [x] 完成\n\n![图](sources/a.png)\n\n<script>alert(1)</script>\n\n[x](javascript:alert(1))\n"
	body, err := json.Marshal(map[string]string{"content": markdown})
	if err != nil {
		t.Fatal(err)
	}
	response := callAPI(handler, token, "POST", "/api/preview", string(body))
	var result struct {
		HTML string `json:"html"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"<h2>今日进展</h2>", `type="checkbox"`, `src="sources/a.png"`} {
		if !strings.Contains(result.HTML, expected) {
			t.Fatalf("missing %q in %s", expected, result.HTML)
		}
	}
	if strings.Contains(result.HTML, "<script") || strings.Contains(result.HTML, `href="javascript:`) {
		t.Fatalf("executable markup: %s", result.HTML)
	}
}

func TestForeignWebsitesCannotMutateLocalFiles(t *testing.T) {
	handler, token := testAPI(t)
	for _, scenario := range []struct {
		name, host, origin string
		status             int
	}{
		{"same origin", "127.0.0.1:8080", "http://127.0.0.1:8080", http.StatusCreated},
		{"localhost", "localhost:8080", "http://localhost:8080", http.StatusCreated},
		{"cross origin", "127.0.0.1:8080", "https://example.com", http.StatusForbidden},
		{"rebound host", "example.com:8080", "http://example.com:8080", http.StatusForbidden},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			request := httptest.NewRequest("POST", "http://"+scenario.host+"/api/projects", strings.NewReader(`{"name":"项目","status":"Doing"}`))
			request.Header.Set("Origin", scenario.origin)
			request.Header.Set("X-Workspace", token)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != scenario.status {
				t.Fatalf("got %d, want %d: %s", response.Code, scenario.status, response.Body)
			}
		})
	}
}
