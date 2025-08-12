<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { createConnectTransport } from '@connectrpc/connect-web'
import { createClient } from '@connectrpc/connect'
import { SickRock } from '../gen/sickrock_pb'
import InsertRow from '../components/InsertRow.vue'

const route = useRoute()
const router = useRouter()
const tableId = route.params.tableName as string
const fieldDefs = ref<Array<{ name: string; type: string; required: boolean }>>([])

const transport = createConnectTransport({ baseUrl: '/api' })
const client = createClient(SickRock, transport)

onMounted(async () => {
  const res = await client.getTableStructure({ pageId: tableId })
  fieldDefs.value = (res.fields ?? []).map(f => ({ name: f.name, type: f.type, required: !!f.required }))
})
function handleCreated() {
  router.push({ name: 'table', params: { tableName: tableId } })
}
</script>

<template>

  <section class = "with-header-and-content">
    <div class = "section-header">
      <h1>Insert Row</h1>
    </div>
    <div class = "section-content">
      <InsertRow :table-id="tableId" :field-defs="fieldDefs" @created="handleCreated" />
    </div>
  </section>
</template>


