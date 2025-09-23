<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRoute } from 'vue-router'

import { createApiClient } from '../stores/api'
import type { GetTableStructureResponse } from '../gen/sickrock_pb'
import TableComponent from '../components/TableComponent.vue'
import CalendarComponent from '../components/CalendarComponent.vue'

const route = useRoute()
const props = defineProps<{ tableName?: string }>()
const tableId = computed(() => (props.tableName ?? (route.params.tableName as string)))

// Transport handled by authenticated client
const client = createApiClient()

const tableStructure = ref<GetTableStructureResponse | null>(null)
const loading = ref(true)

onMounted(async () => {
  try {
    const res = await client.getTableStructure({ pageId: tableId.value })
    tableStructure.value = res
  } catch (error) {
    console.error('Failed to get table structure:', error)
    tableStructure.value = null
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div v-if="loading">Loading...</div>
  <template v-else>
    <CalendarComponent v-if="tableStructure?.view === 'calendar'" :table-id="tableId" />
    <TableComponent v-else :table-id="tableId" :table-structure="tableStructure" :show-toolbar="true" :show-view-switcher="true" :show-export="true" :show-structure="true" :show-insert="true" :show-pagination="true" :show-view-create="true" :show-view-edit="true"/>
  </template>
</template>
