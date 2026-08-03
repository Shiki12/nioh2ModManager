package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogPlainTextRoundTrip(t *testing.T) {
	d := &LogData{}
	d.Append("操作A 成功")
	d.Append("操作B 失败: x")
	got := d.All()
	if len(got) != 2 {
		t.Fatalf("内存条数 = %d, want 2", len(got))
	}
	if got[0].Message != "操作A 成功" || got[1].Message != "操作B 失败: x" {
		t.Fatalf("消息不符: %+v", got)
	}
	if got[0].Time == "" || got[1].Time == "" {
		t.Fatal("时间戳为空")
	}
	if d.Logs == nil {
		t.Fatal("Append 后 Logs 不应为 nil")
	}
	d.Clear()
	if len(d.Logs) != 0 {
		t.Fatalf("Clear 后条数 = %d", len(d.Logs))
	}
}

func TestLoadLogsFromFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "modman.log")
	content := "[2026-08-01 00:12:33] 应用启动\n[2026-08-01 00:12:40] 已启用 Mod: kante\n[2026-08-01 00:13:00] 已禁用 Mod: kante\n"
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	loaded := &LogData{}
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimRight(line, "\r")
		if len(line) < 21 || line[0] != '[' {
			continue
		}
		if idx := strings.Index(line, "] "); idx > 0 {
			loaded.Logs = append(loaded.Logs, LogEntry{Time: line[1:idx], Message: line[idx+2:]})
		}
	}
	if len(loaded.Logs) != 3 {
		t.Fatalf("解析条数 = %d, want 3", len(loaded.Logs))
	}
	if loaded.Logs[1].Time != "2026-08-01 00:12:40" || loaded.Logs[1].Message != "已启用 Mod: kante" {
		t.Fatalf("解析不符: %+v", loaded.Logs[1])
	}
}
