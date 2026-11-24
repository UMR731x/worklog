package platform

import (
	"fmt"
	"os"
	"os/exec"
)

func OpenBrowser(address string) error {
	command := exec.Command("xdg-open", address)
	if err := command.Start(); err != nil {
		return err
	}
	go func() {
		if err := command.Wait(); err != nil {
			ReportError("请手动打开浏览器", address)
		}
	}()
	return nil
}

func ReportError(title, message string) {
	fmt.Fprintf(os.Stderr, "%s：%s\n", title, message)
}
