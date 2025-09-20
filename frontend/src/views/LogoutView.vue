<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const auth = useAuthStore()

onMounted(async () => {
  try {
    await auth.logout()
  } finally {
    router.replace('/login')
  }
})
</script>

<template>
  <section>
    <h2>Logging out…</h2>
  </section>
</template>
