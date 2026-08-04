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

---

## 历史版本

### 2026-08-05（早前）

- HDR 合集重装后曾以单一 Mod 形式登记、父卡片丢失子 Mod；新增 `reconcileCompositeRecords()` 在启动加载 / 刷新时自动补齐合集子 Mod 记录。
- HDR 合集自动识别在 `ImportMod` / `GetModConfig` 中补齐，保证安装后仍以组合包形式展示。