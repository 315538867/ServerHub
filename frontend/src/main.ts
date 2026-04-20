import { createApp } from 'vue'
import { createPinia } from 'pinia'
import TDesign from 'tdesign-vue-next'
import 'tdesign-vue-next/es/style/index.css'
import '@/styles/tokens.css'
import '@/styles/global.css'
import App from './App.vue'
import router from './router'
import { registerUi } from '@/components/ui'

const app = createApp(App)

app.use(createPinia())
app.use(router)
app.use(TDesign)
registerUi(app)

app.mount('#app')
