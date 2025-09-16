<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRoute } from 'vue-router'
import { createConnectTransport } from '@connectrpc/connect-web'
import { createClient } from '@connectrpc/connect'
import { SickRock } from '../gen/sickrock_pb'
import Table from '../components/Table.vue'
import CalendarView from '../components/CalendarView.vue'

const route = useRoute()
const props = defineProps<{ tableName?: string }>()
const tableId = computed(() => (props.tableName ?? (route.params.tableName as string)))

const transport = createConnectTransport({ baseUrl: '/api' })
const client = createClient(SickRock, transport)

const tableStructure = ref<{ view: string } | null>(null)
const loading = ref(true)

onMounted(async () => {
  try {
    const res = await client.getTableStructure({ pageId: tableId.value })
    tableStructure.value = { view: res.view || '' }
  } catch (error) {
    console.error('Failed to get table structure:', error)
    tableStructure.value = { view: '' }
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div v-if="loading">Loading...</div>
  <template v-else>
    <CalendarView v-if="tableStructure?.view === 'calendar'" :table-id="tableId" />
    <Table v-else :table-id="tableId" :show-toolbar="true" :show-pagination="true" />
  </template>
</template>
