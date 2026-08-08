# 更新日志

## 2026-08-08

### 🐛 修复：关闭文件夹/文件选择弹窗导致整个应用退出

- **根因**：第三方库 zenity 在 Wails 的**后台 IPC 线程**上弹模态对话框（内部 `runtime.LockOSThread` + 在该线程跑嵌套消息循环），并用 `Attach(mainWindowHandle())` 把主窗口设为 owner。Win32 模态普通对话框必须在**主窗口所在线程**调度；zenity 从后台线程弹窗、关闭时归还 owner/焦点，与 WebView2 主窗口的消息泵冲突，主窗口收到异常关闭信号（默认 `HideWindowOnClose=false`）→ 整个应用随之退出。
- **修复**：`App.SelectDirectory` / `App.SelectImageFile` 改用 Wails 自带 `runtime.OpenDirectoryDialog` / `runtime.OpenFileDialog`（内部 `invokeSync` 将对话框正确调度到主窗口线程）；删除整个 `internal/dialog` 包并移除 zenity 依赖（`go mod tidy` 清理 go.mod/go.sum）。
- **行为变化**：取消选择不再返回错误，而是返回空字符串 + 无错误（前端 `if (!dir) return` 已兼容），弹窗关闭后应用保持运行。

## 2026-08-05

### ⚡ 性能优化：滚动列表 / 打开卡片卡顿修复

- **服务端按需缩略图（新增 `thumbnail.go`）**
  - `/modfile`、`/localfile` 支持 `?w=<最大边长>`：用标准库 `image` 解码（png/jpeg/gif），双线性缩放到目标边长，非 PNG 统一 JPEG Q82 重编码、PNG 保留透明保持原格式。
  - 内存缓存 key=`路径|边长|size|mtime`（改图自动失效），上限 256 条、超限清空重建。
  - webp 等标准库无法解码的格式自动回退原文件（不压缩，不影响兼容）。
  - 缩略图响应 `Cache-Control: public, max-age=86400`（key 已含 mtime）；原图响应 `max-age=300`。
- **前端按显示场景取小图**：网格封面 `w=360`、缩略图 `w=64`，编辑/子 Mod 弹窗小封面 `w=128`、效果图小图 `w=96`，卡片工具图片 `w=160`；**全屏放大预览仍请求原图**保证清晰（缺省不带 `w`）。
- **图片懒加载 + 异步解码**：`ModCard.vue` 封面与效果图、`App.vue` 弹窗封面/效果图、`AuthorTool.vue` 图片列表的 `<img>` 全部加 `loading="lazy"` + `decoding="async"`。
- **弹窗内子 Mod 卡片隐藏缩略图**：`ModCard.vue` 新增 `hideThumbs` 属性，编辑/安装弹窗内的子 Mod 卡片传 `hide-thumbs`，组合包弹窗不再一次性加载几十张全尺寸图。
- **卡片固定等高**：`ModsPage.vue` 网格与 `App.vue` 子 Mod 网格加 `grid-auto-rows: 1fr`，`ModCard.vue` 卡片 `height:100%` + flex 纵向铺满、效果图行 `margin-top:auto` 贴底，卡片不再因内容多少高低不齐。

### 🪟 修复刷新 Mod 带出 "GDI+ Window"

- 根因：刷新时 `input.BringToForeground` 做了过重的激活（`BringWindowToTop` + 模拟 Alt 键 + `AttachThreadInput`），把游戏进程自带的 `GDI+ Window`（gdiplus 内部消息窗口）激活进 Alt-Tab/任务栏。
- 修复：`BringToForeground` 改为参照 `verify/verifyPresson` 的轻量方式——仅 `ShowWindow(SW_RESTORE)` + `SetForegroundWindow`，删除 `BringWindowToTop`/模拟 Alt/`GetForegroundWindow`/`AttachThreadInput`（对应 proc、常量一并移除）。捕获窗口 + 模拟按键本身不会产生该窗口。
- **补充确认**：`GDI+ Window` 还会因旧的 `SHBrowseForFolder`/`GetOpenFileName` 对话框加载 gdiplus 而出现在本进程（游戏未启动时明显，见下条对话框改造）。

### 📂 文件/文件夹选择框改用现代对话框（zenity）

- 根因：`internal/dialog` 原先用 `SHBrowseForFolder`(shell32) 和 `GetOpenFileName`(comdlg32) 手拼 `OPENFILENAME` 裸结构，老式对话框会把 `gdiplus.dll` 拉进进程并创建 `GDI+ Window`；且裸结构代码脆弱。
- 改造：改用 `github.com/ncruces/zenity`（Windows 端底层为现代 `IFileOpenDialog`/COMDLG32）：`SelectDirectory`=目录选择、`SelectFile`=文件选择，保留原 `filter`（`\x00` 分隔）与"用户取消"语义；owner 主窗口用原 `mainWindowHandle()` 兜底逻辑（`GetForegroundWindow`→`GetActiveWindow`→`FindWindow`）。
- 尝试过但未采用：手写 COM vtable（`harry1453/go-common-file-dialog` 曾引入又移除，最终以 zenity 落地；zenity 需 Go ≤1.24 用 v0.10.14）。

### 🚫 仅游戏运行时允许"刷新游戏 Mod"

- 背景：游戏未启动时，若触发刷新会在 Alt-Tab 中出现多余窗口（属应用/WebView2 常驻辅助窗口在游戏未运行时的暴露，与对话框改造成 zenity 无关）。
- 曲线解决：新增后端 `IsGameRunning()`（复用 `input.FindGameWindow` 按窗口标题识别）绑定给前端；前端每 3s 轮询 `gameRunning`，顶部"刷新游戏 Mod"按钮在游戏未启动时**置灰禁用**（`.is-disabled`，tooltip 提示"请先启动游戏"），且 `refreshGame` 入口先校验，未运行直接提示返回，杜绝"游戏未启动却刷新"。

### 🎮 启动游戏按钮改为启动/停止二合一

- 新增后端 `StopGame()`（`input.FindGamePID` 按游戏窗口取 PID → `OpenProcess`+`TerminateProcess` 强制结束）与 `input.FindGamePID()` 绑定给前端。
- 前端持续（每 3s）监听 `gameRunning`：游戏未运行 → 绿色「▶ 启动游戏」；游戏运行中 → 红色「✕ 立即停止」，点击停止游戏；启动/停止后立即刷新运行状态。

---

## 历史版本

### 🐛 冲突检测 / 冲突弹窗修复

- **修复：有冲突时点击「冲突」按钮弹窗打不开、程序卡死**
  - 根因：`App.vue` 的 `conflictNameToMod` 用 `for (const m of mods || [])` 直接遍历 Vue `ref`（`mods` 为 `ref([])`，不可迭代），仅当存在真实冲突时该 `computed` 才被求值，随即抛出 `TypeError: mods is not iterable`，渲染报错循环导致界面卡死。
  - 修复：改为 `for (const m of mods.value || [])`。
- **修复：冲突徽标不显示红色、数字不变**
  - 根因：`conflictCount` 写成 `conflicts?.length`，而 `conflicts` 为 Vue `ref`，取值恒为 `undefined` → 数字永远为 0、徽标恒为灰色。
  - 修复：改为 `conflicts?.value?.length`。
- **冲突徽标增强**：新增 `.has-conflict` 状态样式（红色、放大至 24px、脉冲动画），有冲突时徽标红字放大醒目提示；无冲突保持灰色小徽标（`.no-conflict`）。

### 🐛 HDR 组合包卸载 / 清除记录修复（后端）

- **验证确认**：正常卸载 HDR 组合包（`UninstallMod`）会通过 `DisableComposite`（`os.RemoveAll`）整体移除游戏 Mods 目录下的组合父目录，**连同所有已启用子 Mod 的软链一并删除**，数据父级/子 Mod 回到未启用、锁文件清除。逻辑正确，无残留软链（新增回归测试 `TestUninstallModCompositeRemovesSubModLinks`）。
- **修复：清除组合 Mod 记录残留孤儿目录**（`RemoveModRecord`）：原先只处理普通符号链接，对组合/HDR Mod（游戏目录下为真实目录）不生效 → 清除记录后父目录与子 Mod 软链残留。改为依据 `IsActive` 区分符号链接（`Disable`）与真实目录（`DisableComposite`）整体移除（新增回归测试 `TestRemoveModRecordCompositeCleansGameModsDir`）。

### 🐛 刷新误关 Mod 引擎开关修复（后端）

- **修复：刷新操作（F10→F2）会把已开启的 Mod 关闭**（换同部位服装刷新时复现）
  - 根因：引擎 `d3dx.ini` 中 **F2 是「切换」键**（`[KeyToggleMods]`，`$mods = 0,1`、`type = cycle`），F10 才是纯重载（`reload_config`/`reload_fixes`）。`input.RefreshMods` 原先固定发送 **F10 → F2**，等价于每次刷新都翻转一次 Mod 总开关——Mod 本开启时被关掉。
  - 修复：刷新只发送 **F10**（纯重载），不再发送 F2；同时移除已无调用点的 `SendF2` 与 `VK_F2`。
  - 已知限制：无法从游戏外部读取引擎 `$mods` 运行时值（`CheckModEngine` 只能判断引擎文件是否已安装），因此游戏启动后若引擎开关处于关闭态，F10 刷新不会自动开启；需手动在游戏内按 F2。

### 🐛 Mod 库分页大小无法切换修复（前端）

- **修复：Mod 库页面「每页条数」选择器切换无效**
  - 根因：`ModLibrary.vue` 的 `tablePagination` 是普通对象，非响应式；ant-design-vue v4 内部修改 pageSize 后不会触发表格重新渲染，导致切分页大小无效。
  - 修复：改为 `ref` 包裹（响应式）。
  - **分页选项调整**：`['10','20','50']` → `['5','10','20']`，默认每页条数 10 → 5。
  - **进一步修复（仍无法选择时）**：将分页改为**完全受控**（显式维护 `current` + `pageSize`，通过 `onChange` / `onShowSizeChange` 回调写回 ref），确保切换条数后表格强制重渲染；并**移除 `.ant-table` 上的 `overflow: hidden`**——该样式会裁剪隐藏分页「每页条数」下拉列表，导致点不开/无法选择。

---

## 历史版本

### 2026-08-05（早前）

- HDR 合集重装后曾以单一 Mod 形式登记、父卡片丢失子 Mod；新增 `reconcileCompositeRecords()` 在启动加载 / 刷新时自动补齐合集子 Mod 记录。
- HDR 合集自动识别在 `ImportMod` / `GetModConfig` 中补齐，保证安装后仍以组合包形式展示。