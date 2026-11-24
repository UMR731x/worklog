package platform

import (
	"fmt"
	"syscall"
	"unsafe"
)

func OpenBrowser(address string) error {
	operation, err := syscall.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	url, err := syscall.UTF16PtrFromString(address)
	if err != nil {
		return err
	}
	result, _, _ := syscall.NewLazyDLL("shell32.dll").NewProc("ShellExecuteW").Call(
		0, uintptr(unsafe.Pointer(operation)), uintptr(unsafe.Pointer(url)), 0, 0, 1,
	)
	if result <= 32 {
		return fmt.Errorf("无法打开默认浏览器（Windows 错误码 %d）", result)
	}
	return nil
}

func ReportError(title, message string) {
	caption, _ := syscall.UTF16PtrFromString(title)
	text, _ := syscall.UTF16PtrFromString(message)
	syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW").Call(
		0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(caption)), 0x10,
	)
}
