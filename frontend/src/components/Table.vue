<script setup lang="ts">
import { onMounted, ref, computed, watch } from 'vue'
import Pagination from 'picocrank/vue/components/Pagination.vue'
import ColumnVisibilityDropdown from './ColumnVisibilityDropdown.vue'
import RowActionsDropdown from './RowActionsDropdown.vue'
import { createConnectTransport } from '@connectrpc/connect-web'
import { createClient } from '@connectrpc/connect'
import { SickRock } from '../gen/sickrock_pb'
import InsertRow from './InsertRow.vue'

const props = defineProps<{ tableId: string; fields?: Array<{ name: string; type: string }> }>()

type Item = Record<string, unknown>

const items = ref<Item[]>([])
const loading = ref(false)
const error = ref<string | null>(null)

const localFields = ref<string[]>([])
const localFieldDefs = ref<Array<{ name: string; type: string; required: boolean }>>([])
watch(
  () => props.fields,
  (f) => {
    if (f && f.length) localFields.value = f.map(x => x.name)
  },
  { immediate: true }
)
async function loadStructure() {
  const res = await client.getTableStructure({ pageId: props.tableId })
  const defs = (res.fields ?? []).map(f => ({ name: f.name, type: f.type, required: !!f.required }))
  const names = defs.map(d => d.name)
  if (names.length) {
    localFieldDefs.value = defs
    localFields.value = names
    selectedColumns.value = [...names]
  }
}
const columns = computed(() => localFields.value.length ? localFields.value : ['id', 'name', 'created_at_unix'])
const selectedColumns = ref<string[]>([])
watch(columns, (cols) => { selectedColumns.value = [...cols] }, { immediate: true })
const visibleColumns = computed(() => selectedColumns.value.filter(c => columns.value.includes(c)))

const sortBy = ref<string | null>(null)
const sortDir = ref<'asc' | 'desc'>('asc')
function toggleSort(col: string) {
  if (sortBy.value === col) {
    sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc'
  } else {
    sortBy.value = col
    sortDir.value = 'asc'
  }
}
const sortedItems = computed(() => {
  const col = sortBy.value
  if (!col) return items.value
  const dir = sortDir.value === 'asc' ? 1 : -1
  return [...items.value].sort((a, b) => {
    const av = (a as any)[col]
    const bv = (b as any)[col]
    if (av == null && bv == null) return 0
    if (av == null) return 1
    if (bv == null) return -1
    const an = typeof av === 'bigint' ? Number(av) : av
    const bn = typeof bv === 'bigint' ? Number(bv) : bv
    if (typeof an === 'number' && typeof bn === 'number') return (an - bn) * dir
    const as = String(an)
    const bs = String(bn)
    return as.localeCompare(bs) * dir
  })
})

const page = ref(1)
const pageSize = ref(10)
const total = computed(() => sortedItems.value.length)
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))
watch([sortedItems, pageSize], () => { page.value = 1 })
const pagedItems = computed(() => {
  const start = (page.value - 1) * pageSize.value
  return sortedItems.value.slice(start, start + pageSize.value)
})

const selectedKeys = ref<Set<string>>(new Set())

// Helper function to get item value for a column, handling both standard and dynamic fields
function getItemValue(item: any, column: string): any {
  // Check standard fields first
  if (column === 'id' || column === 'name' || column === 'created_at_unix') {
    return item[column]
  }
  // Check additional fields from protobuf
  if (item.additionalFields && item.additionalFields[column] !== undefined) {
    return item.additionalFields[column]
  }
  // Fallback to direct property access
  return item[column]
}

function keyOf(it: any): string {
  const k = getItemValue(it, 'id')
  return k == null ? '' : String(k)
}
function isSelected(it: any): boolean {
  const k = keyOf(it)
  return k !== '' && selectedKeys.value.has(k)
}
function toggleSelected(it: any, ev: Event) {
  const k = keyOf(it)
  if (k === '') return
  const checked = (ev.target as HTMLInputElement).checked
  if (checked) selectedKeys.value.add(k)
  else selectedKeys.value.delete(k)
}

const transport = createConnectTransport({ baseUrl: '/api' })
const client = createClient(SickRock, transport)

async function load() {
  loading.value = true
  error.value = null
  try {
    const res = await client.listItems({ pageId: props.tableId })
    items.value = Array.isArray(res.items) ? (res.items as Item[]) : []
  } catch (e) {
    error.value = String(e)
  } finally {
    loading.value = false
  }
}

onMounted(load)
onMounted(loadStructure)
</script>

<template>
  <section class="with-header-and-content">
    <div class="section-header">
      <h2 class="table-title">{{ tableId }}</h2>
      <button @click="load" :disabled="loading">Reload</button>
    </div>
    <div v-if="error" class="error">{{ error }}</div>
    <div v-else-if="loading">Loading…</div>
    <div v-else class="section-content">
      <div role="toolbar" class = "padding">
        <router-link class="button" :to="`/table/${props.tableId}/insert-row`">Insert row</router-link>
        |
        <router-link class="button" :to="`/table/${props.tableId}/add-column`">Add column</router-link>
        <ColumnVisibilityDropdown :columns="columns" v-model="selectedColumns" />
      </div>
      <table class="table">
        <thead>
          <tr>
            <th></th>
            <th v-for="col in visibleColumns" :key="col" @click="toggleSort(col)">
              {{ col }}<span v-if="sortBy === col"> {{ sortDir === 'asc' ? '▲' : '▼' }}</span>
            </th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="it in pagedItems" :key="String((it as any).id ?? Math.random())"
            :class="{ selected: isSelected(it) }">
            <td>
              <input type="checkbox" :checked="isSelected(it)" @change="(e) => toggleSelected(it, e)" />
            </td>
            <td v-for="col in visibleColumns" :key="col">
              <span v-if="col === 'created_at_unix' && getItemValue(it, col) != null">{{ new Date(Number(getItemValue(it, col)) *
                1000).toLocaleString() }}</span>
              <span v-else-if="col === 'id'">
                <router-link :to="`/table/${props.tableId}/${getItemValue(it, 'id')}`">{{ getItemValue(it, col) }}</router-link>
              </span>
              <span v-else>{{ getItemValue(it, col) }}</span>
            </td>
            <td style = "width: 5%">
              <RowActionsDropdown :table-id="props.tableId" :row-id="getItemValue(it, 'id')" @deleted="load" />
            </td>
          </tr>
          <tr v-if="items.length === 0">
            <td :colspan="visibleColumns.length || 1" class="no-items">No items</td>
          </tr>
        </tbody>
      </table>
	  <div class = "padding">
		  <Pagination :total="total" v-model:page="page" v-model:page-size="pageSize" />
	  </div>
    </div>
  </section>
</template>

<style scoped>
.table-title {
  margin: 0;
}

.error {
  color: #b00020;
}

.table {
  width: 100%;
  border-collapse: collapse;
}

.table thead th {
  text-align: left;
  border-bottom: 1px solid #ddd;
  padding: .5rem;
}

.table thead th {
  cursor: pointer;
  transition: color .15s ease-in-out;
  background-color: #fff;
}

.table thead th:hover {
  color: #0366d6;
}

.table tbody td {
  border-bottom: 1px solid #eee;
  padding: .5rem;
}

.no-items {
  padding: .75rem;
  color: #666;
}

.selected {
  background: #f0f7ff;
}

.dropdown-menu {
  position: absolute;
  z-index: 10;
  background: #fff;
  border: 1px solid #ddd;
  padding: .5rem;
  box-shadow: 0 2px 6px rgba(0, 0, 0, .08);
  min-width: 200px;
}
</style>
