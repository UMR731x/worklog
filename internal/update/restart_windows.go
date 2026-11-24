package update

import (
	"errors"

	"golang.org/x/sys/windows"
)

func waitForPreviousProcess(pid int) error {
	process, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return nil // The old process already exited before the new process opened its handle.
	}
	if err != nil {
		return err
	}
	defer windows.CloseHandle(process)
	_, err = windows.WaitForSingleObject(process, windows.INFINITE)
	return err
}
