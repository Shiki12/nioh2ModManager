# 仁王2 Mod 管理器

基于 **Wails v2 + Go + Vue 3 + Ant Design Vue** 的仁王2（Nioh 2）Mod 桌面管理器。以卡片形式管理已安装 / 待安装 Mod，支持 Mod 引擎检测与安装、占用资源识别、冲突检测、HDR 合集批量处理等功能。

## 功能特性

- **Mod 管理**
  - 卡片式网格展示所有 Mod（封面、昵称、分类、启用状态高亮）
  - 一键启用 / 停用 Mod 及 HDR 子 Mod，并同步刷新到游戏目录
  - 启动 / 刷新游戏 Mod 时自动同步启用状态
  - 占用资源识别：安装或编辑时自动识别服装 / 武器占用，生成卡片
  - 自定义昵称、封面图、效果图（添加 / 自动扫描 / 设为封面）
  - 卸载与移除安装记录
- **冲突管理**
  - 自动检测「同一服装 / 武器 slot 被多个 Mod 占用」的冲突
  - 基于 Union-Find 连通分量将冲突 Mod 分组展示
  - 弹窗内直接切换开关解决冲突，冲突数量红色角标实时显示
- **Mod 库**
  - 独立侧边栏页面，集中浏览、搜索、安装 / 卸载托管目录下的 Mod
- **卡片工具**
  - 选择 Mod 文件夹 → 识别封面、填写占用资源、实时预览 → 一键导出 `mod.json`
  - 批量识别 HDR 合集（NN_HDR ... Collection），自动解析子 Mod 占用，未识别项转人工确认
- **Mod 引擎**
  - 启动自动检测游戏根目录是否缺 Mod 引擎
  - 缺失时引导自动安装（或手动安装），侧栏常驻显示引擎状态
- **设置**
  - 游戏目录、Mod 托管目录、更新源地址配置
  - 自动搜索游戏目录（Steam 库探测）
  - 保存后自动重新扫描
- **更新**
  - 启动自动检查新版本，发现新版本弹窗提示并提供下载入口
  - 设置页可手动检查更新
- **日志面板**
  - 实时操作日志，支持一键清空

## 技术栈

| 层 | 技术 |
| --- | --- |
| 桌面框架 | Wails v2 |
| 后端 | Go（标准库 + 内部分包） |
| 前端 | Vue 3 + Vite 3 + Ant Design Vue 4 |
| 打包 | Wails build + NSIS |

## 项目结构

```
src/nioh2mod-js/
├── main.go                    # 入口，Wails 配置与绑定
├── app.go                     # 后端 App 方法（全部业务逻辑）
├── wails.json                 # Wails 项目配置（含 productVersion 等）
├── internal/
│   ├── armordata/             # 服装 / 武器部位数据（armor_parts.json / weapon_parts.json）
│   ├── config/                # 应用配置与 Mod 数据
│   ├── mods/                  # Mod 引擎封装
│   ├── steam/                 # Steam 库扫描
│   ├── dialog/                # 原生对话框
│   └── input/                 # 文本输入
├── frontend/
│   └── src/
│       ├── App.vue            # 布局、顶栏、冲突弹窗、卡片工具弹窗
│       ├── components/        # 页面与组件（ModsPage / ModLibrary / SettingsPage / AuthorTool 等）
│       ├── composables/       # 数据与操作 hooks
│       ├── assets/images/     # 顶栏图片按钮素材
│       └── style.css          # 全局样式（浅色主题）
└── *_test.go                  # 后端单元测试
```

## 开发环境

前置要求：

- [Go 1.20+](https://go.dev/dl/)
- [Node.js 18+](https://nodejs.org/)
- [Wails v2 CLI](https://wails.io/docs/gettingstarted/installation)