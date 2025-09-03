<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { createConnectTransport } from '@connectrpc/connect-web'
import { createClient } from '@connectrpc/connect'
import { SickRock } from '../gen/sickrock_pb'
import InsertRow from '../components/InsertRow.vue'
import Section from 'picocrank/vue/components/Section.vue'

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
  <Section title = "Insert Row">
      <InsertRow :table-id="tableId" :field-defs="fieldDefs" @created="handleCreated" />
  </Section>
</template>
