<template>
  <a-layout-sider v-model:collapsed="collapsed" :trigger="null" collapsible class="app-sider" width="200">
    <div class="logo">
      <span v-show="!collapsed" class="logo-title">Nioh2ModManager</span>
    </div>
    <a-menu v-model:selectedKeys="selectedKeys" mode="inline" class="app-menu" @click="onMenuClick">
      <a-menu-item key="mods"><template #icon><AppstoreOutlined /></template><span>Mod 管理</span></a-menu-item>
      <a-menu-item key="author"><template #icon><ToolOutlined /></template><span>作者工具</span></a-menu-item>
      <a-menu-item key="settings"><template #icon><SettingOutlined /></template><span>设置</span></a-menu-item>
      <a-menu-item key="about"><template #icon><InfoCircleOutlined /></template><span>关于</span></a-menu-item>
    </a-menu>
    <div class="sider-footer">
      <a-tooltip :title="engineTip" :placement="collapsed ? 'right' : 'topLeft'">
        <div class="engine-tag" :class="[engineState, { 'engine-collapsed': collapsed }]" @click="$emit('open-engine-modal')">
          <span v-show="!collapsed" class="engine-text">{{ engineText }}</span>
        </div>
      </a-tooltip>
      <div class="sider-bottom">
        <span v-show="!collapsed" class="version-text">v0.1.0</span>
        <a-button type="text" size="small" class="collapse-btn" @click="collapsed = !collapsed">
          <template #icon><MenuFoldOutlined v-if="!collapsed" /><MenuUnfoldOutlined v-else /></template>
        </a-button>
      </div>
    </div>
  </a-layout-sider>
</template>

<script setup>
import { ref, computed } from 'vue'
import { AppstoreOutlined, SettingOutlined, InfoCircleOutlined, ToolOutlined, MenuFoldOutlined, MenuUnfoldOutlined } from '@ant-design/icons-vue'

const props = defineProps({ engineStatus: { type: Object, default: null } })
const emit = defineEmits(['navigate', 'open-engine-modal'])
const collapsed = ref(false)
const selectedKeys = ref(['mods'])
function onMenuClick({ key }) { emit('navigate', key) }

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
.app-sider { background: #1e1e2d !important; border-right: none; }
.app-sider :deep(.ant-layout-sider-children) { display: flex; flex-direction: column; height: 100%; }
.logo { display: flex; align-items: center; padding: 20px 18px 24px; flex-shrink: 0; }
.logo-title { font-size: 16px; font-weight: 600; color: #ffffff; letter-spacing: 0.5px; }
.app-menu { flex: 1; background: transparent !important; border-inline-end: none !important; padding: 0 6px; }
.app-menu :deep(.ant-menu-item) { border-radius: 4px; margin: 0 0 1px 0; height: 38px; line-height: 38px; color: #a0a0b8 !important; font-size: 14px; }
.app-menu :deep(.ant-menu-item-selected) { background: #1a73e8 !important; color: #fff !important; }
.app-menu :deep(.ant-menu-item:hover:not(.ant-menu-item-selected)) { color: #d0d0e0 !important; background: rgba(255,255,255,0.04) !important; }
.sider-footer { display: flex; flex-direction: column; gap: 8px; padding: 12px 14px; border-top: 1px solid rgba(255,255,255,0.06); flex-shrink: 0; }
.sider-bottom { display: flex; align-items: center; justify-content: space-between; }
.version-text { color: #606078; font-size: 12px; }
.collapse-btn { color: #7f86c4 !important; }

.engine-tag { display: flex; align-items: center; gap: 6px; cursor: pointer; padding: 4px 10px; border-radius: 12px; font-size: 12px; white-space: nowrap; transition: opacity .2s; }
.engine-tag::before { content: ''; width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
.engine-tag:hover { filter: brightness(1.15); }
.engine-text { overflow: hidden; text-overflow: ellipsis; }
.engine-collapsed { justify-content: center; padding: 5px 0; }
.engine-ok { background: rgba(82,196,26,.16); color: #95de64; }
.engine-ok::before { background: #52c41a; box-shadow: 0 0 6px #52c41a; }
.engine-error { background: rgba(255,77,79,.16); color: #ff7875; }
.engine-error::before { background: #ff4d4f; box-shadow: 0 0 6px #ff4d4f; }
.engine-loading { background: rgba(255,255,255,.08); color: #a0a0b8; }
.engine-loading::before { background: #a0a0b8; }
</style>
