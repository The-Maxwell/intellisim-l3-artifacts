<template>
  <div class="flex h-screen overflow-hidden bg-zinc-50">
    <Sidebar />
    <main class="flex-1 overflow-hidden">
      <router-view />
    </main>
    <FloatingChat />
    <Toast position="bottom-right" />
    <ConfirmDialog />
  </div>
</template>

<script setup lang="ts">
import Toast from 'primevue/toast'
import ConfirmDialog from 'primevue/confirmdialog'
import Sidebar from './Sidebar.vue'
import FloatingChat from './FloatingChat.vue'
</script>
