package update

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The copied test binary exercises replacement while the old executable is running.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--worklog-update-probe", "--worklog-updated":
			if err := runUpdateProcess(); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			os.Exit(0)
		}
	}
	os.Exit(m.Run())
}

func runUpdateProcess() error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	if os.Args[1] == "--worklog-updated" {
		if err := FinishRestart(executable, os.Args[1:]); err != nil {
			return err
		}
		fmt.Println("replacement started and old executable removed")
		return nil
	}
	content, err := os.ReadFile(executable)
	if err != nil {
		return err
	}
	u := New(executable, "v0.1.0")
	u.record.Release = Release{Tag: "v0.2.0", Size: int64(len(content))}
	u.client.Transport = roundTrip(func(*http.Request) (*http.Response, error) { return response(content), nil })
	if err := u.Install(context.Background()); err != nil {
		return err
	}
	u.ReleaseRestart()
	return nil
}

func TestRunningExecutableUpdateRestartsAndCleansBackup(t *testing.T) {
	current, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(current)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	executable := filepath.Join(directory, assetName())
	if err := os.WriteFile(executable, content, 0755); err != nil {
		t.Fatal(err)
	}
	settings := filepath.Join(directory, "settings.json")
	if err := os.WriteFile(settings, []byte(`{"directory":"my-projects","theme":"light"}`), 0600); err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command(executable, "--worklog-update-probe").CombinedOutput()
	if err != nil || !strings.Contains(string(output), "replacement started and old executable removed") {
		t.Fatalf("update handoff failed: %s, %v", output, err)
	}
	backups, err := filepath.Glob(filepath.Join(directory, ".worklog-old-*"))
	if err != nil || len(backups) != 0 {
		t.Fatalf("old executable was not cleaned up: %v, %v", backups, err)
	}
	if _, err := os.Stat(executable); err != nil {
		t.Fatalf("updated executable is missing: %v", err)
	}
	preserved, err := os.ReadFile(settings)
	if err != nil || string(preserved) != `{"directory":"my-projects","theme":"light"}` {
		t.Fatal("update changed application settings")
	}
}
