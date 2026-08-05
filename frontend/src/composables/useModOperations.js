import { ref, h } from 'vue'
import { Modal, message } from 'ant-design-vue'
import { WarningOutlined } from '@ant-design/icons-vue'
import {
  SearchNioh2Root, RefreshGameMods,
  EnableMod, EnableModAndRefresh, DisableMod, DisableModAndRefresh, ScanMods as ScanModsGo,
  EnableHdrSubMod, EnableHdrSubModAndRefresh, DisableHdrSubMod, DisableHdrSubModAndRefresh,
  SetModsRepo, SetGameRoot, SelectDirectory, DetectGameRoot, InstallMod, GetMods, CheckModConflicts,
  CheckAllModConflicts, SetUpdateUrl,
} from '../../wailsjs/go/main/App'

export function useModOperations(settingsForm, mods, needSetup, needModsRepoSetup, addLog, refreshLogs) {
  const detectingGameDir = ref(false)
  const detectedGameRoot = ref('')
  const detectedGameDirError = ref('')
  const gameDirConfirmed = ref(false)

  /** 已启用 Mod 之间的资源占用冲突列表（后端整体检查），供前端展示是否冲突及冲突明细 */
  const conflicts = ref([])

  /** 刷新全部已启用 Mod 之间的冲突检测结果 */
  async function loadConflicts() {
    try {
      const arr = await CheckAllModConflicts()
      conflicts.value = Array.isArray(arr) ? arr : []
    } catch (e) { addLog(`检查 Mod 冲突失败: ${e}`) }
  }

  /** 核对游戏目录：系统自动扫描并展示结果，供用户在 Mod 托管目录设置中确认 */
  async function detectGameRootForConfirm() {
    detectingGameDir.value = true
    detectedGameDirError.value = ''
    gameDirConfirmed.value = false
    addLog('正在核对游戏目录...')
    try {
      const root = await DetectGameRoot()
      if (root) {
        detectedGameRoot.value = root
        settingsForm.gameRoot = root
        await refreshLogs()
      } else {
        detectedGameDirError.value = '未找到游戏安装目录'
        addLog('未找到游戏安装目录')
      }
    } catch (e) {
      if (settingsForm.gameRoot) {
        detectedGameRoot.value = settingsForm.gameRoot
        addLog(`已使用保存的游戏目录: ${settingsForm.gameRoot}`)
      } else {
        detectedGameDirError.value = String(e)
        addLog('未检测到游戏目录，请手动指定')
      }
    } finally {
      detectingGameDir.value = false
    }
  }

  /** 核对失败时手动指定游戏目录 */
  async function pickGameDirManual() {
    try {
      const dir = await SelectDirectory()
      if (dir) {
        detectedGameRoot.value = dir
        settingsForm.gameRoot = dir
        detectedGameDirError.value = ''
        gameDirConfirmed.value = false
        addLog(`已手动指定游戏目录: ${dir}`)
      }
    } catch (_) {}
  }
  async function searchGame() {
    addLog('正在自动搜索游戏目录...')
    try { const root = await SearchNioh2Root(); if (root) { settingsForm.gameRoot = root; needSetup.value = false; await refreshLogs() } }
    catch (e) { addLog('未找到游戏目录，请手动指定'); needSetup.value = true }
  }
  async function confirmGameRoot() {
    if (!settingsForm.gameRoot) return
    await SetGameRoot(settingsForm.gameRoot); needSetup.value = false; await refreshLogs()
  }
  async function selectGameDir() { try { const dir = await SelectDirectory(); if (dir) settingsForm.gameRoot = dir } catch (_) {} }
  async function confirmModsRepo() {
    if (!settingsForm.modsRepo) { addLog('请先指定 Mod 托管目录'); return }
    if (!gameDirConfirmed.value) { addLog('请先确认游戏目录'); return }
    if (detectedGameRoot.value) { await SetGameRoot(detectedGameRoot.value) }
    // SetModsRepo 后端已同步过数据，这里直接读取数据文件即可
    await SetModsRepo(settingsForm.modsRepo); needModsRepoSetup.value = false; await loadMods()
  }
  async function selectModsDir() { try { const dir = await SelectDirectory(); if (dir) settingsForm.modsRepo = dir } catch (_) {} }

  /** 安装 Mod：登记为已安装并保存对应的衣服名称（占用的装备资源），不创建符号链接 */
  async function installModRecord(mod, parts) {
    const cleaned = {}
    for (const [slot, vals] of Object.entries(parts || {})) {
      const list = Array.isArray(vals) ? vals : (vals ? [vals] : [])
      const ok = list.filter(Boolean)
      if (ok.length) cleaned[slot] = ok
    }
    await InstallMod(mod.name, cleaned)
    await loadMods()
    await refreshLogs()
  }

  /** 从数据文件读取 Mod 列表（不扫描磁盘） */
  async function loadMods() {
    try { const list = await GetMods(); mods.value = (list || []).map(m => ({ ...m })) }
    catch (e) { addLog(`读取 Mod 数据失败: ${e}`) }
    await loadConflicts()
  }

  /** 同步 Mod 数据：扫描磁盘校正到数据文件后返回（仅导入/刷新列表时调用） */
  async function doScanMods() {
    addLog('正在同步 Mod 数据...')
    try { const list = await ScanModsGo(); mods.value = list.map(m => ({ ...m })); await refreshLogs() }
    catch (e) { await refreshLogs() }
    await loadConflicts()
  }
  async function refreshMods() { await doScanMods() }

  /** Switch 开关：启用/禁用（创建/删除符号链接，不刷新）。
   *  启用前检查与其他已启用 Mod 的资源占用冲突，有冲突时弹窗提醒（不强制，可继续）。
   *  组合包父级关闭时，其子 Mod 一并关闭（与后端 DisableMod 行为保持一致）。 */
  async function toggleMod(mod) {
    try {
      if (mod.enabled) {
        await DisableMod(mod.name)
        mod.enabled = false
        if (mod.submods && mod.submods.length) mod.submods.forEach(s => { s.enabled = false })
      } else {
        const conflicts = await CheckModConflicts(mod.name)
        if (conflicts && conflicts.length) {
          const proceed = await confirmEnableWithConflicts(mod, conflicts)
          if (!proceed) return
        }
        await EnableMod(mod.name)
        mod.enabled = true
      }
    } catch (_) {}
    await loadConflicts()
    await refreshLogs()
  }

  /** 冲突弹窗：列出占用相同资源的其他已启用 Mod，返回用户是否仍要启用 */
  function confirmEnableWithConflicts(mod, conflicts) {
    return new Promise(resolve => {
      Modal.confirm({
        title: '检测到资源占用冲突',
        icon: () => h(WarningOutlined),
        content: () => h('div', [
          h('p', { style: 'margin-bottom:8px' }, `「${mod.nickname || mod.name}」占用的资源与以下已启用 Mod 重复，同时启用可能互相覆盖，是否仍要启用？`),
          h('ul', { style: 'margin:0;padding-left:20px' }, conflicts.map(c =>
            h('li', { style: 'line-height:1.8' }, `${c.nickname || c.modName}：${c.slot} → ${c.value}`))),
        ]),
        okText: '仍然启用',
        cancelText: '取消',
        onOk: () => resolve(true),
        onCancel: () => resolve(false),
      })
    })
  }

  /** 禁用图标（删除符号链接 + 刷新游戏窗口） */
  async function toggleModRefresh(mod) {
    try {
      await DisableModAndRefresh(mod.name)
      mod.enabled = false
      if (mod.submods && mod.submods.length) mod.submods.forEach(s => { s.enabled = false })
    } catch (_) {}
    await loadConflicts()
    await refreshLogs()
  }

  /** 组合 Mod（HDR 合集）子 Mod 开关：须先启用父组合包，才能单独启用/禁用子 Mod */
  async function toggleSubMod(mod, sub) {
    if (!sub.enabled && !mod.enabled) {
      message.warning(`请先启用组合包「${mod.nickname || mod.name}」，再单独启用子 Mod`)
      return
    }
    try {
      if (sub.enabled) {
        await DisableHdrSubMod(mod.name, sub.name)
        sub.enabled = false
      } else {
        await EnableHdrSubMod(mod.name, sub.name)
        sub.enabled = true
      }
    } catch (_) {}
    await loadConflicts()
    await refreshLogs()
  }

  /** 子 Mod 开关 + 刷新游戏窗口 */
  async function toggleSubModRefresh(mod, sub) {
    if (!sub.enabled && !mod.enabled) {
      message.warning(`请先启用组合包「${mod.nickname || mod.name}」，再单独启用子 Mod`)
      return
    }
    try {
      if (sub.enabled) {
        await DisableHdrSubModAndRefresh(mod.name, sub.name)
        sub.enabled = false
      } else {
        await EnableHdrSubModAndRefresh(mod.name, sub.name)
        sub.enabled = true
      }
    } catch (_) {}
    await loadConflicts()
    await refreshLogs()
  }

  async function refreshGame() {
    addLog('正在向游戏窗口发送 F10 刷新指令...')
    try {
      const ok = await RefreshGameMods()
      if (ok) {
        addLog('已发送 F10 → F2，正在重新读取 Mod 列表...')
        setTimeout(async () => { await loadMods() }, 2000)
      } else {
        message.warning('未找到游戏窗口，请确认游戏已启动（窗口标题需包含 Nioh2）')
        addLog('⚠ 刷新失败：未找到游戏窗口')
      }
    } catch (e) {
      addLog(`刷新游戏 Mod 失败: ${e}`)
    }
    await refreshLogs()
  }
  function saveAllSettings() {
    if (!settingsForm.modsRepo) { addLog('请先设置 Mod 托管目录'); return }
    SetModsRepo(settingsForm.modsRepo); if (settingsForm.gameRoot) SetGameRoot(settingsForm.gameRoot)
    if (settingsForm.updateUrl !== undefined) SetUpdateUrl(settingsForm.updateUrl || '')
    loadMods()
  }
  async function saveModsRepo() { await loadMods() }

  return {
    searchGame, confirmGameRoot, selectGameDir,
    confirmModsRepo, selectModsDir, doScanMods, refreshMods, loadMods,
    toggleMod, toggleModRefresh, refreshGame, toggleSubMod, toggleSubModRefresh,
    saveAllSettings, saveModsRepo,
    detectingGameDir, detectedGameRoot, detectedGameDirError, gameDirConfirmed,
    detectGameRootForConfirm, pickGameDirManual, installModRecord,
    conflicts, loadConflicts,
  }
}
