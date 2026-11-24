package workspace

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Document struct {
	Name          string                   `json:"name"`
	Status        string                   `json:"status"`
	Entries       map[string]string        `json:"entries"`
	EntryMetadata map[string]EntryMetadata `json:"entryMetadata,omitempty"`
}

type EntryMetadata struct {
	Name      string    `json:"name,omitempty"`
	CreatedAt time.Time `json:"createdAt,omitzero"`
	UpdatedAt time.Time `json:"updatedAt,omitzero"`
}

type Project struct {
	ID string `json:"id"`
	Document
	UpdatedAt time.Time `json:"updatedAt"`
}

type Summary struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	EntryCount int       `json:"entryCount"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func (p Project) Summary() Summary {
	return Summary{
		ID: p.ID, Name: p.Name, Status: p.Status,
		EntryCount: len(p.Entries), UpdatedAt: p.UpdatedAt,
	}
}

func (w *Workspace) Project(token, id string) (Project, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if token != w.token {
		return Project{}, ErrChanged
	}
	return w.readProject(id)
}

func (w *Workspace) Create(token, name, status string) (Project, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if token != w.token {
		return Project{}, ErrChanged
	}
	name = strings.TrimSpace(name)
	if name == "" || !validStatus(status) {
		return Project{}, fmt.Errorf("%w：项目名称不能为空，状态须为 Doing、Done 或 Pending", ErrInput)
	}
	id := newID()
	if err := w.root.Mkdir(id, 0755); err != nil {
		return Project{}, err
	}
	if err := w.root.Mkdir(filepath.Join(id, "sources"), 0755); err != nil {
		w.root.Remove(id)
		return Project{}, err
	}
	document := Document{Name: name, Status: status, Entries: map[string]string{}}
	if err := writeJSON(w.root, filepath.Join(id, "project.json"), document); err != nil {
		w.root.RemoveAll(id)
		return Project{}, err
	}
	return Project{ID: id, Document: document, UpdatedAt: time.Now()}, nil
}

func (w *Workspace) Delete(token, id string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if token != w.token {
		return ErrChanged
	}
	// A project must be readable before its directory can be deleted.
	if _, err := w.readProject(id); err != nil {
		return err
	}
	return w.root.RemoveAll(id)
}

func (w *Workspace) SetStatus(token, id, status string) (Project, error) {
	return w.changeProject(token, id, func(project *Project) error {
		if !validStatus(status) {
			return fmt.Errorf("%w：状态须为 Doing、Done 或 Pending", ErrInput)
		}
		project.Status = status
		return nil
	})
}

func (w *Workspace) AddEntry(token, id, date string) (Project, string, error) {
	key := date
	project, err := w.changeProject(token, id, func(project *Project) error {
		if _, err := time.Parse(time.DateOnly, date); err != nil {
			return fmt.Errorf("%w：日期应为 YYYY-MM-DD", ErrInput)
		}
		for number := 2; ; number++ {
			if _, exists := project.Entries[key]; !exists {
				break
			}
			key = fmt.Sprintf("%s (%d)", date, number)
		}
		project.Entries[key] = ""
		now := time.Now()
		project.EntryMetadata[key] = EntryMetadata{CreatedAt: now, UpdatedAt: now}
		return nil
	})
	return project, key, err
}

func (w *Workspace) SaveEntry(token, id, key, content string) (Project, error) {
	return w.changeProject(token, id, func(project *Project) error {
		previous, exists := project.Entries[key]
		if !exists {
			return os.ErrNotExist
		}
		if previous == content {
			return nil
		}
		project.Entries[key] = content
		metadata := project.EntryMetadata[key]
		metadata.UpdatedAt = time.Now()
		project.EntryMetadata[key] = metadata
		return nil
	})
}

func (w *Workspace) RenameEntry(token, id, key, name string) (Project, error) {
	return w.changeProject(token, id, func(project *Project) error {
		if _, exists := project.Entries[key]; !exists {
			return os.ErrNotExist
		}
		name = strings.TrimSpace(name)
		if name == "" {
			return fmt.Errorf("%w：进度记录名称不能为空", ErrInput)
		}
		metadata := project.EntryMetadata[key]
		if name == metadata.Name || (metadata.Name == "" && name == key) {
			return nil
		}
		metadata.Name = name
		metadata.UpdatedAt = time.Now()
		project.EntryMetadata[key] = metadata
		return nil
	})
}

func (w *Workspace) DeleteEntry(token, id, key string) (Project, error) {
	return w.changeProject(token, id, func(project *Project) error {
		if _, exists := project.Entries[key]; !exists {
			return os.ErrNotExist
		}
		delete(project.Entries, key)
		delete(project.EntryMetadata, key)
		return nil
	})
}

func (w *Workspace) changeProject(token, id string, change func(*Project) error) (Project, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if token != w.token {
		return Project{}, ErrChanged
	}
	project, err := w.readProject(id)
	if err != nil {
		return Project{}, err
	}
	if err := change(&project); err != nil {
		return Project{}, err
	}
	if err := writeJSON(w.root, filepath.Join(id, "project.json"), project.Document); err != nil {
		return Project{}, err
	}
	project.UpdatedAt = time.Now()
	return project, nil
}

func (w *Workspace) readProject(id string) (Project, error) {
	if !validID(id) {
		return Project{}, os.ErrNotExist
	}
	file, err := w.root.Open(filepath.Join(id, "project.json"))
	if err != nil {
		return Project{}, err
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil {
		return Project{}, err
	}
	var document Document
	if err := json.Unmarshal(content, &document); err != nil {
		return Project{}, fmt.Errorf("project.json 格式错误：%w", err)
	}
	if strings.TrimSpace(document.Name) == "" || !validStatus(document.Status) || document.Entries == nil {
		return Project{}, fmt.Errorf("project.json 缺少有效的 name、status 或 entries 字段")
	}
	if document.EntryMetadata == nil {
		document.EntryMetadata = make(map[string]EntryMetadata)
	}
	stat, err := file.Stat()
	if err != nil {
		return Project{}, err
	}
	return Project{ID: id, Document: document, UpdatedAt: stat.ModTime()}, nil
}

func validStatus(status string) bool {
	return status == "Doing" || status == "Done" || status == "Pending"
}

func newID() string {
	var bytes [16]byte
	_, _ = rand.Read(bytes[:])
	return hex.EncodeToString(bytes[:])
}

func validID(id string) bool {
	if len(id) != 32 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}

// Write beside the destination, then replace it; never truncate the saved document.
func writeJSON(root *os.Root, name string, value any) error {
	temporary := filepath.Join(filepath.Dir(name), ".worklog-"+newID())
	file, err := root.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	defer root.Remove(temporary)
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return root.Rename(temporary, name)
}
