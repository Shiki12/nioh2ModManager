import { createApp } from 'vue'
import Antd from 'ant-design-vue'
import App from './App.vue'
import './style.css'

const app = createApp(App)
app.use(Antd)
// 全局错误处理：渲染/组件错误默认会让界面卡死且无提示，这里统一上报避免“点击无响应”
app.config.errorHandler = (err, _instance, info) => {
  console.error('[Vue error]', info, err)
  try {
    if (window.go && window.go.main && window.go.main.App && window.go.main.App.AddLog) {
      window.go.main.App.AddLog('界面错误: ' + (err && err.message ? err.message : String(err)))
    }
  } catch (_) {}
}
app.mount('#app')
