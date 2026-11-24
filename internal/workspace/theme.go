package workspace

import "fmt"

func validTheme(theme string) bool {
	return theme == "system" || theme == "light" || theme == "dark"
}

func (w *Workspace) Theme() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.theme
}

func (w *Workspace) SetTheme(theme string) error {
	if !validTheme(theme) {
		return fmt.Errorf("%w：请选择日间、夜间或跟随系统", ErrInput)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := writeJSON(w.base, "settings.json", Settings{Directory: w.directory, Theme: theme}); err != nil {
		return err
	}
	w.theme = theme
	return nil
}
