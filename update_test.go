package main

import (
	"strings"
	"testing"

	"nioh2mod-js/internal/config"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"0.1.2", "0.1.2", 0},
		{"0.1.2", "0.2.0", -1},
		{"0.2.0", "0.1.2", 1},
		{"0.10.0", "0.9.9", 1},
		{"1.0", "0.9.9", 1},
		{"0.5", "0.5.0", 0},
	}
	for _, c := range cases {
		if got := compareVersions(c.a, c.b); got != c.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestCheckUpdateNoSource(t *testing.T) {
	app := &App{cfg: &config.App{}, modData: &config.ModData{}, logData: &config.LogData{}}
	info := app.CheckUpdate()
	if info.HasUpdate {
		t.Fatal("未配置更新源时不应判定有更新")
	}
	if !strings.Contains(info.Message, "未配置更新源") {
		t.Fatalf("应提示未配置更新源: %q", info.Message)
	}
	if info.CurrentVersion != AppVersion {
		t.Fatalf("当前版本应返回 %s，得到 %s", AppVersion, info.CurrentVersion)
	}
}

func TestSetUpdateUrl(t *testing.T) {
	app := &App{cfg: &config.App{}, logData: &config.LogData{}}
	if err := app.SetUpdateUrl("  https://example.com/version.json  "); err != nil {
		t.Fatal(err)
	}
	if app.cfg.UpdateURL != "https://example.com/version.json" {
		t.Fatalf("更新源地址应去除首尾空格: %q", app.cfg.UpdateURL)
	}
}
