package workspace

import (
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
)

func (w *Workspace) SaveImage(token, id, contentType string, content io.Reader) (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if token != w.token {
		return "", ErrChanged
	}
	if _, err := w.readProject(id); err != nil {
		return "", err
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || !strings.HasPrefix(mediaType, "image/") {
		return "", fmt.Errorf("%w：请选择图片文件", ErrInput)
	}
	extensions, err := mime.ExtensionsByType(mediaType)
	if err != nil || len(extensions) == 0 {
		return "", fmt.Errorf("%w：无法识别图片格式 %s", ErrInput, mediaType)
	}
	name := newID() + extensions[0]
	relative := filepath.Join(id, "sources", name)
	file, err := w.root.OpenFile(relative, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return "", err
	}
	_, copyErr := io.Copy(file, content)
	closeErr := file.Close()
	if copyErr != nil || closeErr != nil {
		w.root.Remove(relative)
		if copyErr != nil {
			return "", copyErr
		}
		return "", closeErr
	}
	return "sources/" + name, nil
}

func (w *Workspace) Image(token, id, name string) (*os.File, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if token != w.token {
		return nil, ErrChanged
	}
	if !validID(id) || name == "." || name == ".." || filepath.Base(name) != name || strings.ContainsAny(name, `/\`) {
		return nil, os.ErrNotExist
	}
	return w.root.Open(filepath.Join(id, "sources", name))
}
