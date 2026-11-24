package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/mod/semver"
)

// Version is set from the release tag by the build scripts.
var Version = "v0.3.0-dev"

const repository = "https://github.com/UMR731x/worklog"

type Release struct {
	Tag  string `json:"tag"`
	Size int64  `json:"size"`
}

type Status struct {
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	Available bool   `json:"available"`
	CheckedAt string `json:"checkedAt"`
	URL       string `json:"url"`
	Error     string `json:"error"`
}

type checkRecord struct {
	CheckedAt time.Time `json:"checkedAt"`
	Release   Release   `json:"release"`
	Error     string    `json:"error,omitempty"`
}

// Updater owns release checks and replacement of this executable only.
type Updater struct {
	mu          sync.Mutex
	executable  string
	version     string
	cachePath   string
	client      *http.Client
	endpoint    string
	record      checkRecord
	installed   bool
	restartPipe *os.File
}

func New(executable, version string) *Updater {
	u := &Updater{
		executable: executable, version: version,
		cachePath: filepath.Join(filepath.Dir(executable), ".worklog-update.json"),
		client:    &http.Client{},
		endpoint:  "https://api.github.com/repos/UMR731x/worklog/releases/latest",
	}
	// The check cache is disposable; a missing or damaged cache requests a fresh check.
	if content, err := os.ReadFile(u.cachePath); err == nil {
		var record checkRecord
		if json.Unmarshal(content, &record) == nil {
			u.record = record
		}
	}
	return u
}

func (u *Updater) Check(ctx context.Context, force bool) Status {
	u.mu.Lock()
	defer u.mu.Unlock()
	now := time.Now()
	if !force && u.record.CheckedAt.In(time.Local).Format(time.DateOnly) == now.Format(time.DateOnly) {
		return u.status()
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	release, err := u.fetchRelease(ctx)
	u.record.CheckedAt = now
	u.record.Error = ""
	if err != nil {
		u.record.Error = err.Error()
	} else {
		u.record.Release = release
	}
	if err := u.saveRecord(); err != nil {
		u.record.Error += " 保存检查时间失败：" + err.Error()
	}
	return u.status()
}

func (u *Updater) status() Status {
	latest := u.record.Release.Tag
	status := Status{
		Current: u.version, Latest: latest, Error: strings.TrimSpace(u.record.Error),
		Available: newer(latest, u.version),
	}
	if !u.record.CheckedAt.IsZero() {
		status.CheckedAt = u.record.CheckedAt.Format(time.RFC3339)
	}
	if semver.IsValid(versionTag(latest)) {
		status.URL = repository + "/releases/tag/" + latest
	}
	return status
}

func (u *Updater) fetchRelease(ctx context.Context) (Release, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, u.endpoint, nil)
	if err != nil {
		return Release{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "Worklog/"+u.version)
	response, err := u.client.Do(request)
	if err != nil {
		return Release{}, fmt.Errorf("无法连接 GitHub：%w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("检查更新失败：GitHub 返回 HTTP %d", response.StatusCode)
	}
	var release struct {
		Tag        string `json:"tag_name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
		Assets     []struct {
			Name string `json:"name"`
			Size int64  `json:"size"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(response.Body).Decode(&release); err != nil {
		return Release{}, fmt.Errorf("无法读取 GitHub 版本信息：%w", err)
	}
	tag := versionTag(release.Tag)
	if release.Draft || release.Prerelease || !semver.IsValid(tag) || semver.Prerelease(tag) != "" {
		return Release{}, fmt.Errorf("GitHub 未提供有效的正式版本")
	}
	for _, asset := range release.Assets {
		if asset.Name == assetName() && asset.Size > 0 {
			return Release{Tag: release.Tag, Size: asset.Size}, nil
		}
	}
	return Release{}, fmt.Errorf("版本 %s 尚未提供 %s", release.Tag, assetName())
}

func (u *Updater) saveRecord() error {
	content, err := json.Marshal(u.record)
	if err != nil {
		return err
	}
	return os.WriteFile(u.cachePath, content, 0600)
}

func versionTag(version string) string {
	return "v" + strings.TrimPrefix(version, "v")
}

func newer(latest, current string) bool {
	latest, current = versionTag(latest), versionTag(current)
	return semver.IsValid(latest) && semver.IsValid(current) && semver.Compare(latest, current) > 0
}

func assetName() string {
	if runtime.GOOS == "windows" {
		return "windows-x86_64.exe"
	}
	return "linux-x86_64.bin"
}
