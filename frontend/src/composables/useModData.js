import { ref, reactive, onMounted } from 'vue'
import { GetConfig, GetMods } from '../../wailsjs/go/main/App'

export function useModData(addLog) {
  const settingsForm = reactive({ gameRoot: '', modsRepo: '', updateUrl: '' })
  const mods = ref([])
  const needSetup = ref(false)
  const needModsRepoSetup = ref(false)

  onMounted(async () => {
    addLog('ModMan 启动')
    try {
      const cfg = await GetConfig()
      if (cfg) {
        settingsForm.gameRoot = cfg.gameRoot || ''
        settingsForm.modsRepo = cfg.modsRepo || ''
        settingsForm.updateUrl = cfg.updateUrl || ''
        if (!cfg.gameRoot) { addLog('⚠ 未检测到游戏目录'); needSetup.value = true }
        else { addLog(`游戏目录: ${cfg.gameRoot}`) }
        if (!cfg.modsRepo) { needModsRepoSetup.value = true }
        else {
          addLog(`Mod 托管目录: ${cfg.modsRepo}`)
          const list = await GetMods()
          mods.value = (list || []).map(m => ({ ...m }))
          if (list && list.length > 0) addLog(`已加载 ${list.length} 个 Mod`)
        }
      }
    } catch (e) { addLog(`加载配置失败: ${e}`) }
  })

  return { settingsForm, mods, needSetup, needModsRepoSetup }
}
