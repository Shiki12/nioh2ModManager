<template>
  <a-config-provider :theme="themeConfig">
    <a-layout class="app-layout">
      <AppSider :engine-status="engineStatus" @navigate="key => currentPage = key" @open-engine-modal="openEngineModal" />

      <a-layout class="main-layout">
        <a-layout-header class="app-header">
          <h2 class="page-title">{{ currentPageTitle }}</h2>
          <a-space :size="8">
            <a-button class="btn-cyan" @click="refreshGame"><template #icon><ReloadOutlined /></template>刷新游戏 Mod</a-button>
            <a-button type="primary" class="btn-green" @click="launchGame"><template #icon><CaretRightOutlined /></template>启动游戏</a-button>
          </a-space>
        </a-layout-header>

        <a-layout-content class="app-content">
          <ModsPage
            v-if="currentPage === 'mods'"
            :mods="mods"
            :conflicts="conflicts"
            :toggleMod="toggleMod"
            :toggleModRefresh="toggleModRefresh"
            :toggleSubMod="toggleSubMod"
            :toggleSubModRefresh="toggleSubModRefresh"
            :openEditMod="openEditMod"
            :beginImportProgress="beginImportProgress"
            :endImportProgress="endImportProgress"
            :refreshMods="refreshMods"
            :installMod="openInstallMod"
            :importMod="installMod"
            :uninstallMod="uninstallMod"
            :removeRecord="removeRecord"
          />
          <SettingsPage
            v-else-if="currentPage === 'settings'"
            :settingsForm="settingsForm"
            :selectGameDir="selectGameDir"
            :searchGame="searchGame"
            :selectModsDir="selectModsDir"
            :saveAllSettings="saveAllSettings"
            :saveModsRepo="saveModsRepo"
          />
          <AuthorTool v-else-if="currentPage === 'author'" />
          <AboutPage v-else />
        </a-layout-content>

        <LogPanel />
      </a-layout>
    </a-layout>

    <!-- 弹窗：安装 Mod 引擎 -->
    <a-modal :open="engineModalVisible" title="安装 Mod 引擎" :closable="false" :maskClosable="false"
      :okText="engineInstalling ? '安装中...' : '立即安装'" :cancelText="'稍后再说'"
      :confirm-loading="engineInstalling" @ok="installEngine" @cancel="engineModalVisible = false"
      :ok-button-props="{ class: 'btn-blue' }" :cancel-button-props="{ class: 'btn-cancel' }">
      <a-space direction="vertical" style="width:100%">
        <a-alert message="检测到游戏根目录缺少 Mod 引擎，请选择安装方式" type="warning" show-icon />

        <div class="engine-mode">
          <div class="engine-mode-title">方式一：系统安装（自动）</div>
          <div style="color:#555;font-size:13px">
            点击"立即安装"，程序将自动完成 Mod 引擎的安装：
            <b style="word-break:break-all">{{ settingsForm.gameRoot }}</b>
          </div>
        </div>

        <div class="engine-mode engine-mode-manual">
          <div class="engine-mode-title">方式二：手动安装</div>
          <div style="color:#555;font-size:13px">
            复制下方引擎包路径，自行完成 Mod 引擎的安装：
          </div>
          <a-space style="width:100%;margin-top:6px">
            <a-input :value="engineZipPath" readonly style="flex:1" />
            <a-button class="btn-gold" @click="copyEnginePath"><CopyOutlined /> 复制</a-button>
            <a-button class="btn-purple" @click="openEngineZipFolder"><FolderOpenOutlined /> 打开所在文件夹</a-button>
          </a-space>
        </div>
      </a-space>
    </a-modal>

    <!-- 弹窗：游戏目录 -->
    <a-modal :open="needSetup" title="确认游戏目录" :closable="false" :maskClosable="false" okText="确认" cancelText="手动搜索" @ok="confirmGameRoot" @cancel="searchGame"
      :ok-button-props="{ class: 'btn-blue' }" :cancel-button-props="{ class: 'btn-gold' }">
      <a-space direction="vertical" style="width:100%">
        <a-alert message="未检测到游戏目录或需要确认" type="warning" show-icon />
        <a-space style="width:100%">
          <a-input v-model:value="settingsForm.gameRoot" placeholder="如: E:\SteamLibrary\steamapps\common\Nioh2" style="flex:1" />
          <a-button class="btn-cyan" @click="selectGameDir"><FolderOpenOutlined /> 选择目录</a-button>
        </a-space>
        <span style="color:#888;font-size:12px">点击"手动搜索"自动查找，或"选择目录"手动浏览</span>
      </a-space>
    </a-modal>

    <!-- 弹窗：Mod 托管目录 -->
    <a-modal :open="needModsRepoSetup && !needSetup" title="指定 Mod 托管目录" :closable="false" :maskClosable="false"
      :okText="detectingGameDir ? '检测中...' : '确认并扫描'"
      :ok-button-props="{ disabled: detectingGameDir || !gameDirConfirmed, class: 'btn-blue' }"
      :cancel-button-props="{ class: 'btn-cancel' }"
      @ok="confirmModsRepo" @cancel="needModsRepoSetup = false">
      <a-space direction="vertical" style="width:100%">
        <a-alert message="系统将自动核对游戏目录，请确认扫描结果" type="info" show-icon />

        <a-spin :spinning="detectingGameDir" tip="正在扫描游戏目录...">
          <Transition name="fade" mode="out-in">
            <div v-if="detectingGameDir" key="loading" class="detect-panel detect-loading">
              <LoadingOutlined spin class="detect-icon" />
              <span>正在自动扫描游戏目录...</span>
            </div>
            <a-alert v-else-if="detectedGameDirError" key="error" type="warning" show-icon message="未检测到游戏目录">
              <template #description>
                <div class="detect-path">{{ detectedGameDirError }}</div>
                <a-button size="small" class="btn-orange" style="margin-top:8px" @click="pickGameDirManual"><FolderOpenOutlined /> 手动选择游戏目录</a-button>
              </template>
            </a-alert>
            <div v-else-if="detectedGameRoot" key="ok" class="detect-panel detect-ok">
              <CheckCircleOutlined class="detect-icon detect-icon-ok" />
              <div class="detect-result">
                <div class="detect-result-title">系统扫描到游戏目录</div>
                <div class="detect-path">{{ detectedGameRoot }}</div>
                <a-checkbox v-model:checked="gameDirConfirmed" style="margin-top:6px">确认使用该游戏目录</a-checkbox>
              </div>
            </div>
          </Transition>
        </a-spin>

        <a-divider style="margin:4px 0" />

        <a-alert message="请指定存放 Mod 的目录，所有 Mod 文件夹将存放在该目录下" type="info" show-icon />
        <a-space style="width:100%">
          <a-input v-model:value="settingsForm.modsRepo" placeholder="如: F:\Mod" style="flex:1" />
          <a-button class="btn-cyan" @click="selectModsDir"><FolderOpenOutlined /> 选择目录</a-button>
        </a-space>
      </a-space>
    </a-modal>

    <!-- 弹窗：安装 Mod -->
    <a-modal :open="installModVisible" title="安装 Mod" :closable="false" :maskClosable="false" okText="确认安装" cancelText="取消"
      :confirm-loading="installRecognizing" @ok="installModConfirm" @cancel="installModVisible = false"
      :ok-button-props="{ class: 'btn-green' }" :cancel-button-props="{ class: 'btn-cancel' }">
      <a-space direction="vertical" style="width:100%">
        <div v-if="installRecognizing" class="recognize-panel">
          <LoadingOutlined spin class="recognize-icon" />
          <span>正在识别该 Mod 占用的服装/武器资源…</span>
        </div>
        <a-alert v-else :message="installPrompt.text || '请填写该 Mod 对应的衣服名称（占用的装备资源），用于生成已安装卡片'" :type="installPrompt.type" show-icon />
        <div class="install-mod-name">{{ installingMod?.name }}</div>
        <div class="parts-grid">
          <div v-for="slot in allSlots" :key="slot.name" class="parts-row">
            <a-tag class="parts-tag">{{ slot.name }}</a-tag>
            <a-select
              v-if="slot.name === '武器'"
              v-model:value="installParts[slot.name]"
              :options="slotOptions(slot)"
              placeholder="选择占用的武器（可添加多个）"
              mode="multiple"
              allow-clear
              show-search
              option-filter-prop="label"
              class="parts-select"
            />
            <a-select
              v-else
              v-model:value="installParts[slot.name]"
              :options="slotOptions(slot)"
              placeholder="未占用"
              allow-clear
              show-search
              option-filter-prop="label"
              class="parts-select"
            />
          </div>
        </div>
      </a-space>
    </a-modal>

    <!-- 弹窗：Mod 导入进度 -->
    <a-modal :open="importProgressVisible" :footer="null" :closable="false" :maskClosable="false" title="正在安装 Mod">
      <a-steps :current="importStep" size="small" style="margin-bottom:16px">
        <a-step title="移动文件夹到托管目录" />
        <a-step title="登记到 Mod 数据仓库" />
        <a-step title="识别占用资源（服装/武器）" />
      </a-steps>
      <a-progress v-if="importPercent > 0" :percent="importPercent" size="small" style="margin-bottom:8px" />
      <div style="color:#666;font-size:13px">{{ importMessage }}</div>
    </a-modal>

    <!-- 弹窗：Mod 编辑 -->
    <a-modal :open="editModVisible" title="编辑 Mod" :footer="null" :width="640" @cancel="editModVisible = false">
      <a-form layout="vertical" v-if="editingMod">
        <a-form-item label="昵称">
          <a-input v-model:value="editNickname" placeholder="自定义显示名称" @pressEnter="saveModEdit" />
        </a-form-item>
        <a-form-item label="封面图片">
          <a-space direction="vertical" style="width:100%">
            <div class="edit-cover-box" @click="selectModCover" :title="editCover || '点击选择封面图片'">
              <img v-if="editCoverUrl()" :src="editCoverUrl()" alt="封面预览" @error="e => e.target.style.display = 'none'" />
              <div v-else class="edit-cover-placeholder">
                <PictureOutlined style="font-size:28px;color:#ccc" />
                <span>点击选择封面图</span>
              </div>
            </div>
            <div v-if="editCover" class="edit-cover-path">
              <span class="edit-cover-path-text" :title="editCover">{{ editCover }}</span>
              <a-button type="link" size="small" @click.stop="editCover = ''"><CloseOutlined /></a-button>
            </div>
          </a-space>
        </a-form-item>
        <a-form-item label="效果图（多张）">
          <div class="preview-list">
            <div v-for="(p, i) in editPreviews" :key="p" class="preview-item" @click="openEditPreview(i)">
              <img :src="editPreviewUrl(p)" :alt="`效果图 ${i + 1}`" @error="e => e.target.style.display = 'none'" />
              <a-tooltip title="设为封面"><StarOutlined class="preview-item-cover" :class="{ active: p === editCover }" @click.stop="setCover(p)" /></a-tooltip>
              <a-tooltip title="移除效果图"><CloseOutlined class="preview-item-remove" @click.stop="removeModPreview(p)" /></a-tooltip>
            </div>
            <div v-if="!editPreviews.length" class="preview-empty">暂无效果图，可点击下方按钮添加或自动扫描</div>
          </div>
          <a-space style="margin-top:8px">
            <a-button size="small" class="btn-blue" @click="selectModPreview"><PictureOutlined /> 添加效果图</a-button>
            <a-button size="small" @click="scanModPreviews"><ReloadOutlined /> 自动扫描</a-button>
          </a-space>
        </a-form-item>
        <template v-if="editingMod.submods && editingMod.submods.length">
          <a-divider style="margin:12px 0">子 Mod（组合包，共 {{ editingMod.submods.length }} 个）</a-divider>
          <div style="color:#888;font-size:12px;margin-bottom:10px">
            组合包由多个子 Mod 组成，每个子 Mod 各自占用一套装备资源，父级不单独占用。点击子 Mod 卡片的编辑图标可手动填写/修改其占用
          </div>
          <div class="edit-submods">
            <ModCard
              v-for="sub in editingMod.submods"
              :key="sub.name"
              :mod="subModView(sub)"
              :is-sub="true"
              :parent-mod="editingMod"
              @toggle-sub="() => toggleSubMod(editingMod, sub)"
              @toggle-sub-refresh="() => toggleSubModRefresh(editingMod, sub)"
              @edit="() => openSubEdit(sub)"
            />
          </div>
        </template>
        <template v-if="!isEditingComposite">
          <a-divider style="margin:12px 0">占用装备资源</a-divider>
          <div style="color:#888;font-size:12px;margin-bottom:10px">
            选择该 Mod 替换的游戏资源：可同时占用服装部位和武器，用于检测与其他 Mod 的冲突
          </div>
          <div class="parts-grid">
            <div v-for="slot in allSlots" :key="slot.name" class="parts-row">
              <a-tag class="parts-tag">{{ slot.name }}</a-tag>
              <a-select
                v-if="slot.name === '武器'"
                v-model:value="editParts[slot.name]"
                :options="slotOptions(slot)"
                placeholder="选择占用的武器（可添加多个）"
                mode="multiple"
                allow-clear
                show-search
                option-filter-prop="label"
                class="parts-select"
              />
              <a-select
                v-else
                v-model:value="editParts[slot.name]"
                :options="slotOptions(slot)"
                placeholder="未占用"
                allow-clear
                show-search
                option-filter-prop="label"
                class="parts-select"
              />
            </div>
          </div>
          <a-space style="margin-top:12px;display:flex;justify-content:center">
            <a-button :loading="generatingModJson" class="btn-purple" @click="generateModJson"><FileTextOutlined /> 生成 mod.json</a-button>
          </a-space>
        </template>
        <a-space style="margin-top:16px;display:flex;justify-content:center;width:100%">
          <a-button type="primary" class="btn-blue" @click="saveModEdit">保存</a-button>
          <a-button class="btn-cancel" @click="editModVisible = false">取消</a-button>
        </a-space>
      </a-form>
    </a-modal>

    <!-- 弹窗：修改子 Mod 占用 -->
    <a-modal :open="subEditVisible" :title="`修改子 Mod 占用：${editingSub?.name || ''}`" :footer="null" :width="560" @cancel="subEditVisible = false">
      <div class="sub-cover-row">
        <div class="edit-cover-box" @click="selectSubCover" :title="editSubCover || '点击选择封面图'">
          <img v-if="subCoverUrl()" :src="subCoverUrl()" alt="子 Mod 封面" @error="e => e.target.style.display = 'none'" />
          <div v-else class="edit-cover-placeholder">
            <PictureOutlined style="font-size:20px;color:#ccc" />
            <span>点选封面</span>
          </div>
        </div>
        <div class="sub-cover-tip">
          点击选择本地图片作为子 Mod 封面（会复制到子 Mod 目录）；也可在下方效果图中点星标设为封面
        </div>
      </div>
      <div style="color:#888;font-size:12px;margin-bottom:10px">
        该子 Mod 自动解析的装备资源可能不完整，可在此手动选择其占用的服装部位和武器
      </div>
      <div class="parts-grid">
        <div v-for="slot in allSlots" :key="slot.name" class="parts-row">
          <a-tag class="parts-tag">{{ slot.name }}</a-tag>
          <a-select
            v-if="slot.name === '武器'"
            v-model:value="editSubParts[slot.name]"
            :options="slotOptions(slot)"
            placeholder="选择占用的武器（可添加多个）"
            mode="multiple"
            allow-clear
            show-search
            option-filter-prop="label"
            class="parts-select"
          />
          <a-select
            v-else
            v-model:value="editSubParts[slot.name]"
            :options="slotOptions(slot)"
            placeholder="未占用"
            allow-clear
            show-search
            option-filter-prop="label"
            class="parts-select"
          />
        </div>
      </div>
      <a-divider style="margin:12px 0">子 Mod 效果图</a-divider>
      <div class="preview-list">
        <div v-for="(p, i) in editSubPreviews" :key="p" class="preview-item" @click="openSubPreview(i)">
          <img :src="subPreviewUrl(p)" :alt="`效果图 ${i + 1}`" @error="e => e.target.style.display = 'none'" />
          <a-tooltip title="设为封面"><StarOutlined class="preview-item-cover" :class="{ active: p === editSubCover }" @click.stop="setSubCover(p)" /></a-tooltip>
          <a-tooltip title="移除效果图"><CloseOutlined class="preview-item-remove" @click.stop="removeSubPreview(p)" /></a-tooltip>
        </div>
        <div v-if="!editSubPreviews.length" class="preview-empty">暂无效果图，可点击下方按钮添加或自动扫描</div>
      </div>
      <a-space style="margin-top:8px">
        <a-button size="small" class="btn-blue" @click="addSubPreview"><PictureOutlined /> 添加效果图</a-button>
        <a-button size="small" @click="scanSubPreviews"><ReloadOutlined /> 自动扫描</a-button>
      </a-space>
      <a-space style="margin-top:16px;display:flex;justify-content:center;width:100%">
        <a-button type="primary" class="btn-blue" :loading="savingSubParts" @click="saveSubParts">保存占用</a-button>
        <a-button class="btn-cancel" @click="subEditVisible = false">取消</a-button>
      </a-space>
    </a-modal>

    <!-- 弹窗：效果图放大预览（编辑弹窗 / 子 Mod 弹窗共用） -->
    <a-modal v-model:open="previewModalVisible" :footer="null" :title="previewModalTitle" width="min(720px, 92vw)" centered>
      <div class="preview-wrap">
        <img v-if="previewModalSrc" :src="previewModalSrc" :alt="previewModalTitle" class="preview-img" @error="e => e.target.style.display = 'none'" />
        <a-button v-if="previewModalImages.length > 1" class="preview-nav prev" shape="circle" @click="previewModalNav(-1)"><LeftOutlined /></a-button>
        <a-button v-if="previewModalImages.length > 1" class="preview-nav next" shape="circle" @click="previewModalNav(1)"><RightOutlined /></a-button>
      </div>
      <div v-if="previewModalImages.length > 1" class="preview-count">{{ previewModalIndex + 1 }} / {{ previewModalImages.length }}</div>
    </a-modal>
  </a-config-provider>
</template>

<script setup>
import { ref, computed, provide, watch, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { ReloadOutlined, DownloadOutlined, FolderOpenOutlined, FileImageOutlined, CopyOutlined, LoadingOutlined, CheckCircleOutlined, CaretRightOutlined, PictureOutlined, CloseOutlined, FileTextOutlined, LeftOutlined, RightOutlined, StarOutlined } from '@ant-design/icons-vue'

import { SelectImageFile, SetModCover, SetModNickname, CheckModEngine, InstallModEngine, GetEnginePath, OpenDirectory, GetArmorParts, GetWeaponParts, SetModParts, SetSubModParts, GenerateSubModModJson, RemoveModRecord, UninstallMod, LaunchGame, SelectDirectory, ImportMod, GetModConfig, AddModPreview, RemoveModPreview, RefreshModPreviews, GetSubModPreviews, AddSubModPreview, RemoveSubModPreview, SetSubModCover } from '../wailsjs/go/main/App'
import { EventsOn } from '../wailsjs/runtime/runtime'
import AppSider from './components/AppSider.vue'
import ModsPage from './components/ModsPage.vue'
import SettingsPage from './components/SettingsPage.vue'
import AboutPage from './components/AboutPage.vue'
import AuthorTool from './components/AuthorTool.vue'
import LogPanel from './components/LogPanel.vue'
import ModCard from './components/ModCard.vue'

import { useLogger } from './composables/useLogger.js'
import { useModData } from './composables/useModData.js'
import { useModOperations } from './composables/useModOperations.js'

// ---- 主题 ----
const themeConfig = {
  token: {
    colorPrimary: '#1a73e8', colorLink: '#1a73e8', borderRadius: 4,
    fontFamily: "'Segoe UI', 'Microsoft YaHei', 'PingFang SC', sans-serif",
    colorBgBase: '#ffffff', colorBgContainer: '#ffffff', colorBgLayout: '#ffffff',
    colorBorder: '#e0e0e0', colorBorderSecondary: '#eeeeee',
  },
}

// ---- 导航 ----
const currentPage = ref('mods')
const navItems = [
  { key: 'mods', label: 'Mod 管理' },
  { key: 'author', label: '作者工具' },
  { key: 'settings', label: '设置' },
  { key: 'about', label: '关于' },
]
const currentPageTitle = computed(() => navItems.find(i => i.key === currentPage.value)?.label ?? '')

// ---- 日志（provide 给子组件） ----
const { logs, logBody, addLog, clearLogs, refreshLogs } = useLogger()
provide('addLog', addLog)
provide('logs', logs)
provide('logBody', logBody)
provide('clearLogs', clearLogs)

// ---- 数据 ----
const { settingsForm, mods, needSetup, needModsRepoSetup } = useModData(addLog)

// ---- Mod 引擎检测 ----
const engineStatus = ref(null)
const engineModalVisible = ref(false)
const engineInstalling = ref(false)
const engineZipPath = ref('')
async function loadEngineZipPath() {
  try { engineZipPath.value = await GetEnginePath() } catch (e) { addLog(`获取引擎包路径失败: ${e}`) }
}
async function copyEnginePath() {
  try { await navigator.clipboard.writeText(engineZipPath.value); addLog('引擎包路径已复制') }
  catch (e) { addLog(`复制失败: ${e}`) }
}
async function openEngineZipFolder() {
  const idx = engineZipPath.value.lastIndexOf('\\')
  const dir = idx === -1 ? engineZipPath.value : engineZipPath.value.substring(0, idx)
  try { await OpenDirectory(dir) } catch (e) { addLog(`打开文件夹失败: ${e}`) }
}
async function checkEngine() {
  try {
    engineStatus.value = await CheckModEngine()
    if (engineStatus.value && !engineStatus.value.present) {
      await loadEngineZipPath()
      engineModalVisible.value = true
    }
  } catch (e) { addLog(`引擎检测失败: ${e}`) }
}
async function openEngineModal() {
  await loadEngineZipPath()
  engineModalVisible.value = true
}
async function installEngine() {
  engineInstalling.value = true
  try {
    await InstallModEngine()
    engineModalVisible.value = false
    await checkEngine()
  } catch (_) {
    await refreshLogs()
  } finally {
    engineInstalling.value = false
  }
}
watch(() => settingsForm.gameRoot, () => { if (settingsForm.gameRoot) checkEngine() })
checkEngine()

// ---- 操作 ----
const {
  searchGame, confirmGameRoot, selectGameDir,
  confirmModsRepo, selectModsDir, refreshMods, loadMods,
  toggleMod, toggleModRefresh, toggleSubMod, toggleSubModRefresh, refreshGame,
  saveAllSettings, saveModsRepo,
  detectingGameDir, detectedGameRoot, detectedGameDirError, gameDirConfirmed,
  detectGameRootForConfirm, pickGameDirManual, installModRecord,
  conflicts, loadConflicts,
} = useModOperations(settingsForm, mods, needSetup, needModsRepoSetup, addLog, refreshLogs)

// 启动后加载一次冲突检测结果（数据文件已在后端加载）
onMounted(() => { loadConflicts() })

// 打开 Mod 托管目录弹窗时自动核对游戏目录（带加载动画）
watch(() => needModsRepoSetup.value && !needSetup.value, (open) => { if (open) detectGameRootForConfirm() }, { immediate: true })

// ---- 启动游戏 ----
async function launchGame() {
  try {
    await LaunchGame()
    await refreshLogs()
  } catch (_) { await refreshLogs() }
}

// ---- 自动安装 Mod ----
const importProgressVisible = ref(false)
const importStep = ref(0)
const importPercent = ref(0)
const importMessage = ref('')

// 监听后端导入进度事件
EventsOn('modInstallProgress', (p) => {
  if (!p) return
  importMessage.value = p.message || ''
  importPercent.value = p.percent || 0
  importStep.value = Math.max(0, Math.min((p.step || 0) - 1, 2))
})

// 供子组件（HDR 一键安装等）打开/关闭"正在安装 Mod"进度弹窗
function beginImportProgress() {
  importProgressVisible.value = true
  importStep.value = 0
  importPercent.value = 0
  importMessage.value = '正在识别并准备安装…'
}
function endImportProgress() {
  importProgressVisible.value = false
}

async function installMod() {
  let folder
  try { folder = await SelectDirectory() } catch (e) { addLog(`选择目录失败: ${e}`); return }
  if (!folder) return
  let res
  importProgressVisible.value = true
  importStep.value = 0
  importPercent.value = 0
  importMessage.value = '正在准备导入…'
  try { res = await ImportMod(folder) } catch (e) {
    importProgressVisible.value = false
    addLog(`导入失败: ${e}`)
    return
  }
  importProgressVisible.value = false
  // 导入已登记进数据文件，直接读取展示
  await loadMods()
  const hasParts = res.configFound && Object.keys(res.parts || {}).length > 0
  if (hasParts) {
    // 有 mod.json 且能解析出有效部位 → 自动补全安装
    try {
      await installModRecord({ name: res.name }, res.parts)
      if (res.nickname) await SetModNickname(res.name, res.nickname)
      message.success(`已检测到 mod.json，已自动补全占用装备资源并完成安装`)
      addLog(`已自动安装（读取 mod.json）: ${res.name}`)
    } catch (e) { addLog(`自动安装失败: ${e}`) }
  } else {
    // 没有 mod.json 或解析不到有效部位 → 打开弹窗手动输入（已解析到的部位预填）
    addLog(res.configFound ? 'mod.json 无有效部位数据，请手动补充' : '未检测到 mod.json，请手动填写占用装备资源')
    openInstallMod({ name: res.name, parts: res.parts || {}, nickname: res.nickname || '' })
  }
}

// ---- Mod 安装弹窗 ----
const installModVisible = ref(false)
const installingMod = ref(null)
const installParts = ref({})
const installCover = ref('')
const pendingNickname = ref('')
const installPrompt = ref({ type: 'info', text: '' })
const installRecognizing = ref(false)

/** 打开安装弹窗：尝试读取 mod.json，有则自动补全占用装备资源并提示，无则提示手动填写 */
async function openInstallMod(mod) {
  installingMod.value = mod
  installParts.value = normalizeParts(mod.parts)
  installCover.value = mod.cover || ''
  pendingNickname.value = mod.nickname || ''
  installPrompt.value = { type: 'info', text: '' }
  installModVisible.value = true
  // 后端识别期间显示加载态（按钮禁用 + 加载提示），避免无反应
  installRecognizing.value = true
  try {
    const cfg = await GetModConfig(mod.name)
    if (cfg && cfg.configFound) {
      if (cfg.nickname && !pendingNickname.value) pendingNickname.value = cfg.nickname
      if (cfg.cover && !installCover.value) installCover.value = cfg.cover
      installParts.value = normalizeParts({ ...(cfg.parts || {}), ...(mod.parts || {}) })
      installPrompt.value = Object.keys(cfg.parts || {}).length > 0
        ? { type: 'success', text: '已检测到 mod.json，以下占用装备资源已自动补全，可确认或修改' }
        : { type: 'warning', text: '已检测到 mod.json，但未解析到有效部位数据，请手动填写' }
    } else {
      installPrompt.value = { type: 'info', text: '未检测到 mod.json，请手动填写该 Mod 占用的装备资源' }
    }
  } catch (_) {
    installPrompt.value = { type: 'info', text: '未检测到 mod.json，请手动填写该 Mod 占用的装备资源' }
  } finally {
    installRecognizing.value = false
  }
}
async function installModConfirm() {
  if (!installingMod.value) return
  try {
    // installModRecord 内部已读取数据文件刷新列表
    await installModRecord(installingMod.value, cleanParts(installParts.value))
    if (installCover.value && !installingMod.value.cover) {
      const saved = await SetModCover(installingMod.value.name, installCover.value)
      installingMod.value.cover = saved
      installCover.value = saved
    }
    if (pendingNickname.value) await SetModNickname(installingMod.value.name, pendingNickname.value)
    installModVisible.value = false
  } catch (e) { addLog(`安装失败: ${e}`) }
}

// ---- Mod 卸载（回到未安装，保留文件夹与记录） ----
async function uninstallMod(mod) {
  try {
    await UninstallMod(mod.name)
    await loadMods()
    await refreshLogs()
  } catch (e) { addLog(`卸载失败: ${e}`) }
}

// ---- Mod 记录清理（磁盘文件已缺失时清除记录） ----
async function removeRecord(mod) {
  try {
    await RemoveModRecord(mod.name)
    const idx = mods.value.findIndex(m => m.name === mod.name)
    if (idx !== -1) mods.value.splice(idx, 1)
    await refreshLogs()
  } catch (e) { addLog(`清除记录失败: ${e}`) }
}

// ---- Mod 编辑弹窗 ----
const editModVisible = ref(false)
const editingMod = ref(null)
const editNickname = ref('')
const editCover = ref('')
const editPreviews = ref([])
const armorSlots = ref([])
const weaponSlots = ref([])
const editParts = ref({})

// 全部可选槽位：服装部位 + 武器（可同时占用）
const allSlots = computed(() => [...armorSlots.value, ...weaponSlots.value])

/** 根据占用的资源推导分类：mixed=服装+武器 / weapon=仅武器 / armor=仅服装 */
function deriveCategory(parts) {
  let armor = false, weapon = false
  for (const k of Object.keys(parts || {})) { if (k === '武器') weapon = true; else armor = true }
  if (weapon && armor) return 'mixed'
  if (weapon) return 'weapon'
  return 'armor'
}

async function loadArmorSlots() {
  try {
    const slots = await GetArmorParts()
    armorSlots.value = slots || []
  } catch (e) { addLog(`加载装备资源失败: ${e}`) }
}
loadArmorSlots()

async function loadWeaponSlots() {
  try {
    const parts = await GetWeaponParts()
    // 武器无部位分组，统一归入"武器"槽位
    weaponSlots.value = [{ name: '武器', parts: parts || [] }]
  } catch (e) { addLog(`加载武器资源失败: ${e}`) }
}
loadWeaponSlots()

/** 把后端多值占用归一化为编辑弹窗表单：服装部位取单个值，武器部位为数组（可多选） */
function normalizeParts(parts) {
  const out = {}
  for (const [slot, vals] of Object.entries(parts || {})) {
    const list = Array.isArray(vals) ? vals : (vals ? [vals] : [])
    const clean = list.map(v => (typeof v === 'string' ? v.trim() : '')).filter(Boolean)
    if (!clean.length) continue
    if (slot === '武器') out[slot] = clean
    else out[slot] = clean[0]
  }
  return out
}

/** 过滤占用资源，仅保留有效槽位（服装部位 + 武器），输出为 部位 -> 数组（后端格式）。
 *  武器部位可含多个值，服装部位每个一个值。 */
function cleanParts(parts) {
  const keys = new Set(allSlots.value.map(s => s.name))
  const cleaned = {}
  for (const [slot, v] of Object.entries(parts || {})) {
    if (!keys.has(slot)) continue
    if (slot === '武器') {
      const list = (Array.isArray(v) ? v : [v]).map(x => (typeof x === 'string' ? x.trim() : '')).filter(Boolean)
      if (list.length) cleaned[slot] = list
    } else {
      const s = (typeof v === 'string' ? v : Array.isArray(v) ? v[0] : '').trim()
      if (s) cleaned[slot] = [s]
    }
  }
  return cleaned
}

// 编辑弹窗封面缩略图：相对文件名走 /modfile（随 mod 文件夹），绝对路径走 /localfile
function editCoverUrl() {
  const cover = editCover.value
  if (!cover) return ''
  if (/^[a-zA-Z]:[\\/]/.test(cover) || cover.startsWith('\\\\')) {
    return '/localfile?file=' + encodeURIComponent(cover)
  }
  return '/modfile?mod=' + encodeURIComponent(editingMod.value?.name || '') + '&file=' + encodeURIComponent(cover)
}

function slotOptions(slot) {
  const seen = new Set()
  const opts = []
  for (const p of slot.parts || []) {
    if (!p.name || seen.has(p.name)) continue
    seen.add(p.name)
    opts.push({ value: p.name, label: p.name })
  }
  return opts
}

function openEditMod(mod) {
  editingMod.value = mod
  editNickname.value = mod.nickname || ''
  editCover.value = mod.cover || ''
  editPreviews.value = (Array.isArray(mod.previews) ? mod.previews : []).slice()
  editParts.value = normalizeParts(mod.parts)
  subEditVisible.value = false
  editModVisible.value = true
}
// 组合包父级不占用：隐藏"占用装备资源"选择器
const isEditingComposite = computed(() => !!(editingMod.value?.submods && editingMod.value.submods.length))
// 编辑弹窗内子 Mod 卡片的展示视图（与外部 ModCard 结构一致）
function subModView(sub) {
  return {
    name: sub.name,
    nickname: sub.name,
    parts: sub.parts || {},
    enabled: sub.enabled,
    cover: sub.cover || '',
    previews: sub.previews || [],
    category: editingMod.value?.category || 'armor',
    submods: [],
  }
}
// 手动填写/修改子 Mod 占用：打开子 Mod 占用编辑弹窗
const editingSub = ref(null)
const subEditVisible = ref(false)
const editSubParts = ref({})
const editSubPreviews = ref([])
const editSubCover = ref('')
const savingSubParts = ref(false)
function openSubEdit(sub) {
  editingSub.value = sub
  editSubParts.value = normalizeParts(sub.parts)
  editSubPreviews.value = (Array.isArray(sub.previews) ? sub.previews : []).slice()
  editSubCover.value = sub.cover || ''
  subEditVisible.value = true
}
async function saveSubParts() {
  const mod = editingMod.value
  const sub = editingSub.value
  if (!mod || !sub) return
  savingSubParts.value = true
  try {
    const cleaned = cleanParts(editSubParts.value)
    await SetSubModParts(mod.name, sub.name, cleaned)
    sub.parts = cleaned
    const union = {}
    for (const sm of mod.submods) for (const [k, v] of Object.entries(sm.parts || {})) {
      const list = Array.isArray(v) ? v : (v ? [v] : [])
      union[k] = (union[k] || []).concat(list.filter(Boolean))
    }
    mod.parts = union
    mod.category = deriveCategory(union)
    subEditVisible.value = false
    message.success(`已更新子 Mod「${sub.name}」占用资源`)
    await refreshLogs()
  } catch (e) {
    message.error(String(e))
  } finally {
    savingSubParts.value = false
  }
}
// 手动生成/刷新合集 mod.json：将当前手动填写的全部子 Mod 占用写入合集目录
const generatingModJson = ref(false)
async function generateModJson() {
  const mod = editingMod.value
  if (!mod) return
  generatingModJson.value = true
  try {
    await GenerateSubModModJson(mod.name)
    message.success('已生成/更新 mod.json')
    await refreshLogs()
  } catch (e) {
    message.error(String(e))
  } finally {
    generatingModJson.value = false
  }
}
async function selectModCover() { try { const file = await SelectImageFile(); if (file) editCover.value = file } catch (_) {} }
// 将某张效果图设为 Mod 封面
async function setCover(p) {
  const mod = editingMod.value
  if (!mod) return
  try {
    const saved = await SetModCover(mod.name, p)
    editCover.value = saved || p
    mod.cover = saved || p
    message.success('已设为封面')
  } catch (e) { message.error(String(e)) }
}
// 子 Mod 封面缩略图：相对父 Mod 目录路径 → /modfile（owner 为父 Mod）
function subCoverUrl() {
  const cover = editSubCover.value
  if (!cover) return ''
  if (/^[a-zA-Z]:[\\/]/.test(cover) || cover.startsWith('\\\\')) return '/localfile?file=' + encodeURIComponent(cover)
  return '/modfile?mod=' + encodeURIComponent(editingMod.value?.name || '') + '&file=' + encodeURIComponent(cover)
}
// 选择本地图片作为子 Mod 封面：复制进子 Mod 目录后设为封面
async function selectSubCover() {
  const mod = editingMod.value, sub = editingSub.value
  if (!mod || !sub) return
  try {
    const file = await SelectImageFile()
    if (!file) return
    const rel = await AddSubModPreview(mod.name, sub.name, file)
    await SetSubModCover(mod.name, sub.name, rel)
    editSubCover.value = rel
    sub.cover = rel
    if (!editSubPreviews.value.includes(rel)) {
      editSubPreviews.value.push(rel)
      sub.previews = editSubPreviews.value.slice()
    }
    message.success('已设置子 Mod 封面')
  } catch (e) { message.error(String(e)) }
}
// 将某张效果图设为子 Mod 封面
async function setSubCover(p) {
  const mod = editingMod.value, sub = editingSub.value
  if (!mod || !sub) return
  try {
    await SetSubModCover(mod.name, sub.name, p)
    editSubCover.value = p
    sub.cover = p
    message.success('已设为封面')
  } catch (e) { message.error(String(e)) }
}
// 效果图：相对文件名走 /modfile（随 Mod 文件夹），绝对路径走 /localfile
function editPreviewUrl(p) {
  if (!p) return ''
  if (/^[a-zA-Z]:[\\/]/.test(p) || p.startsWith('\\\\')) return '/localfile?file=' + encodeURIComponent(p)
  return '/modfile?mod=' + encodeURIComponent(editingMod.value?.name || '') + '&file=' + encodeURIComponent(p)
}
function subPreviewUrl(p) {
  if (!p) return ''
  if (/^[a-zA-Z]:[\\/]/.test(p) || p.startsWith('\\\\')) return '/localfile?file=' + encodeURIComponent(p)
  return '/modfile?mod=' + encodeURIComponent(editingMod.value?.name || '') + '&file=' + encodeURIComponent(p)
}
async function selectModPreview() {
  const mod = editingMod.value
  if (!mod) return
  try {
    const file = await SelectImageFile()
    if (!file) return
    const saved = await AddModPreview(mod.name, file)
    if (saved && !editPreviews.value.includes(saved)) {
      editPreviews.value.push(saved)
      mod.previews = editPreviews.value.slice()
    }
    message.success('已添加效果图')
  } catch (e) { message.error(String(e)) }
}
async function removeModPreview(file) {
  const mod = editingMod.value
  if (!mod) return
  try {
    await RemoveModPreview(mod.name, file)
    editPreviews.value = editPreviews.value.filter(p => p !== file)
    mod.previews = editPreviews.value.slice()
  } catch (e) { message.error(String(e)) }
}
async function scanModPreviews() {
  const mod = editingMod.value
  if (!mod) return
  try {
    const list = await RefreshModPreviews(mod.name)
    editPreviews.value = (Array.isArray(list) ? list : []).slice()
    mod.previews = editPreviews.value.slice()
    message.success('已自动扫描效果图')
  } catch (e) { message.error(String(e)) }
}
async function addSubPreview() {
  const mod = editingMod.value, sub = editingSub.value
  if (!mod || !sub) return
  try {
    const file = await SelectImageFile()
    if (!file) return
    const saved = await AddSubModPreview(mod.name, sub.name, file)
    if (saved && !editSubPreviews.value.includes(saved)) {
      editSubPreviews.value.push(saved)
      sub.previews = editSubPreviews.value.slice()
    }
    message.success('已添加子 Mod 效果图')
  } catch (e) { message.error(String(e)) }
}
async function removeSubPreview(file) {
  const mod = editingMod.value, sub = editingSub.value
  if (!mod || !sub) return
  try {
    await RemoveSubModPreview(mod.name, sub.name, file)
    editSubPreviews.value = editSubPreviews.value.filter(p => p !== file)
    sub.previews = editSubPreviews.value.slice()
  } catch (e) { message.error(String(e)) }
}
async function scanSubPreviews() {
  const mod = editingMod.value, sub = editingSub.value
  if (!mod || !sub) return
  try {
    await RefreshModPreviews(mod.name)
    const subs = await GetSubModPreviews(mod.name, sub.name)
    editSubPreviews.value = (Array.isArray(subs) ? subs : []).slice()
    sub.previews = editSubPreviews.value.slice()
    message.success('已自动扫描子 Mod 效果图')
  } catch (e) { message.error(String(e)) }
}
// 效果图放大预览（编辑弹窗 / 子 Mod 弹窗共用）
const previewModalVisible = ref(false)
const previewModalImages = ref([])
const previewModalIndex = ref(0)
const previewModalTitle = ref('效果图预览')
const previewModalSrc = computed(() => previewModalImages.value[previewModalIndex.value] || '')
function openEditPreview(i) {
  previewModalImages.value = editPreviews.value.map(p => editPreviewUrl(p)).filter(Boolean)
  previewModalIndex.value = Math.min(i, previewModalImages.value.length - 1)
  previewModalTitle.value = `${editingMod.value?.nickname || editingMod.value?.name || ''} 效果图`
  previewModalVisible.value = true
}
function openSubPreview(i) {
  previewModalImages.value = editSubPreviews.value.map(p => subPreviewUrl(p)).filter(Boolean)
  previewModalIndex.value = Math.min(i, previewModalImages.value.length - 1)
  previewModalTitle.value = `${editingSub.value?.name || ''} 效果图`
  previewModalVisible.value = true
}
function previewModalNav(d) {
  const n = previewModalImages.value.length
  if (n) previewModalIndex.value = (previewModalIndex.value + d + n) % n
}
async function saveModEdit() {
  if (!editingMod.value) return
  const mod = editingMod.value
  if (editNickname.value !== (mod.nickname || '')) { await SetModNickname(mod.name, editNickname.value || mod.name); mod.nickname = editNickname.value }
  if (editCover.value !== (mod.cover || '')) {
    const saved = await SetModCover(mod.name, editCover.value)
    mod.cover = saved
    editCover.value = saved
  }
  const cleaned = cleanParts(editParts.value)
  const prev = mod.parts || {}
  const changed = JSON.stringify(cleaned) !== JSON.stringify(prev)
  if (changed) {
    await SetModParts(mod.name, cleaned)
    mod.parts = cleaned
    mod.category = deriveCategory(cleaned)
  }
  editModVisible.value = false
  await refreshLogs()
}
</script>

<style>html, body { background: #fff; margin:0; padding:0; }</style>

<style scoped>
.app-layout { height: 100vh; overflow: hidden; font-family: 'Segoe UI','Microsoft YaHei','PingFang SC',sans-serif; }
.main-layout { background: #fff !important; }
.app-header { display: flex; align-items: center; justify-content: space-between; height: 48px !important; line-height: 48px !important; padding: 0 24px !important; background: #ffffff !important; border-bottom: 1px solid #e8e8e8; }
.page-title { margin: 0; font-size: 16px; font-weight: 600; color: #333; }
.app-content { flex: 1; overflow-y: auto; background: #fafafa; }
.app-content > :deep(.page) { padding: 20px 24px; animation: fadeIn .2s ease; }
.engine-mode { width: 100%; padding: 10px 12px; border: 1px solid #e8e8e8; border-radius: 4px; background: #fafafa; }
.engine-mode-title { font-size: 13px; font-weight: 600; color: #333; margin-bottom: 4px; }
.engine-mode-manual { border-style: dashed; }
.parts-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 8px 12px; }
.edit-submods { display: grid; grid-template-columns: repeat(auto-fill, minmax(180px, 1fr)); gap: 12px; }
.edit-cover-box { width: 80px; height: 80px; border: 1px dashed #d9d9d9; border-radius: 6px; display: flex; align-items: center; justify-content: center; overflow: hidden; cursor: pointer; background: #fafafa; }
.edit-cover-box:hover { border-color: #1a73e8; }
.edit-cover-box img { width: 100%; height: 100%; object-fit: cover; }
.edit-cover-placeholder { display: flex; flex-direction: column; align-items: center; gap: 4px; color: #999; font-size: 11px; }
.edit-cover-placeholder .anticon { font-size: 20px !important; }
.edit-cover-path { display: flex; align-items: center; gap: 4px; font-size: 12px; color: #888; max-width: 100%; }
.edit-cover-path-text { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.preview-list { display: flex; flex-wrap: wrap; gap: 8px; }
.preview-item { position: relative; width: 72px; height: 72px; border-radius: 4px; overflow: hidden; border: 1px solid #e8e8e8; cursor: zoom-in; }
.preview-item:hover { border-color: #1a73e8; }
.preview-item img { width: 100%; height: 100%; object-fit: cover; display: block; }
.preview-item-cover { position: absolute; bottom: 2px; left: 2px; color: #fff; background: rgba(0,0,0,.5); border-radius: 50%; padding: 2px; font-size: 11px; cursor: pointer; }
.preview-item-cover:hover { background: #faad14; }
.preview-item-cover.active { background: #faad14; color: #fff; }
.preview-item-remove { position: absolute; top: 2px; right: 2px; color: #fff; background: rgba(0,0,0,.5); border-radius: 50%; padding: 2px; font-size: 11px; cursor: pointer; }
.preview-item-remove:hover { background: #ff4d4f; }
.preview-empty { width: 100%; padding: 10px 0; font-size: 12px; color: #999; border: 1px dashed #e8e8e8; border-radius: 4px; text-align: center; }
.sub-cover-row { display: flex; align-items: center; gap: 12px; margin-bottom: 12px; }
.sub-cover-tip { flex: 1; font-size: 12px; color: #888; line-height: 1.6; }
.preview-wrap { position: relative; }
.preview-img { width: 100%; display: block; }
.preview-nav { position: absolute; top: 50%; transform: translateY(-50%); z-index: 2; border-color: rgba(0,0,0,.2); background: rgba(0,0,0,.45); color: #fff; }
.preview-nav:hover { background: #1a73e8; color: #fff; }
.preview-nav.prev { left: 8px; }
.preview-nav.next { right: 8px; }
.preview-count { text-align: center; color: #999; font-size: 12px; margin-top: 8px; }
.category-radio { margin-bottom: 4px; }
.category-radio .ant-radio-button-wrapper { font-size: 12px; }
.parts-row { display: flex; align-items: center; gap: 10px; min-width: 0; }
.parts-tag { width: 40px; text-align: center; margin-inline-end: 0; flex-shrink: 0; }
.parts-select { flex: 1; }
.install-mod-name { font-weight: 600; color: #333; word-break: break-all; }
.detect-panel { display: flex; align-items: flex-start; gap: 10px; padding: 12px 14px; border: 1px solid #e8e8e8; border-radius: 4px; background: #fafafa; }
.recognize-panel { display: flex; align-items: center; gap: 10px; padding: 12px 14px; border: 1px solid #b7d8ff; border-radius: 4px; background: #f0f7ff; color: #1a73e8; font-size: 13px; }
.recognize-icon { font-size: 16px; }
.detect-loading { align-items: center; color: #555; }
.detect-icon { font-size: 20px; color: #1a73e8; }
.detect-icon-ok { color: #52c41a; }
.detect-result { flex: 1; }
.detect-result-title { font-weight: 600; color: #333; }
.detect-path { font-size: 13px; color: #555; word-break: break-all; margin-top: 4px; }
.detect-ok { border-color: #b7eb8f; background: #f6ffed; }
.fade-enter-active, .fade-leave-active { transition: opacity .25s ease, transform .25s ease; }
.fade-enter-from { opacity: 0; transform: translateY(6px); }
.fade-leave-to { opacity: 0; transform: translateY(-6px); }
@keyframes fadeIn { from{opacity:0;transform:translateY(4px)} to{opacity:1;transform:translateY(0)} }
</style>
