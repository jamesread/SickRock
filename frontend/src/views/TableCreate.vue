<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'

import { createApiClient } from '../stores/api'
import { SickRock } from '../gen/sickrock_pb'
import Section from 'picocrank/vue/components/Section.vue'

const router = useRouter()
const name = ref('')
const loading = ref(false)
const error = ref<string | null>(null)

// Transport handled by authenticated client
const client = createApiClient()

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
  <Section title = "Create Table">
    <input v-model="name" type="text" placeholder="Table name" @keyup.enter="submit" />
    <button @click="submit" :disabled="loading || !name">Create</button>
    <div v-if="error">{{ error }}</div>
  </Section>
</template>
