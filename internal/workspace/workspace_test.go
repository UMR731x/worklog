package workspace

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func openTestWorkspace(t *testing.T) (*Workspace, string) {
	t.Helper()
	work, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { work.Close() })
	snapshot, err := work.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	return work, snapshot.Token
}

func TestDailyEntriesPersistWithoutOverwriting(t *testing.T) {
	work, token := openTestWorkspace(t)
	project, err := work.Create(token, "产品 A/B：迭代 📝", "Pending")
	if err != nil {
		t.Fatal(err)
	}
	_, first, err := work.AddEntry(token, project.ID, "2026-11-24")
	if err != nil {
		t.Fatal(err)
	}
	content := "## 今日进展\n- [x] 完成登录\n\n**中文**与 emoji 🎉\n"
	if _, err := work.SaveEntry(token, project.ID, first, content); err != nil {
		t.Fatal(err)
	}
	_, second, err := work.AddEntry(token, project.ID, "2026-11-24")
	if err != nil {
		t.Fatal(err)
	}
	if first != "2026-11-24" || second != "2026-11-24 (2)" {
		t.Fatalf("same-day keys: %q, %q", first, second)
	}
	if _, err := work.SetStatus(token, project.ID, "Doing"); err != nil {
		t.Fatal(err)
	}
	// A fresh reader must recover the saved text and status from disk.
	reopened, err := Open(filepath.Dir(work.directory))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	loaded, err := reopened.Project(reopened.token, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Name != project.Name || loaded.Status != "Doing" || loaded.Entries[first] != content || len(loaded.Entries) != 2 {
		t.Fatalf("saved project changed: %#v", loaded)
	}
	if _, err := work.SaveEntry(token, project.ID, second, ""); err != nil {
		t.Fatalf("an empty entry must remain valid: %v", err)
	}
}

func TestWorkspaceSwitchRejectsLateWritesAndPersistsChoice(t *testing.T) {
	work, oldToken := openTestWorkspace(t)
	project, err := work.Create(oldToken, "目录隔离", "Doing")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := work.AddEntry(oldToken, project.ID, "2026-11-24"); err != nil {
		t.Fatal(err)
	}
	original := work.directory
	next := filepath.Join(t.TempDir(), "另一个工作目录")
	projectDirectory := filepath.Join(next, project.ID)
	if err := os.MkdirAll(filepath.Join(projectDirectory, "sources"), 0755); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(original, project.ID, "project.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDirectory, "project.json"), content, 0644); err != nil {
		t.Fatal(err)
	}
	snapshot, err := work.Switch(oldToken, next)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := work.SaveEntry(oldToken, project.ID, "2026-11-24", "late write"); !errors.Is(err, ErrChanged) {
		t.Fatalf("late write was not rejected: %v", err)
	}
	if _, err := work.SaveEntry(snapshot.Token, project.ID, "2026-11-24", "新目录"); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(filepath.Dir(original))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if reopened.directory != next {
		t.Fatalf("directory choice was not persisted: %s", reopened.directory)
	}
	snapshot, err = work.Switch(snapshot.Token, original)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := work.Project(snapshot.Token, project.ID)
	if err != nil || loaded.Entries["2026-11-24"] != "" {
		t.Fatalf("original directory was modified: %#v, %v", loaded, err)
	}
}

func TestImagesStayWithProjectAndDeleteRemovesDirectory(t *testing.T) {
	work, token := openTestWorkspace(t)
	project, err := work.Create(token, "图片项目", "Pending")
	if err != nil {
		t.Fatal(err)
	}
	var content bytes.Buffer
	if err := png.Encode(&content, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	path, err := work.SaveImage(token, project.ID, "image/png", &content)
	if err != nil {
		t.Fatal(err)
	}
	file, err := work.Image(token, project.ID, filepath.Base(path))
	if err != nil {
		t.Fatal(err)
	}
	_, decodeErr := png.Decode(file)
	file.Close()
	if decodeErr != nil {
		t.Fatal(decodeErr)
	}
	for _, name := range []string{".", "..", "../../settings.json"} {
		if _, err := work.Image(token, project.ID, name); err == nil {
			t.Fatalf("invalid image path was accepted: %q", name)
		}
	}
	if err := work.Delete(token, project.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(work.directory, project.ID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("project directory still exists: %v", err)
	}
}

func TestMalformedProjectDoesNotHideHealthyProjects(t *testing.T) {
	work, token := openTestWorkspace(t)
	if _, err := work.Create(token, "正常项目", "Done"); err != nil {
		t.Fatal(err)
	}
	broken, err := work.Create(token, "损坏项目", "Pending")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(work.directory, broken.ID, "project.json")
	if err := os.WriteFile(path, []byte("{broken"), 0644); err != nil {
		t.Fatal(err)
	}
	snapshot, err := work.Snapshot()
	if err != nil || len(snapshot.Projects) != 1 || len(snapshot.Warnings) != 1 {
		t.Fatalf("healthy projects should remain accessible: %#v, %v", snapshot, err)
	}
	if _, err := work.SetStatus(token, broken.ID, "Doing"); err == nil {
		t.Fatal("a malformed project was overwritten")
	}
}
