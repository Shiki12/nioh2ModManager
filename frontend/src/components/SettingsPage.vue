<template>
  <div class="page">
    <a-card title="应用设置" :bordered="false" class="settings-card">
      <a-form layout="vertical" :model="settingsForm">
        <a-form-item label="游戏目录">
          <a-space style="width:100%">
            <a-input v-model:value="settingsForm.gameRoot" placeholder="游戏根目录路径" style="flex:1" />
            <a-button class="btn-cyan" @click="selectGameDir">选择目录</a-button>
            <a-button class="btn-purple" @click="searchGame"><template #icon><SearchOutlined /></template>自动搜索</a-button>
          </a-space>
        </a-form-item>
        <a-form-item label="Mod 托管目录">
          <a-space style="width:100%">
            <a-input v-model:value="settingsForm.modsRepo" placeholder="Mod 托管目录路径" style="flex:1" />
            <a-button class="btn-cyan" @click="selectModsDir">选择目录</a-button>
          </a-space>
        </a-form-item>
        <a-form-item label="更新源地址">
          <a-space style="width:100%">
            <a-input v-model:value="settingsForm.updateUrl" placeholder="版本清单 URL（如 https://example.com/version.json），留空则不检查更新" style="flex:1" />
          </a-space>
          <div style="color:#999;font-size:12px;margin-top:4px">清单格式：{"version":"0.2.0","url":"下载地址","notes":"更新说明"}；后续可切换为 GitHub / Gitee Releases 接口</div>
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
import { SearchOutlined } from '@ant-design/icons-vue'
defineProps(['settingsForm', 'selectGameDir', 'searchGame', 'selectModsDir', 'saveAllSettings', 'saveModsRepo'])
</script>

<style scoped>
.settings-card { max-width: 700px; }
</style>
