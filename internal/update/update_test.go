package update

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func response(body []byte) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body)), Header: make(http.Header)}
}

func TestDailyCheckPersistsAndDoesNotDowngradeDevelopmentBuild(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "worklog")
	u := New(executable, "v0.2.0-dev")
	calls := 0
	tag := "v0.1.0"
	u.client.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		return response([]byte(fmt.Sprintf(`{"tag_name":%q,"assets":[{"name":%q,"size":100}]}`, tag, assetName()))), nil
	})
	status := u.Check(context.Background(), false)
	if status.Error != "" || status.Available || status.Latest != "v0.1.0" {
		t.Fatalf("older release should not replace development build: %#v", status)
	}
	reopened := New(executable, u.version)
	reopened.client = u.client
	reopened.Check(context.Background(), false)
	if calls != 1 {
		t.Fatal("same-day restart repeated the automatic check")
	}
	tag = "v0.2.0"
	status = reopened.Check(context.Background(), true)
	if !status.Available || status.Error != "" || calls != 2 {
		t.Fatalf("manual check must allow upgrade to the stable version: %#v", status)
	}
	reopened.record.CheckedAt = time.Now().AddDate(0, 0, -1)
	reopened.Check(context.Background(), false)
	if calls != 3 {
		t.Fatal("a new day must check again")
	}
}

func TestIncompleteReleaseDoesNotOfferUpdate(t *testing.T) {
	for _, body := range []string{
		`{"tag_name":"v0.3.0","assets":[]}`,
		fmt.Sprintf(`{"tag_name":"v0.3.0-beta.1","prerelease":true,"assets":[{"name":%q,"size":100}]}`, assetName()),
	} {
		u := New(filepath.Join(t.TempDir(), "worklog"), "v0.2.0")
		u.client.Transport = roundTrip(func(*http.Request) (*http.Response, error) { return response([]byte(body)), nil })
		status := u.Check(context.Background(), true)
		if status.Available || status.Error == "" {
			t.Fatalf("unsupported release offered for installation: %#v", status)
		}
	}
}

func TestDownloadRejectsBrokenFilesBeforeReplacingExecutable(t *testing.T) {
	current, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	valid, err := os.ReadFile(current)
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []struct {
		name string
		body []byte
		size int64
		ok   bool
	}{
		{"valid executable", valid, int64(len(valid)), true},
		{"interrupted download", valid[:len(valid)/2], int64(len(valid)), false},
		{"HTML instead of executable", []byte("<html>error</html>"), 18, false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			directory := t.TempDir()
			target := filepath.Join(directory, "worklog")
			if err := os.WriteFile(target, []byte("existing program"), 0755); err != nil {
				t.Fatal(err)
			}
			u := New(target, "v0.1.0")
			u.record.Release = Release{Tag: "v0.2.0", Size: scenario.size}
			u.client.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
				if !strings.HasPrefix(r.URL.String(), repository+"/releases/download/v0.2.0/") {
					t.Fatalf("unexpected download source: %s", r.URL)
				}
				return response(scenario.body), nil
			})
			staged, err := u.download(context.Background())
			if (err == nil) != scenario.ok {
				t.Fatalf("download result: %v", err)
			}
			if staged != "" {
				defer os.Remove(staged)
			}
			saved, err := os.ReadFile(target)
			if err != nil || string(saved) != "existing program" {
				t.Fatalf("download changed the current executable: %s, %v", saved, err)
			}
		})
	}
}

func TestFailedReplacementRestoresOldExecutable(t *testing.T) {
	directory := t.TempDir()
	target, backup := filepath.Join(directory, "worklog"), filepath.Join(directory, ".old")
	if err := os.WriteFile(target, []byte("working version"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := replaceExecutable(target, filepath.Join(directory, "missing"), backup); err == nil {
		t.Fatal("missing staged file should fail replacement")
	}
	content, err := os.ReadFile(target)
	if err != nil || string(content) != "working version" {
		t.Fatalf("old executable was not restored: %s, %v", content, err)
	}
}
