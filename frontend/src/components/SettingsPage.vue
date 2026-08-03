<template>
  <div class="page">
    <a-card title="应用设置" :bordered="false" class="settings-card">
      <a-form layout="vertical" :model="settingsForm">
        <a-form-item label="游戏目录">
          <div class="input-row">
            <a-input v-model:value="settingsForm.gameRoot" placeholder="游戏根目录路径" class="big-input" />
            <a-button class="btn-cyan" @click="selectGameDir">选择目录</a-button>
            <a-button class="btn-purple" @click="searchGame"><template #icon><SearchOutlined /></template>自动搜索</a-button>
          </div>
        </a-form-item>
        <a-form-item label="Mod 托管目录">
          <div class="input-row">
            <a-input v-model:value="settingsForm.modsRepo" placeholder="Mod 托管目录路径" class="big-input" />
            <a-button class="btn-cyan" @click="selectModsDir">选择目录</a-button>
          </div>
        </a-form-item>
        <a-form-item label="更新源地址">
          <div class="input-row">
            <a-input v-model:value="settingsForm.updateUrl" placeholder="版本清单 URL（如 https://example.com/version.json），留空则不检查更新" class="big-input" />
          </div>
          <div style="color:#999;font-size:12px;margin-top:4px">清单格式：{"version":"0.2.0","url":"下载地址","notes":"更新说明"}；后续可切换为 GitHub / Gitee Releases 接口</div>
        </a-form-item>
        <a-form-item label="检查更新">
          <a-space wrap>
            <a-button class="btn-blue" :loading="checking" @click="checkUpdate"><template #icon><ReloadOutlined /></template>检查更新</a-button>
            <span v-if="result" class="update-result" :class="result.hasUpdate ? 'update-new' : 'update-ok'">{{ result.message }}</span>
            <a-button v-if="result && result.hasUpdate && result.downloadUrl" type="primary" class="btn-blue" @click="openDownload"><template #icon><DownloadOutlined /></template>前往下载 {{ result.latestVersion }}</a-button>
          </a-space>
        </a-form-item>
        <a-form-item>
          <a-space>
            <a-button type="primary" class="btn-blue" @click="saveAllSettings">保存配置</a-button>
            <a-button class="btn-gold" @click="saveModsRepo">重新扫描</a-button>
          </a-space>
        </a-form-item>
      </a-form>
    </a-card>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { message } from 'ant-design-vue'
import { SearchOutlined, ReloadOutlined, DownloadOutlined } from '@ant-design/icons-vue'
import { CheckUpdate } from '../../wailsjs/go/main/App'
defineProps(['settingsForm', 'selectGameDir', 'searchGame', 'selectModsDir', 'saveAllSettings', 'saveModsRepo'])

const checking = ref(false)
const result = ref(null)
async function checkUpdate() {
  checking.value = true
  try {
    result.value = await CheckUpdate()
  } catch (e) {
    message.error(String(e))
  } finally {
    checking.value = false
  }
}
function openDownload() {
  if (result.value?.downloadUrl) window.open(result.value.downloadUrl, '_blank')
}
</script>

<style scoped>
.settings-card { max-width: 960px; }
.input-row { display: flex; align-items: center; gap: 8px; width: 100%; }
.input-row .big-input { flex: 1; min-width: 0; }
.big-input { height: 40px; }
.big-input :deep(.ant-input) { height: 40px; font-size: 14px; }
.update-result { font-size: 13px; }
.update-ok { color: #52c41a; }
.update-new { color: #fa8c16; }
</style>
