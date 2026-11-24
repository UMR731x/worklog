package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEntryRenameAndDeletePreserveOtherRecords(t *testing.T) {
	work, token := openTestWorkspace(t)
	project, err := work.Create(token, "记录管理", "Doing")
	if err != nil {
		t.Fatal(err)
	}
	project, first, err := work.AddEntry(token, project.ID, "2026-11-24")
	if err != nil {
		t.Fatal(err)
	}
	created := project.EntryMetadata[first]
	if created.CreatedAt.IsZero() || !created.CreatedAt.Equal(created.UpdatedAt) {
		t.Fatalf("new entry timestamps: %#v", created)
	}
	project, second, err := work.AddEntry(token, project.ID, "2026-11-24")
	if err != nil {
		t.Fatal(err)
	}
	image, err := work.SaveImage(token, project.ID, "image/svg+xml", strings.NewReader(`<svg xmlns="http://www.w3.org/2000/svg"/>`))
	if err != nil {
		t.Fatal(err)
	}
	content := "## 保留正文\n![图](" + image + ")"
	project, err = work.SaveEntry(token, project.ID, first, content)
	if err != nil || !project.EntryMetadata[first].CreatedAt.Equal(created.CreatedAt) {
		t.Fatalf("save changed creation time: %#v, %v", project, err)
	}
	name := "接口 / 联调 📝"
	project, err = work.RenameEntry(token, project.ID, first, name)
	if err != nil || project.Entries[first] != content || project.EntryMetadata[first].Name != name {
		t.Fatalf("rename lost content or identity: %#v, %v", project, err)
	}
	if !project.EntryMetadata[first].CreatedAt.Equal(created.CreatedAt) {
		t.Fatal("rename changed creation time")
	}
	if _, err := work.RenameEntry(token, project.ID, second, name); err != nil {
		t.Fatalf("distinct entries may share a display name: %v", err)
	}
	if _, err := work.RenameEntry(token, project.ID, first, " \t "); !errors.Is(err, ErrInput) {
		t.Fatalf("empty name accepted: %v", err)
	}
	project, err = work.DeleteEntry(token, project.ID, first)
	if err != nil || len(project.Entries) != 1 || project.EntryMetadata[second].Name != name {
		t.Fatalf("delete affected another entry: %#v, %v", project, err)
	}
	if _, exists := project.EntryMetadata[first]; exists {
		t.Fatal("deleted entry metadata remains")
	}
	if _, err := work.SaveEntry(token, project.ID, first, "late save"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("late save recreated deleted entry: %v", err)
	}
	if _, err := work.RenameEntry(token, project.ID, first, "late rename"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("rename recreated deleted entry: %v", err)
	}
	if _, err := os.Stat(filepath.Join(work.directory, project.ID, image)); err != nil {
		t.Fatalf("deleting a record removed a shared image: %v", err)
	}
	project, err = work.DeleteEntry(token, project.ID, second)
	if err != nil || len(project.Entries) != 0 || len(project.EntryMetadata) != 0 {
		t.Fatalf("last record deletion failed: %#v, %v", project, err)
	}
}

func TestLegacyEntryTimesAndNamesPersistWithoutInventingHistory(t *testing.T) {
	work, token := openTestWorkspace(t)
	id := newID()
	if err := work.root.Mkdir(id, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(work.directory, id, "project.json")
	original := `{"name":"旧项目","status":"Pending","entries":{"2026-11-23":"旧正文","2026-11-24":"另一条"}}`
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	project, err := work.Project(token, id)
	if err != nil || len(project.EntryMetadata) != 0 {
		t.Fatalf("legacy project not loaded: %#v, %v", project, err)
	}
	untouched, err := os.ReadFile(path)
	if err != nil || string(untouched) != original {
		t.Fatal("reading the legacy project rewrote it")
	}
	before := time.Now()
	project, err = work.RenameEntry(token, id, "2026-11-23", "昨日总结")
	if err != nil {
		t.Fatal(err)
	}
	renamed := project.EntryMetadata["2026-11-23"]
	if !renamed.CreatedAt.IsZero() || renamed.UpdatedAt.Before(before) {
		t.Fatalf("legacy rename invented creation time or missed modification: %#v", renamed)
	}
	project, err = work.SaveEntry(token, id, "2026-11-23", "旧正文")
	if err != nil || !project.EntryMetadata["2026-11-23"].UpdatedAt.Equal(renamed.UpdatedAt) {
		t.Fatal("saving identical text changed modification time")
	}
	before = time.Now()
	project, err = work.SaveEntry(token, id, "2026-11-23", "修改后的正文")
	if err != nil || project.EntryMetadata["2026-11-23"].UpdatedAt.Before(before) {
		t.Fatalf("content edit did not record its time: %v", err)
	}
	loaded, err := work.Project(token, id)
	if err != nil || loaded.EntryMetadata["2026-11-23"].Name != "昨日总结" || loaded.Entries["2026-11-23"] != "修改后的正文" {
		t.Fatalf("entry name or content not persisted: %#v, %v", loaded, err)
	}
	if !loaded.EntryMetadata["2026-11-23"].CreatedAt.IsZero() || loaded.EntryMetadata["2026-11-24"] != (EntryMetadata{}) {
		t.Fatal("unavailable historical times must remain unknown")
	}
	if !loaded.EntryMetadata["2026-11-23"].UpdatedAt.Equal(project.EntryMetadata["2026-11-23"].UpdatedAt) {
		t.Fatal("modification time not persisted")
	}
}
