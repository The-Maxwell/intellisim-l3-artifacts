// Copyright (c) 2026 Vladyslav Pryshchepa
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0
// Noncommercial use only. Commercial licensing: vlad.prishhepa@gmail.com

import { createApp } from 'vue'
import { createPinia } from 'pinia'
import PrimeVue from 'primevue/config'
import ToastService from 'primevue/toastservice'
import ConfirmationService from 'primevue/confirmationservice'
import IntelliSimPreset from '@/shared/theme/primevue-preset'
import router from '@/app/router'
import App from './App.vue'
import './style.css'

const app = createApp(App)

app.use(createPinia())
app.use(router)

app.use(PrimeVue, {
  theme: {
    preset: IntelliSimPreset,
  },
})
app.use(ToastService)
app.use(ConfirmationService)

app.mount('#app')
