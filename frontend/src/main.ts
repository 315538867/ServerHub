import { createApp } from 'vue'
import { createPinia } from 'pinia'
import TDesign from 'tdesign-vue-next'
import 'tdesign-vue-next/es/style/index.css'
import '@/styles/tokens.css'
import '@/styles/animations.css'
import '@/styles/global.css'
import App from './App.vue'
import router from './router'
import { registerUi } from '@/components/ui'
import { useThemeStore } from '@/stores/theme'

const app = createApp(App)
const pinia = createPinia()

app.use(pinia)
app.use(router)
app.use(TDesign)
registerUi(app)

// Initialize theme before mount to avoid flash of wrong colors
useThemeStore().init()

app.mount('#app')
