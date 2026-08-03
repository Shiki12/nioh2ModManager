import { ref, nextTick, onMounted } from 'vue'
import { GetLogs, AddLog, ClearLogs } from '../../wailsjs/go/main/App'

function mapLogs(backendLogs) {
  return (backendLogs || []).map((l, i) => ({ id: i + 1, time: l.time, message: l.message }))
}

export function useLogger() {
  const logs = ref([])
  const logBody = ref(null)

  /** 从后端重新读取全部日志 */
  async function refreshLogs() {
    try { logs.value = mapLogs(await GetLogs()) } catch (_) {}
  }

  /** 追加一条日志：持久化到后端并从后端返回全部日志 */
  async function addLog(message) {
    try { logs.value = mapLogs(await AddLog(message)) } catch (_) {}
    nextTick(() => { if (logBody.value) logBody.value.scrollTop = logBody.value.scrollHeight })
  }

  /** 清空后端日志 */
  async function clearLogs() {
    try { await ClearLogs() } catch (_) {}
    logs.value = []
  }

  onMounted(refreshLogs)

  return { logs, logBody, addLog, clearLogs, refreshLogs }
}
