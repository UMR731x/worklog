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
	"worklog/internal/update"
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
	if err := update.FinishRestart(executable, os.Args[1:]); err != nil {
		platform.ReportError("更新清理提示", err.Error())
	}
	work, err := workspace.Open(filepath.Dir(executable))
	if err != nil {
		return err
	}
	updates := update.New(executable, update.Version)
	defer func() {
		work.Close()
		updates.ReleaseRestart()
	}()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go updates.Check(ctx, false)
	application := &http.Server{
		Handler:     server.New(work, web.Assets, stop, updates),
		BaseContext: func(net.Listener) context.Context { return ctx },
	}
	served := make(chan error, 1)
	go func() {
		served <- application.Serve(listener)
		stop()
	}()
	address := "http://" + listener.Addr().String()
	fmt.Printf("工作手记 %s 已启动：%s\n通过系统托盘或页面右上角退出应用。\n", update.Version, address)
	open := func() {
		if err := platform.OpenBrowser(address); err != nil {
			platform.ReportError("请手动打开浏览器", address+"\n"+err.Error())
		}
	}
	trayErr := platform.RunTray(ctx, web.Icon, open, stop)
	stop()
	shutdownErr := application.Shutdown(context.Background())
	serveErr := <-served
	if errors.Is(serveErr, http.ErrServerClosed) {
		serveErr = nil
	}
	return errors.Join(trayErr, shutdownErr, serveErr)
}
