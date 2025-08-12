<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { createConnectTransport } from '@connectrpc/connect-web'
import { createClient } from '@connectrpc/connect'
import { SickRock } from '../gen/sickrock_pb'

const router = useRouter()
const name = ref('')
const loading = ref(false)
const error = ref<string | null>(null)

const transport = createConnectTransport({ baseUrl: '/api' })
const client = createClient(SickRock, transport)

async function submit() {
  if (!name.value || loading.value) return
  loading.value = true
  error.value = null
  try {
    await client.getTableStructure({ pageId: name.value })
    await router.push(`/table/${encodeURIComponent(name.value)}`)
  } catch (e) {
    error.value = String(e)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <section>
    <h2>Create Table</h2>
    <input v-model="name" type="text" placeholder="Table name" @keyup.enter="submit" />
    <button @click="submit" :disabled="loading || !name">Create</button>
    <div v-if="error">{{ error }}</div>
  </section>
</template>


