<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { HugeiconsIcon } from '@hugeicons/vue'
import {
  Delete02Icon,
  MoreVerticalSquare01Icon,
  ViewIcon,
} from '@hugeicons/core-free-icons'
import TableRowContextMenu from 'picocrank/vue/components/TableRowContextMenu.vue'

import { createApiClient } from '../stores/api'

const props = defineProps<{ tableId: string; rowId: string | number }>()
const emit = defineEmits<{ deleted: [] }>()

const router = useRouter()
const client = createApiClient()
const open = ref(false)
const clientX = ref(0)
const clientY = ref(0)

const menuItems = [
  { id: 'view', label: 'View', icon: ViewIcon },
  { divider: true },
  { id: 'delete', label: 'Delete', icon: Delete02Icon, danger: true },
]

function toggle(event: MouseEvent) {
  if (open.value) {
    open.value = false
    return
  }
  const button = event.currentTarget as HTMLElement
  const rect = button.getBoundingClientRect()
  clientX.value = rect.right
  clientY.value = rect.bottom
  open.value = true
}

async function onDelete() {
  const ok = window.confirm('Delete this row?')
  if (!ok) return
  await client.deleteItem({ pageId: props.tableId, id: String(props.rowId) })
  emit('deleted')
}

async function onSelect(item: { id?: string }) {
  if (item.id === 'view') {
    await router.push(`/table/${props.tableId}/${props.rowId}`)
  } else if (item.id === 'delete') {
    await onDelete()
  }
}
</script>

<template>
  <button
    type="button"
    class="neutral inline-icon"
    aria-label="Row actions"
    aria-haspopup="menu"
    :aria-expanded="open"
    data-row-click-ignore
    @click.stop="toggle"
  >
    <HugeiconsIcon
      :icon="MoreVerticalSquare01Icon"
      width="1em"
      height="1em"
      aria-hidden="true"
    />
  </button>
  <TableRowContextMenu
    v-model:open="open"
    :items="menuItems"
    :client-x="clientX"
    :client-y="clientY"
    :target-count="1"
    @select="onSelect"
  />
</template>
