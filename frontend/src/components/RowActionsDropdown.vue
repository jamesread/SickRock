<script setup lang="ts">
import { ref, onMounted } from 'vue'
import DropdownMenu from './DropdownMenu.vue'
import { createConnectTransport } from '@connectrpc/connect-web'
import { createClient } from '@connectrpc/connect'
import { SickRock } from '../gen/sickrock_pb'

const props = defineProps<{ tableId: string; rowId: string | number }>()
const emit = defineEmits<{ deleted: [] }>()

const transport = createConnectTransport({ baseUrl: '/api' })
const client = createClient(SickRock, transport)

async function onDelete() {
  const ok = window.confirm('Delete this row?')
  if (!ok) return
  await client.deleteItem({ id: String(props.rowId) })
  emit('deleted')
}

const dropdown = ref<InstanceType<typeof DropdownMenu> | null>(null);

onMounted(() => {
    dropdown.value.addRouterLink(`/table/${props.tableId}/${props.rowId}`, 'View')
    dropdown.value.addCallback('Delete', onDelete, 'bad')
})
</script>

<template>
  <DropdownMenu ref="dropdown" />
</template>


