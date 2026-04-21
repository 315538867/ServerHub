import { createApp } from 'vue'
import { createPinia } from 'pinia'
import '@/styles/tokens.css'
import '@/styles/global.css'
import '@/styles/utilities.css'
import App from './App.vue'
import router from './router'
import { registerUi } from '@/components/ui'
import { useThemeStore } from '@/stores/theme'

const app = createApp(App)
const pinia = createPinia()

app.use(pinia)
app.use(router)
registerUi(app)

useThemeStore().init()

app.mount('#app')
