package server

import (
	"net/http"
	"strings"
	"testing"
)

func TestThemeIsAppliedBeforePagePaint(t *testing.T) {
	handler, token := testAPI(t)
	for _, theme := range []string{"light", "dark", "system"} {
		response := callAPI(handler, token, "PUT", "/api/theme", `{"theme":"`+theme+`"}`)
		if response.Code != http.StatusNoContent {
			t.Fatalf("set theme: %d %s", response.Code, response.Body)
		}
		page := callAPI(handler, token, "GET", "/", "")
		if !strings.Contains(page.Body.String(), `<html lang="zh-CN" data-theme="`+theme+`">`) {
			t.Fatalf("saved theme missing from initial HTML: %s", theme)
		}
	}
}
