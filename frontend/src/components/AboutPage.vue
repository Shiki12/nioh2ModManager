<template>
  <div class="page">
    <a-card title="关于 仁王2 Mod 管理器" :bordered="false" class="about-card">
      <a-descriptions :column="1" size="middle" bordered>
        <a-descriptions-item v-for="item in about" :key="item.label" :label="item.label">
          {{ item.value }}
        </a-descriptions-item>
      </a-descriptions>
      <div class="update-area">
        <a-button class="btn-blue" :loading="checking" @click="checkUpdate"><template #icon><ReloadOutlined /></template>检查更新</a-button>
        <span v-if="result" class="update-result" :class="result.hasUpdate ? 'update-new' : 'update-ok'">{{ result.message }}</span>
      </div>
      <a-alert v-if="result && result.hasUpdate" type="success" show-icon class="update-alert">
        <template #message>
          <div>发现新版本 {{ result.latestVersion }}（当前 {{ result.currentVersion }}）</div>
        </template>
        <template #description>
          <div v-if="result.notes" class="update-notes">{{ result.notes }}</div>
          <a-button v-if="result.downloadUrl" type="link" href="javascript:void(0)" @click="openDownload">前往下载 {{ result.latestVersion }}</a-button>
        </template>
      </a-alert>
    </a-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { ReloadOutlined } from '@ant-design/icons-vue'
import { GetAbout, CheckUpdate } from '../../wailsjs/go/main/App'

const about = ref([])
const checking = ref(false)
const result = ref(null)

onMounted(async () => {
  about.value = await GetAbout()
})

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
.about-card { max-width: 700px; }
.update-area { display: flex; align-items: center; gap: 12px; margin-top: 16px; }
.update-result { font-size: 13px; }
.update-ok { color: #52c41a; }
.update-new { color: #fa8c16; }
.update-alert { margin-top: 12px; }
.update-notes { margin-bottom: 6px; white-space: pre-wrap; }
</style>
