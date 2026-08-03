package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nioh2mod-js/internal/armordata"
	"nioh2mod-js/internal/config"
)

// 往返测试：作者工具 WriteModCard 生成的 mod.json 能被 readModConfig 正确解析（含混合占用）
func TestModCardRoundTrip(t *testing.T) {
	dir := t.TempDir()
	app := &App{logData: &config.LogData{}}

	// 混合占用：服装部位 + 武器
	weapon := armordata.AllWeapons()
	if len(weapon) == 0 {
		t.Fatal("无法加载武器数据")
	}
	slots := armordata.AllSlots()
	if len(slots) == 0 {
		t.Fatal("无法加载服装部位数据")
	}
	card := ModCard{
		Nickname: "测试混合Mod",
		Cover:    `C:\some\preview.jpg`,
		Parts:    map[string][]string{slots[0].Name: {slots[0].Parts[0].Name}, "武器": {weapon[0].Name}},
	}
	res, err := app.WriteModCard(dir, card)
	if err != nil {
		t.Fatalf("WriteModCard 失败: %v", err)
	}
	if res.Category != "mixed" {
		t.Fatalf("期望 mixed，得到 %q", res.Category)
	}

	// 读取文件并解析：校验"武器"键在序列化文本中位于服装部位键之后
	data, err := os.ReadFile(filepath.Join(dir, "mod.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg modConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(cfg.Parts)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	idxArmor := len(s)
	for k := range cfg.Parts {
		if k == "武器" {
			continue
		}
		if i := strings.Index(s, `"`+k+`"`); i >= 0 && i < idxArmor {
			idxArmor = i
		}
	}
	if idxArmor >= len(s) {
		t.Fatalf("未找到服装部位键: %s", s)
	}
	if i := strings.Index(s[idxArmor:], `"武器"`); i < 0 {
		t.Fatalf("武器键应位于服装部位之后: %s", s)
	}

	// readModConfig 解析回来
	nickname, category, cover, _, parts2, _, found := app.readModConfig(dir)
	if !found {
		t.Fatal("未识别到 mod.json")
	}
	if category != "mixed" {
		t.Fatalf("解析分类期望 mixed，得到 %q", category)
	}
	if nickname != "测试混合Mod" {
		t.Fatalf("昵称解析错误: %q", nickname)
	}
	if cover != "preview.jpg" {
		t.Fatalf("封面应存文件名 preview.jpg，得到 %q", cover)
	}
	if _, ok := parts2["武器"]; !ok {
		t.Fatalf("武器占用未解析出来: %v", parts2)
	}
}

// 旧格式兼容：老版本 mod.json 只写分类+单一占用，应能正确解析
func TestLegacyModConfigCompat(t *testing.T) {
	dir := t.TempDir()
	app := &App{logData: &config.LogData{}}
	slots := armordata.AllSlots()
	weapon := armordata.AllWeapons()

	// 1) 老版武器 mod：category=weapon，parts 只在"武器"键
	legacyWeapon := `{"nickname":"旧武器","category":"weapon","cover":"a.png","parts":{"武器":"` + weapon[0].Name + `"}}`
	if err := os.WriteFile(filepath.Join(dir, "mod.json"), []byte(legacyWeapon), 0644); err != nil {
		t.Fatal(err)
	}
	_, category, _, _, parts, _, found := app.readModConfig(dir)
	if !found || category != "weapon" {
		t.Fatalf("旧武器 mod 解析失败: category=%q", category)
	}
	if _, ok := parts["武器"]; !ok {
		t.Fatal("旧武器 mod 的武器占用未解析")
	}

	// 2) 老版服装 mod：无 category，parts 只在服装部位
	legacyArmor := `{"nickname":"旧服装","parts":{"` + slots[0].Name + `":"` + slots[0].Parts[0].Name + `"}}`
	if err := os.WriteFile(filepath.Join(dir, "mod.json"), []byte(legacyArmor), 0644); err != nil {
		t.Fatal(err)
	}
	_, category, _, _, parts, _, found = app.readModConfig(dir)
	if !found || category != "armor" {
		t.Fatalf("旧服装 mod 解析失败: category=%q", category)
	}
	if _, ok := parts[slots[0].Name]; !ok {
		t.Fatal("旧服装 mod 的服装占用未解析")
	}
}
