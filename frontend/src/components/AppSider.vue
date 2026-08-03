<template>
  <a-layout-sider v-model:collapsed="collapsed" :trigger="null" collapsible class="app-sider" width="200">
    <div class="logo">
      <span v-show="!collapsed" class="logo-title">Nioh2ModManager</span>
    </div>
    <a-menu v-model:selectedKeys="selectedKeys" mode="inline" class="app-menu" @click="onMenuClick">
      <a-menu-item key="mods"><template #icon><AppstoreOutlined /></template><span>Mod 管理</span></a-menu-item>
      <a-menu-item key="library"><template #icon><FolderOpenOutlined /></template><span>Mod 库</span></a-menu-item>
      <a-menu-item key="settings"><template #icon><SettingOutlined /></template><span>设置</span></a-menu-item>
    </a-menu>
    <div class="sider-footer">
      <a-tooltip :title="engineTip" :placement="collapsed ? 'right' : 'topLeft'">
        <div class="engine-tag" :class="[engineState, { 'engine-collapsed': collapsed }]" @click="$emit('open-engine-modal')">
          <span v-show="!collapsed" class="engine-text">{{ engineText }}</span>
        </div>
      </a-tooltip>
      <div v-show="!collapsed" class="about-info">
        <div v-for="item in aboutInfo" :key="item.label" class="about-item">
          <span class="about-label">{{ item.label }}</span>
          <span class="about-value">{{ item.value }}</span>
        </div>
        <div v-if="result && result.hasUpdate" class="update-result update-new">{{ result.message }}</div>
        <a-button v-if="result && result.hasUpdate && result.downloadUrl" size="small" type="primary" class="btn-blue update-download" @click="openDownload"><template #icon><DownloadOutlined /></template>前往下载 {{ result.latestVersion }}</a-button>
      </div>
      <div class="sider-bottom">
        <a-button type="text" size="small" class="collapse-btn" @click="collapsed = !collapsed">
          <template #icon><MenuFoldOutlined v-if="!collapsed" /><MenuUnfoldOutlined v-else /></template>
        </a-button>
      </div>
    </div>
  </a-layout-sider>

  <!-- 弹窗：启动发现新版本 -->
  <a-modal v-model:open="updateModalVisible" :title="updateModalTitle" :footer="null" centered width="440px">
    <div class="update-modal-body">
      <div class="update-modal-cur">当前版本 {{ result?.currentVersion }}，发现新版本 <b class="update-modal-new">{{ result?.latestVersion }}</b></div>
      <div v-if="result?.notes" class="update-modal-notes">{{ result.notes }}</div>
      <div class="update-modal-link">
        <a-button type="primary" class="btn-blue" @click="openDownload"><template #icon><DownloadOutlined /></template>前往下载 {{ result?.latestVersion }}</a-button>
      </div>
    </div>
  </a-modal>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { AppstoreOutlined, SettingOutlined, FolderOpenOutlined, MenuFoldOutlined, MenuUnfoldOutlined, DownloadOutlined } from '@ant-design/icons-vue'
import { GetAbout, CheckUpdate } from '../../wailsjs/go/main/App'

const props = defineProps({ engineStatus: { type: Object, default: null } })
const emit = defineEmits(['navigate', 'open-engine-modal'])
const collapsed = ref(false)
const selectedKeys = ref(['mods'])
function onMenuClick({ key }) { emit('navigate', key) }

const aboutInfo = ref([])
const checking = ref(false)
const result = ref(null)
const updateModalVisible = ref(false)
const updateModalTitle = computed(() => (result.value?.hasUpdate ? `发现新版本 ${result.value.latestVersion}` : '检查更新'))
onMounted(async () => {
  try { aboutInfo.value = await GetAbout() } catch (e) { /* 忽略 */ }
  await checkUpdate()
})
async function checkUpdate() {
  checking.value = true
  try {
    result.value = await CheckUpdate()
    if (result.value?.hasUpdate) updateModalVisible.value = true
  } catch (e) {
    message.error(String(e))
  } finally {
    checking.value = false
  }
}
function openDownload() {
  if (result.value?.downloadUrl) window.open(result.value.downloadUrl, '_blank')
}

const engineText = computed(() => {
  if (props.engineStatus === null) return '检测引擎中...'
  return props.engineStatus.present ? 'Mod 引擎已安装' : 'Mod 引擎未安装'
})
const engineState = computed(() => {
  if (props.engineStatus === null) return 'engine-loading'
  return props.engineStatus.present ? 'engine-ok' : 'engine-error'
})
const engineTip = computed(() => {
  if (props.engineStatus === null) return '正在检测 Mod 引擎...'
  return props.engineStatus.present
    ? `Mod 引擎已安装\n${props.engineStatus.gameRoot}`
    : '未检测到 Mod 引擎，请安装 Mod 引擎'
})
</script>

<style scoped>
.app-sider { background: #fff !important; border-right: 1px solid #f0f0f0; }
.app-sider :deep(.ant-layout-sider-children) { display: flex; flex-direction: column; height: 100%; }
.logo { display: flex; align-items: center; justify-content: center; margin: 16px 12px 10px; padding: 8px 10px; border-radius: 6px; background: #1a73e8; flex-shrink: 0; }
.logo-title { font-size: 14px; font-weight: 700; color: #fff; letter-spacing: .5px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.app-menu { flex: 1; background: #fff !important; border-inline-end: none !important; }
.app-menu :deep(.ant-menu-item) { border-radius: 6px; margin: 2px 0; height: 40px; line-height: 40px; font-size: 14px; }
.sider-footer { display: flex; flex-direction: column; gap: 10px; padding: 12px 14px; border-top: 1px solid #f0f0f0; flex-shrink: 0; margin-top: auto; }
.about-info { border-top: 1px dashed #f0f0f0; padding-top: 8px; display: flex; flex-direction: column; align-items: center; gap: 4px; text-align: center; }
.about-item { display: flex; align-items: center; gap: 6px; font-size: 11px; max-width: 100%; }
.about-label { color: #999; flex-shrink: 0; }
.about-value { color: #333; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.update-result { font-size: 11px; margin-top: 4px; }
.update-new { color: #fa8c16; }
.update-download { padding: 0 10px; font-size: 12px; margin-top: 4px; }
.update-modal-body { text-align: center; }
.update-modal-cur { font-size: 14px; color: #333; margin-bottom: 10px; }
.update-modal-new { color: #fa8c16; }
.update-modal-notes { font-size: 13px; color: #666; margin-bottom: 16px; white-space: pre-wrap; text-align: left; background: #fafafa; border: 1px solid #f0f0f0; border-radius: 4px; padding: 10px; }
.update-modal-link { margin-top: 8px; }
.sider-bottom { display: flex; align-items: center; justify-content: flex-end; }
.collapse-btn { color: #666; }

.engine-tag { display: flex; align-items: center; justify-content: center; gap: 6px; cursor: pointer; padding: 4px 10px; border-radius: 12px; font-size: 12px; white-space: nowrap; transition: opacity .2s; }
.engine-tag::before { content: ''; width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
.engine-tag:hover { filter: brightness(1.05); }
.engine-text { overflow: hidden; text-overflow: ellipsis; }
.engine-collapsed { justify-content: center; padding: 5px 0; }
.engine-ok { background: #f6ffed; color: #52c41a; }
.engine-ok::before { background: #52c41a; }
.engine-error { background: #fff2f0; color: #f5222d; }
.engine-error::before { background: #f5222d; }
.engine-loading { background: #fafafa; color: #999; }
.engine-loading::before { background: #d9d9d9; }
</style>
