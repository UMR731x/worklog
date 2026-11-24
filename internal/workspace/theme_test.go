package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestThemePersistsWithoutChangingWorkspace(t *testing.T) {
	base := t.TempDir()
	if err := os.WriteFile(filepath.Join(base, "settings.json"), []byte(`{"directory":"existing"}`), 0600); err != nil {
		t.Fatal(err)
	}
	work, err := Open(base)
	if err != nil {
		t.Fatal(err)
	}
	defer work.Close()
	if work.Theme() != "system" {
		t.Fatal("old settings must default to the system theme")
	}
	project, err := work.Create(work.token, "已有项目", "Doing")
	if err != nil {
		t.Fatal(err)
	}
	originalDirectory := work.directory
	if err := work.SetTheme("light"); err != nil {
		t.Fatal(err)
	}
	if _, err := work.Switch(work.token, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if work.Theme() != "light" {
		t.Fatal("switching directories must retain the theme")
	}
	if err := work.SetTheme("invalid"); !errors.Is(err, ErrInput) {
		t.Fatalf("invalid theme: %v", err)
	}
	if err := work.SetTheme("dark"); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(base)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if reopened.Theme() != "dark" || reopened.directory != work.directory {
		t.Fatal("theme or directory was lost on restart")
	}
	if _, err := work.Switch(work.token, originalDirectory); err != nil {
		t.Fatal(err)
	}
	if _, err := work.Project(work.token, project.ID); err != nil {
		t.Fatalf("theme changes affected existing project: %v", err)
	}
}
