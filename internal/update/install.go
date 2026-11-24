package update

import (
	"context"
	"debug/elf"
	"debug/pe"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
)

func (u *Updater) Install(ctx context.Context) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.installed {
		return fmt.Errorf("更新已完成，正在重启")
	}
	if !u.status().Available {
		return fmt.Errorf("没有可安装的新版本，请先检查更新")
	}
	staged, err := u.download(ctx)
	if err != nil {
		return err
	}
	defer os.Remove(staged)
	backup := backupPath(u.executable, os.Getpid())
	if err := replaceExecutable(u.executable, staged, backup); err != nil {
		return err
	}
	pipe, err := startReplacement(u.executable)
	if err != nil {
		// Restore the old executable if the OS could not start its replacement.
		moveErr := os.Rename(u.executable, staged)
		if moveErr != nil {
			return errors.Join(err, fmt.Errorf("回退失败，旧程序保留在 %s：%w", backup, moveErr))
		}
		return errors.Join(err, os.Rename(backup, u.executable))
	}
	u.restartPipe = pipe
	u.installed = true
	return nil
}

func (u *Updater) download(ctx context.Context) (string, error) {
	current, err := os.Stat(u.executable)
	if err != nil {
		return "", err
	}
	address := repository + "/releases/download/" + url.PathEscape(u.record.Release.Tag) + "/" + assetName()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return "", err
	}
	response, err := u.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("下载失败：%w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载失败：GitHub 返回 HTTP %d", response.StatusCode)
	}
	file, err := os.CreateTemp(filepath.Dir(u.executable), ".worklog-download-*")
	if err != nil {
		return "", err
	}
	complete := false
	defer func() {
		file.Close()
		if !complete {
			os.Remove(file.Name())
		}
	}()
	written, err := io.Copy(file, response.Body)
	if err != nil {
		return "", err
	}
	if written != u.record.Release.Size {
		return "", fmt.Errorf("下载文件不完整，请重新下载")
	}
	if err := file.Chmod(current.Mode().Perm()); err != nil {
		return "", err
	}
	if err := file.Sync(); err != nil {
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	if err := verifyExecutable(file.Name()); err != nil {
		return "", err
	}
	complete = true
	return file.Name(), nil
}

func verifyExecutable(path string) error {
	if runtime.GOOS == "windows" {
		file, err := pe.Open(path)
		if err != nil {
			return fmt.Errorf("下载文件不是有效的 Windows 程序：%w", err)
		}
		defer file.Close()
		if file.Machine != pe.IMAGE_FILE_MACHINE_AMD64 {
			return fmt.Errorf("下载程序不是 Windows x86_64 版本")
		}
		return nil
	}
	file, err := elf.Open(path)
	if err != nil {
		return fmt.Errorf("下载文件不是有效的 Linux 程序：%w", err)
	}
	defer file.Close()
	if file.Machine != elf.EM_X86_64 || file.Class != elf.ELFCLASS64 {
		return fmt.Errorf("下载程序不是 Linux x86_64 版本")
	}
	return nil
}

func replaceExecutable(target, staged, backup string) error {
	if err := os.Rename(target, backup); err != nil {
		return fmt.Errorf("无法替换程序：%w", err)
	}
	if err := os.Rename(staged, target); err != nil {
		rollbackErr := os.Rename(backup, target)
		return errors.Join(fmt.Errorf("安装失败：%w", err), rollbackErr)
	}
	return nil
}

func startReplacement(executable string) (*os.File, error) {
	reader, writer, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	command := exec.Command(executable, "--worklog-updated", strconv.Itoa(os.Getpid()))
	command.Stdin = reader
	// Windows GUI launches may have no standard output or error handles.
	if _, err := os.Stdout.Stat(); err == nil {
		command.Stdout = os.Stdout
	}
	if _, err := os.Stderr.Stat(); err == nil {
		command.Stderr = os.Stderr
	}
	command.Dir = filepath.Dir(executable)
	if err := command.Start(); err != nil {
		writer.Close()
		return nil, fmt.Errorf("新版启动失败：%w", err)
	}
	command.Process.Release()
	return writer, nil
}

// ReleaseRestart lets the new process continue after the HTTP server and data roots close.
func (u *Updater) ReleaseRestart() {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.restartPipe != nil {
		u.restartPipe.Close()
	}
}

func FinishRestart(executable string, args []string) error {
	if len(args) != 2 || args[0] != "--worklog-updated" {
		return nil
	}
	parent, err := strconv.Atoi(args[1])
	if err != nil || parent <= 0 {
		return fmt.Errorf("更新交接的进程编号无效")
	}
	if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
		return err
	}
	if err := waitForPreviousProcess(parent); err != nil {
		return err
	}
	if err := os.Remove(backupPath(executable, parent)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("新版已启动，旧程序暂未清理：%w", err)
	}
	return nil
}

func backupPath(executable string, pid int) string {
	return filepath.Join(filepath.Dir(executable), ".worklog-old-"+strconv.Itoa(pid)+"-"+filepath.Base(executable))
}
