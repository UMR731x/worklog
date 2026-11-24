package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"worklog/internal/platform"
	"worklog/internal/server"
	"worklog/internal/workspace"
	"worklog/web"
)

func main() {
	if err := run(); err != nil {
		platform.ReportError("工作手记启动失败", err.Error())
		os.Exit(1)
	}
}

func run() error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	work, err := workspace.Open(filepath.Dir(executable))
	if err != nil {
		return err
	}
	defer work.Close()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	application := &http.Server{Handler: server.New(work, web.Assets, stop)}
	served := make(chan error, 1)
	go func() { served <- application.Serve(listener) }()
	address := "http://" + listener.Addr().String()
	fmt.Printf("工作手记已启动：%s\n数据保存在所选工作目录。通过页面右上角退出应用。\n", address)
	if err := platform.OpenBrowser(address); err != nil {
		platform.ReportError("请手动打开浏览器", address+"\n"+err.Error())
	}
	select {
	case <-ctx.Done():
		return application.Shutdown(context.Background())
	case err := <-served:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
