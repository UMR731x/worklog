package platform

import (
	"context"

	"github.com/gogpu/systray"
)

func RunTray(ctx context.Context, icon []byte, open, quit func()) error {
	tray := systray.New()
	menu := systray.NewMenu()
	menu.Add("打开工作手记", open)
	menu.AddSeparator()
	menu.Add("彻底退出", quit)
	tray.SetIcon(icon).SetTooltip("工作手记 · Worklog").SetMenu(menu).Show()
	tray.OnClick(open)
	go func() {
		<-ctx.Done()
		tray.Remove()
	}()
	open()
	return tray.Run()
}
