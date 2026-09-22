<!-- Fork of picocrank Table: adds rowClickable + rowClick emit. -->
<template>
  <table class="row-hover">
    <thead>
      <th
        v-for="(header, index) in visibleHeaders"
        :key="index"
        @click="toggleSort(header)"
        :class="{ sortable: header.sortable }"
        :style="{ width: header.width || 'auto' }"
      >
        {{ header.label || header.key }}
        <span v-if="header.sortable" style="width: 1.5em; display: inline-block; text-align: center">
          <span v-if="sortBy === header.key">
            <span v-if="sortDir === 'asc'">▲</span>
            <span v-else-if="sortDir === 'desc'">▼</span>
          </span>
        </span>
      </th>
    </thead>
    <tbody>
      <tr v-if="pagedItems.length === 0">
        <td :colspan="visibleHeaders.length">No items found</td>
      </tr>
      <tr
        v-else
        v-for="(row, index) in pagedItems"
        :key="index"
        :class="{ 'pc-row-clickable': rowClickable }"
        @click="onRowClick($event, row)"
      >
        <td v-for="(header, hIdx) in visibleHeaders" :key="hIdx" :class="{ hidden: header.hidden }">
          <component
            v-if="slotFor(header.key)"
            :is="slotFor(header.key)"
            :class="{ hidden: header.hidden }"
            :row="row"
            :value="row[header.key]"
            :key="hIdx"
          />
          <span v-else>{{ row[header.key] }}</span>
        </td>
      </tr>
    </tbody>
  </table>
  <div v-if="showPagination" class="padding">
    <Pagination :total="total" v-model:page="page" v-model:page-size="pageSize" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, useSlots } from 'vue'
import Pagination from 'picocrank/vue/components/Pagination.vue'

type TableHeader = {
  key: string
  label?: string
  sortable?: boolean
  width?: string
  hidden?: boolean
}

const sortBy = ref<string | null>(null)
const sortDir = ref<'asc' | 'desc'>('asc')
const page = ref(1)
const pageSize = ref(10)

const props = withDefaults(defineProps<{
  headers?: TableHeader[]
  data?: Record<string, unknown>[]
  showPagination?: boolean
  rowClickable?: boolean
}>(), {
  headers: () => [],
  data: () => [],
  showPagination: true,
  rowClickable: false,
})

const emit = defineEmits<{ rowClick: [row: Record<string, unknown>] }>()
const slots = useSlots()

function slotFor(key: string) {
  return slots[`cell-${key}`] || slots.cell || null
}

function onRowClick(event: MouseEvent, row: Record<string, unknown>) {
  if (!props.rowClickable) return
  const target = event.target as HTMLElement | null
  if (target?.closest('a, button, input, select, textarea, label')) return
  emit('rowClick', row)
}

const items = computed(() => [...props.data])

const sortedItems = computed(() => {
  if (!sortBy.value) return [...items.value]
  const col = sortBy.value
  return [...items.value].sort((a, b) => {
    const av = a[col]
    const bv = b[col]
    if (av === bv) return 0
    if (av == null) return 1
    if (bv == null) return -1
    if (typeof av === 'string' && typeof bv === 'string') {
      return sortDir.value === 'asc' ? av.localeCompare(bv) : bv.localeCompare(av)
    }
    if (typeof av === 'number' && typeof bv === 'number') {
      return sortDir.value === 'asc' ? av - bv : bv - av
    }
    if (av < bv) return sortDir.value === 'asc' ? -1 : 1
    return sortDir.value === 'asc' ? 1 : -1
  })
})

watch([sortedItems, pageSize], () => {
  page.value = 1
})

const pagedItems = computed(() => {
  const start = (page.value - 1) * pageSize.value
  return sortedItems.value.slice(start, start + pageSize.value)
})

const total = computed(() => sortedItems.value.length)
const visibleHeaders = computed(() => props.headers.filter((h) => !h.hidden))

function toggleSort(header: TableHeader) {
  if (!header.sortable) return
  if (sortBy.value === header.key) {
    sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc'
  } else {
    sortBy.value = header.key
    sortDir.value = 'asc'
  }
}
</script>

<style scoped>
table thead th.sortable:hover {
  cursor: pointer;
  color: #0366d6;
}

td:first-child,
th:first-child {
  padding-left: 1rem;
}

tbody tr.pc-row-clickable {
  cursor: pointer;
}
</style>
