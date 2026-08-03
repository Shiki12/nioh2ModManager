package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"nioh2mod-js/internal/config"

	"golang.org/x/text/encoding/simplifiedchinese"
)

// 构造一个模拟的 HDR 合集（GBK 编码的 Read Me），验证批量识别生成 mod.json
func TestBatchGenerateHdrCollection(t *testing.T) {
	root := t.TempDir()
	col := filepath.Join(root, "01_HDR Test Collection")
	mkdir(t, filepath.Join(col, "meshes"))
	mkdir(t, filepath.Join(col, "textures"))
	mkdir(t, filepath.Join(col, "DISABLED 01_[X] Variant"))
	mkdir(t, filepath.Join(col, "THEME COLOR OPTIONS [01]"))

	subA := filepath.Join(col, "01_[A] Special Costume Set")
	mkdir(t, subA)
	writeGBK(t, filepath.Join(subA, "01 TYPE [A] Read Me.txt"),
		";-01_[A] Special Costume Set\r\n"+
			";-生效幻化素材: \\ Effective Materials:\r\n"+
			";-胸 - 讨魔首领 \\ Chest - DemonSlayer\r\n"+
			";-手 - 守护大名之铠 \\ Hand - Principal Governor\r\n"+
			";-腰 - 日轮之子之铠 \\Waist - Child Of Sun\r\n"+
			";-脚 - 刑部轻铠  \\ Foot -  JusticeMinistry\r\n")
	writeFile(t, filepath.Join(subA, "[01A] A1 - x.ini"), "hash = 1234\n")

	// 武器子 Mod：无 Read Me → 应进入待确认，且不写入 submods
	subW := filepath.Join(col, "01_[W] Weapon Neko Hand")
	mkdir(t, subW)
	writeFile(t, filepath.Join(subW, "[01C] A1 - Weapon.ini"), "hash = 9999\n")

	app := &App{logData: &config.LogData{}}
	res, err := app.BatchGenerateModCards(root)
	if err != nil {
		t.Fatalf("BatchGenerateModCards 失败: %v", err)
	}
	if res.Total != 1 {
		t.Fatalf("期望识别 1 个合集，得到 %d", res.Total)
	}
	if res.Generated != 1 {
		t.Fatalf("期望生成 1 个 mod.json，得到 %d", res.Generated)
	}
	if len(res.Mods) != 1 || len(res.Mods[0].SubMods) != 1 {
		t.Fatalf("期望 1 个子 Mod 写入，得到 %d", len(res.Mods[0].SubMods))
	}
	sm := res.Mods[0].SubMods[0]
	if sm.Name != "01_[A] Special Costume Set" {
		t.Fatalf("子 Mod 名错误: %q", sm.Name)
	}
	expect := map[string]string{"胸甲": "讨魔首领之铠-胸甲", "臂甲": "守护大名之铠-臂甲", "膝甲": "日轮之子之铠-膝甲", "腿甲": "刑部轻铠-腿甲"}
	for slot, name := range expect {
		vals := sm.Parts[slot]
		if len(vals) != 1 || vals[0] != name {
			t.Fatalf("槽位 %s 期望 %q，得到 %v", slot, name, sm.Parts[slot])
		}
	}
	// 待确认应包含武器子 Mod（无 Read Me）
	foundPending := false
	for _, p := range res.Pending {
		if p.SubMod == "01_[W] Weapon Neko Hand" {
			foundPending = true
		}
	}
	if !foundPending {
		t.Fatalf("期望武器子 Mod 进入待确认，实际: %+v", res.Pending)
	}

	// 验证磁盘上的 mod.json 内容
	data, err := os.ReadFile(filepath.Join(col, "mod.json"))
	if err != nil {
		t.Fatal("mod.json 未生成: " + err.Error())
	}
	var cfg modConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("mod.json 解析失败: %v", err)
	}
	if len(cfg.SubMods) != 1 {
		t.Fatalf("mod.json submods 数量错误: %d", len(cfg.SubMods))
	}
	if len(cfg.Parts) != 0 {
		t.Fatalf("父级 parts 应为空，得到 %v", cfg.Parts)
	}
}

func mkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
}
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
func writeGBK(t *testing.T, path, content string) {
	t.Helper()
	enc := simplifiedchinese.GBK.NewEncoder()
	buf, err := enc.Bytes([]byte(content))
	if err != nil {
		t.Fatalf("GBK 编码失败: %v", err)
	}
	if err := os.WriteFile(path, buf, 0644); err != nil {
		t.Fatal(err)
	}
}
