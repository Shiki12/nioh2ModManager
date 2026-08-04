# 更新日志

## 2026-08-05

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

---

## 历史版本

### 2026-08-05（早前）

- HDR 合集重装后曾以单一 Mod 形式登记、父卡片丢失子 Mod；新增 `reconcileCompositeRecords()` 在启动加载 / 刷新时自动补齐合集子 Mod 记录。
- HDR 合集自动识别在 `ImportMod` / `GetModConfig` 中补齐，保证安装后仍以组合包形式展示。