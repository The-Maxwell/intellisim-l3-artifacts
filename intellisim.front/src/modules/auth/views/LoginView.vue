<template>
  <div class="min-h-screen flex items-center justify-center bg-zinc-50">
    <div class="bg-white border border-zinc-200 rounded-2xl shadow-sm p-10 w-full max-w-sm text-center">
      <div class="w-14 h-14 bg-violet-500 rounded-2xl flex items-center justify-center font-bold text-xl text-white mx-auto mb-4">
        IS
      </div>
      <h1 class="text-lg font-semibold text-zinc-800 mb-1">IntelliSim</h1>
      <p class="text-sm text-zinc-500 mb-8">LLM-driven agent-based modeling</p>
      <Button label="Sign in" icon="pi pi-sign-in" class="w-full" severity="contrast" @click="signIn" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Button from 'primevue/button'
import { useAuthStore } from '../stores/authStore'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

const redirect = () => (route.query.redirect as string | undefined) || '/'

onMounted(async () => {
  await auth.init()
  if (auth.isAuthenticated) router.replace(redirect())
})

function signIn() {
  auth.login(redirect())
}
</script>
