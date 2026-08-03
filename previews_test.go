package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"nioh2mod-js/internal/config"
)

func TestCollectFolderImages(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "b.jpg"), "x")
	writeFile(t, filepath.Join(dir, "a.png"), "x")
	writeFile(t, filepath.Join(dir, "notes.txt"), "x")
	mkdir(t, filepath.Join(dir, "01_[A]"))
	writeFile(t, filepath.Join(dir, "01_[A]", "c.webp"), "x")
	writeFile(t, filepath.Join(dir, "01_[A]", "z.gif"), "x")

	// 仅根目录
	root := collectFolderImages(dir, dir, false, 12)
	if len(root) != 2 || !sort.StringsAreSorted(root) {
		t.Fatalf("根目录效果图收集错误: %v", root)
	}
	if root[0] != "a.png" || root[1] != "b.jpg" {
		t.Fatalf("根目录效果图应排序且排除非图片: %v", root)
	}

	// 递归收集（含子目录图片，相对 dir 路径）
	all := collectFolderImages(dir, dir, true, 12)
	if len(all) != 4 {
		t.Fatalf("递归效果图收集错误: %v", all)
	}
	joined := strings.Join(all, " ")
	if !strings.Contains(joined, "01_[A]/c.webp") || !strings.Contains(joined, "01_[A]/z.gif") {
		t.Fatalf("递归收集应包含子目录图片（相对路径）: %v", all)
	}

	// limit 限制
	limited := collectFolderImages(dir, dir, true, 2)
	if len(limited) != 2 {
		t.Fatalf("limit 未生效: %v", limited)
	}
}

func TestReadModConfigPreviews(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "mod.json"), `{
  "nickname": "HDR合集",
  "cover": "cover.jpg",
  "previews": ["cover.jpg", "b.jpg"],
  "submods": [
    {"name": "01_[A]", "cover": "01_[A]/p.png", "previews": ["01_[A]/p.png", "01_[A]/q.png"]}
  ]
}`)
	app := &App{logData: &config.LogData{}}
	_, _, cover, previews, _, submods, found := app.readModConfig(dir)
	if !found {
		t.Fatal("未识别到 mod.json")
	}
	if cover != "cover.jpg" {
		t.Fatalf("封面解析错误: %q", cover)
	}
	if len(previews) != 2 || previews[0] != "cover.jpg" || previews[1] != "b.jpg" {
		t.Fatalf("父级效果图解析错误: %v", previews)
	}
	if len(submods) != 1 {
		t.Fatalf("子 Mod 解析错误: %v", submods)
	}
	sm := submods[0]
	if len(sm.Previews) != 2 || sm.Previews[0] != "01_[A]/p.png" {
		t.Fatalf("子 Mod 效果图解析错误: %v", sm.Previews)
	}

	// 无 previews 字段时回退为封面
	writeFile(t, filepath.Join(dir, "mod.json"), `{"cover":"a.png"}`)
	_, _, _, previews2, _, _, _ := app.readModConfig(dir)
	if len(previews2) != 1 || previews2[0] != "a.png" {
		t.Fatalf("previews 缺省应回退封面: %v", previews2)
	}
}

// refreshModPreviews 对组合包：父级取根目录图片、子 Mod 递归取子目录图片并写回数据文件
func TestRefreshModPreviewsComposite(t *testing.T) {
	repo := t.TempDir()
	modDir := filepath.Join(repo, "HDR Test")
	mkdir(t, modDir)
	writeFile(t, filepath.Join(modDir, "cover.jpg"), "x")
	writeFile(t, filepath.Join(modDir, "extra.png"), "x")
	mkdir(t, filepath.Join(modDir, "01_[A]"))
	writeFile(t, filepath.Join(modDir, "01_[A]", "mod.ini"), "[mod]\n")
	writeFile(t, filepath.Join(modDir, "01_[A]", "p1.jpg"), "x")
	mkdir(t, filepath.Join(modDir, "01_[A]", "sub"))
	writeFile(t, filepath.Join(modDir, "01_[A]", "sub", "p2.png"), "x")

	app := &App{
		cfg:     &config.App{ModsRepo: repo},
		modData: &config.ModData{Mods: []config.ModInfo{{Name: "HDR Test", Path: modDir, SubMods: []config.SubModInfo{{Name: "01_[A]"}}}}},
		logData: &config.LogData{},
	}
	md := app.modData.Find("HDR Test")
	app.refreshModPreviews(md, false)

	// 父级：仅根目录图片
	if len(md.Preview) != 2 {
		t.Fatalf("父级效果图应为根目录 2 张: %v", md.Preview)
	}
	// 子 Mod：递归收集（相对父 Mod 目录路径）
	var subPreviews []string
	for _, sm := range md.SubMods {
		if sm.Name == "01_[A]" {
			subPreviews = sm.Preview
		}
	}
	if len(subPreviews) != 2 {
		t.Fatalf("子 Mod 效果图应递归收集 2 张: %v", subPreviews)
	}
	has := func(list []string, want string) bool {
		for _, s := range list {
			if s == want {
				return true
			}
		}
		return false
	}
	if !has(subPreviews, "01_[A]/p1.jpg") || !has(subPreviews, "01_[A]/sub/p2.png") {
		t.Fatalf("子 Mod 效果图应为相对父目录路径: %v", subPreviews)
	}

	// onlyIfEmpty=true 时不覆盖已存在的效果图
	md.Preview = []string{"manual.jpg"}
	app.refreshModPreviews(md, true)
	if len(md.Preview) != 1 || md.Preview[0] != "manual.jpg" {
		t.Fatalf("onlyIfEmpty 不应覆盖已有效果图: %v", md.Preview)
	}
}

// AddModPreview / RemoveModPreview / GetSubModPreviews 增删查闭环
func TestModPreviewAddRemove(t *testing.T) {
	repo := t.TempDir()
	modDir := filepath.Join(repo, "TestMod")
	mkdir(t, modDir)
	src := filepath.Join(t.TempDir(), "in.png")
	writeFile(t, src, "img")

	app := &App{
		cfg:     &config.App{ModsRepo: repo},
		modData: &config.ModData{Mods: []config.ModInfo{{Name: "TestMod", Path: modDir}}},
		logData: &config.LogData{},
	}

	added, err := app.AddModPreview("TestMod", src)
	if err != nil {
		t.Fatalf("AddModPreview 失败: %v", err)
	}
	if added != "preview.png" {
		t.Fatalf("效果图应命名为 preview.png: %q", added)
	}
	if _, err := os.Stat(filepath.Join(modDir, "preview.png")); err != nil {
		t.Fatalf("效果图文件未写入: %v", err)
	}
	got, _ := app.GetModPreviews("TestMod")
	if len(got) != 1 || got[0] != "preview.png" {
		t.Fatalf("GetModPreviews 错误: %v", got)
	}

	// 二次添加：序号递增，不覆盖
	added2, _ := app.AddModPreview("TestMod", src)
	if added2 != "preview_2.png" {
		t.Fatalf("第二次添加应命名 preview_2.png: %q", added2)
	}

	if err := app.RemoveModPreview("TestMod", added); err != nil {
		t.Fatalf("RemoveModPreview 失败: %v", err)
	}
	got, _ = app.GetModPreviews("TestMod")
	if len(got) != 1 || got[0] != "preview_2.png" {
		t.Fatalf("移除后效果图列表错误: %v", got)
	}

	// 子 Mod 效果图增删
	app.modData.SetSubMods("TestMod", []config.SubModInfo{{Name: "01_[A]"}})
	mkdir(t, filepath.Join(modDir, "01_[A]"))
	subRel, err := app.AddSubModPreview("TestMod", "01_[A]", src)
	if err != nil {
		t.Fatalf("AddSubModPreview 失败: %v", err)
	}
	if subRel != "01_[A]/preview.png" {
		t.Fatalf("子 Mod 效果图应为相对父目录路径: %q", subRel)
	}
	if _, err := os.Stat(filepath.Join(modDir, "01_[A]", "preview.png")); err != nil {
		t.Fatalf("子 Mod 效果图文件未写入: %v", err)
	}
	subs, _ := app.GetSubModPreviews("TestMod", "01_[A]")
	if len(subs) != 1 || subs[0] != subRel {
		t.Fatalf("GetSubModPreviews 错误: %v", subs)
	}
	if err := app.RemoveSubModPreview("TestMod", "01_[A]", subRel); err != nil {
		t.Fatalf("RemoveSubModPreview 失败: %v", err)
	}
	subs, _ = app.GetSubModPreviews("TestMod", "01_[A]")
	if len(subs) != 0 {
		t.Fatalf("移除后子 Mod 效果图应清空: %v", subs)
	}

	// 子 Mod 封面设置
	if err := app.SetSubModCover("TestMod", "01_[A]", "01_[A]/preview.png"); err != nil {
		t.Fatalf("SetSubModCover 失败: %v", err)
	}
	md2 := app.modData.Find("TestMod")
	if len(md2.SubMods) != 1 || md2.SubMods[0].Cover != "01_[A]/preview.png" {
		t.Fatalf("子 Mod 封面未更新: %+v", md2.SubMods)
	}
}
