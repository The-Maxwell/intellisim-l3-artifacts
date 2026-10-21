<template>
  <div class="min-h-screen flex items-center justify-center bg-zinc-50">
    <div v-if="error" class="text-center">
      <p class="text-red-600 mb-4">Sign-in failed: {{ error }}</p>
      <router-link to="/login" class="text-violet-600 underline">Back to login</router-link>
    </div>
    <ProgressSpinner v-else style="width: 40px; height: 40px" strokeWidth="4" />
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import ProgressSpinner from 'primevue/progressspinner'
import { useAuthStore } from '../stores/authStore'

const router = useRouter()
const auth = useAuthStore()
const error = ref<string | null>(null)

onMounted(async () => {
  try {
    const redirect = await auth.handleCallback()
    auth.loadMe() // fire-and-forget; avatar fills in when it lands
    router.replace(redirect)
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'unexpected error'
  }
})
</script>
