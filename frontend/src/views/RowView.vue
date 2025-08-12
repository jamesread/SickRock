<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRoute } from 'vue-router'
import { createConnectTransport } from '@connectrpc/connect-web'
import { createClient } from '@connectrpc/connect'
import { SickRock } from '../gen/sickrock_pb'

const route = useRoute()
const tableName = route.params.tableName as string
const rowId = route.params.rowId as string

const transport = createConnectTransport({ baseUrl: '/api' })
const client = createClient(SickRock, transport)

const item = ref<Record<string, unknown> | null>(null)
const loading = ref(false)
const error = ref<string | null>(null)

onMounted(async () => {
  loading.value = true
  try {
    const res = await client.listItems({ pageId: tableName })
    const found = (res.items as any[] | undefined)?.find((it) => String(it.id) === String(rowId))
    item.value = found ?? null
  } catch (e) {
    error.value = String(e)
  } finally {
    loading.value = false
  }
})

const entries = computed(() => item.value ? Object.entries(item.value) : [])
</script>

<template>
  <section class = "with-header-and-content">
    <div class = "section-header">
    <h2>Row {{ rowId }}</h2>
      </div>
      <div class = "section-content">
        <div v-if="error">{{ error }}</div>
        <div v-else-if="loading">Loading…</div>
        <table v-else>
      <tbody>
        <tr v-for="[k, v] in entries" :key="k">
          <th>{{ k }}</th>
          <td>{{ typeof v === 'bigint' ? Number(v) : v as any }}</td>
        </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>


