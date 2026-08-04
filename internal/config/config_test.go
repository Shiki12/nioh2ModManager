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

// 回归测试：卸载已启用的 Mod 后，重启 Sync 不应把卡片“复活”为已安装。
// 旧逻辑 Uninstall 不清 Enabled，Sync 用 prev.Enabled 判定 Installed 导致复活。
func TestUninstallPersistsThroughSync(t *testing.T) {
	d := &ModData{}
	d.Mods = []ModInfo{
		{Name: "modA", Path: "C:/repo/modA", Installed: true, Enabled: true},
		{Name: "modB", Path: "C:/repo/modB", Installed: true, Enabled: false},
	}
	// 卸载已启用的 modA
	d.Uninstall("modA")
	if d.Mods[0].Installed {
		t.Fatal("卸载后 Installed 应为 false")
	}
	if d.Mods[0].Enabled {
		t.Fatal("卸载后 Enabled 应为 false（否则 Sync 会误判为已安装）")
	}
	// 模拟重启：磁盘扫描结果里 modA 已无链接（Enabled=false），modB 无变化
	scanned := []ModInfo{
		{Name: "modA", Path: "C:/repo/modA", Enabled: false},
		{Name: "modB", Path: "C:/repo/modB", Enabled: false},
	}
	d.Sync(scanned)
	for _, m := range d.Mods {
		if m.Name == "modA" {
			if m.Installed {
				t.Fatalf("Sync 后 modA 不应复活为已安装: %+v", m)
			}
			if m.Enabled {
				t.Fatalf("Sync 后 modA 不应被标记为启用: %+v", m)
			}
		}
		if m.Name == "modB" && !m.Installed {
			t.Fatalf("modB 未卸载过，Installed 应保持不变: %+v", m)
		}
	}
}
