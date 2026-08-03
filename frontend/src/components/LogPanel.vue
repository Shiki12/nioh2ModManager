<template>
  <section class="log-panel" :class="{ expanded }">
    <div class="log-header" @click="expanded = !expanded">
      <span class="log-title">
        <CaretRightOutlined v-if="!expanded" class="log-caret" />
        <CaretDownOutlined v-else class="log-caret" />
        操作日志
      </span>
      <a-button size="small" type="text" class="btn-link-gray" @click.stop="clearLogs">清空</a-button>
    </div>
    <div v-show="expanded" class="log-body" ref="el">
      <div v-for="log in logs" :key="log.id" class="log-row">
        <span class="log-time">{{ log.time }}</span>
        <span class="log-msg">{{ log.message }}</span>
      </div>
    </div>
  </section>
</template>

<script setup>
import { ref, inject, watchEffect } from 'vue'
import { CaretRightOutlined, CaretDownOutlined } from '@ant-design/icons-vue'

const logs = inject('logs')
const clearLogs = inject('clearLogs')
const logBody = inject('logBody')

const expanded = ref(false)
const el = ref(null)
watchEffect(() => { logBody.value = el.value })
</script>

<style scoped>
.log-panel { flex-shrink: 0; background: #fff; border-top: 1px solid #e8e8e8; display: flex; flex-direction: column; }
.log-header { display: flex; align-items: center; justify-content: space-between; padding: 4px 20px; border-bottom: 1px solid #f0f0f0; flex-shrink: 0; font-size: 13px; color: #888; cursor: pointer; user-select: none; }
.log-header:hover { color: #1a73e8; }
.log-title { display: inline-flex; align-items: center; gap: 6px; }
.log-caret { font-size: 12px; transition: transform .2s; }
.log-body { height: 128px; overflow-y: auto; padding: 4px 20px; font-family: 'Consolas','Fira Code','Courier New',monospace; font-size: 12px; }
.log-row { display: flex; gap: 14px; padding: 2px 0; }
.log-row:hover { background: #fafafa; }
.log-time { color: #aaa; flex-shrink: 0; }
.log-msg { color: #555; }
</style>
