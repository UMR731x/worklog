package workspace

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

var (
	ErrChanged = errors.New("工作目录已切换，请先复制未保存的内容，再刷新页面")
	ErrInput   = errors.New("输入无效")
)

type Settings struct {
	Directory string `json:"directory"`
	Theme     string `json:"theme"`
}

type Snapshot struct {
	Directory string    `json:"directory"`
	Token     string    `json:"token"`
	Projects  []Summary `json:"projects"`
	Warnings  []string  `json:"warnings"`
}

// Workspace owns the active directory and serializes reads, writes and switches.
type Workspace struct {
	mu        sync.Mutex
	base      *os.Root
	root      *os.Root
	directory string
	token     string
	theme     string
}

func Open(baseDirectory string) (*Workspace, error) {
	base, err := os.OpenRoot(baseDirectory)
	if err != nil {
		return nil, err
	}
	settings := Settings{Directory: filepath.Join(baseDirectory, "data"), Theme: "system"}
	file, err := base.Open("settings.json")
	if err == nil {
		err = json.NewDecoder(file).Decode(&settings)
		file.Close()
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		base.Close()
		return nil, fmt.Errorf("读取 settings.json：%w", err)
	}
	if !filepath.IsAbs(settings.Directory) {
		settings.Directory = filepath.Join(baseDirectory, settings.Directory)
	}
	root, directory, err := openDirectory(settings.Directory)
	if err != nil {
		base.Close()
		return nil, err
	}
	if !validTheme(settings.Theme) {
		settings.Theme = "system"
	}
	return &Workspace{base: base, root: root, directory: directory, token: newID(), theme: settings.Theme}, nil
}

func (w *Workspace) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return errors.Join(w.root.Close(), w.base.Close())
}

func (w *Workspace) Snapshot() (Snapshot, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.snapshot()
}

func (w *Workspace) snapshot() (Snapshot, error) {
	result := Snapshot{
		Directory: w.directory, Token: w.token,
		Projects: []Summary{}, Warnings: []string{},
	}
	directory, err := w.root.Open(".")
	if err != nil {
		return result, err
	}
	defer directory.Close()
	entries, err := directory.ReadDir(-1)
	if err != nil {
		return result, err
	}
	for _, entry := range entries {
		if !entry.IsDir() || !validID(entry.Name()) {
			continue
		}
		project, err := w.readProject(entry.Name())
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("项目 %s 无法读取：%v", entry.Name(), err))
			continue
		}
		result.Projects = append(result.Projects, project.Summary())
	}
	sort.Slice(result.Projects, func(i, j int) bool {
		return result.Projects[i].UpdatedAt.After(result.Projects[j].UpdatedAt)
	})
	return result, nil
}

func (w *Workspace) Switch(token, directory string) (Snapshot, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if token != w.token {
		return Snapshot{}, ErrChanged
	}
	root, absolute, err := openDirectory(directory)
	if err != nil {
		return Snapshot{}, err
	}
	// List before committing the setting so an unreadable directory is not selected.
	next := &Workspace{root: root, directory: absolute, token: newID()}
	snapshot, err := next.snapshot()
	if err == nil {
		err = writeJSON(w.base, "settings.json", Settings{Directory: absolute, Theme: w.theme})
	}
	if err != nil {
		root.Close()
		return Snapshot{}, err
	}
	w.root.Close()
	w.root, w.directory, w.token = root, absolute, next.token
	return snapshot, nil
}

func openDirectory(directory string) (*os.Root, string, error) {
	if directory == "" {
		return nil, "", fmt.Errorf("%w：请填写工作目录", ErrInput)
	}
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return nil, "", err
	}
	if err := os.MkdirAll(absolute, 0755); err != nil {
		return nil, "", err
	}
	root, err := os.OpenRoot(absolute)
	return root, absolute, err
}
