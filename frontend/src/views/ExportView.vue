<script setup lang="ts">
import { initialExportViewId } from '../utils/exportView'
import { ref, onMounted, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { createApiClient } from '../stores/api'
import type { GetTableStructureResponse, Item } from '../gen/sickrock_pb'
import { exportCellValue, type ReferencedRowsByColumn } from '../utils/exportValues'
import Section from 'picocrank/vue/components/Section.vue'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { ArrowLeft01Icon, Download01Icon } from '@hugeicons/core-free-icons'
import { tableListLinkLabel } from '../utils/rowInsertLabel'

const route = useRoute()
const router = useRouter()
const tableId = computed(() => route.params.tableName as string)

const client = createApiClient()

const loading = ref(true)
const error = ref<string | null>(null)
const csvText = ref<string>('')
const items = ref<Item[]>([])
const referencedRows = ref<ReferencedRowsByColumn>(new Map())
const format = ref<'csv' | 'plaintext' | 'yaml'>('csv')
const includeHeaders = ref<boolean>(true)

const structure = ref<GetTableStructureResponse | null>(null)
const backToTableLabel = computed(() =>
  tableListLinkLabel(structure.value?.rowNamePlural, structure.value?.rowName),
)

// Views state
const tableViews = ref<Array<{ id: number; tableName: string; viewName: string; isDefault: boolean; columns: Array<{ columnName: string; isVisible: boolean; columnOrder: number; sortOrder: string }> }>>([])
const selectedViewId = ref<number | null>(null)
const viewOptions = computed(() => {
  const options = [...tableViews.value]
  if (options.length === 0 || !options.some(v => v.isDefault)) {
    options.unshift({
      id: -1,
      tableName: tableId.value,
      viewName: 'All Columns',
      isDefault: true,
      columns: []
    })
  }
  return options
})

const visibleColumnsFromSelection = computed<string[]>(() => {
  const current = tableViews.value.find(v => v.id === selectedViewId.value) || null
  if (!current || current.columns.length === 0) {
    return []
  }
  return current.columns
    .filter(c => c.isVisible)
    .sort((a, b) => a.columnOrder - b.columnOrder)
    .map(c => c.columnName)
})

// When no view columns are selected (e.g. "All Columns" or empty view), use table structure so export has data
const exportColumns = computed<string[]>(() => {
  const fromView = visibleColumnsFromSelection.value
  if (fromView.length > 0) return fromView
  const st = structure.value
  if (!st?.fields?.length) return ['id', 'sr_created']
  return ['id', ...st.fields.map(f => f.name), 'sr_created']
})

function escapeCsv(value: unknown): string {
  if (value === null || value === undefined) return ''
  const s = String(value)
  // Escape double quotes and wrap if needed
  const needsQuotes = /[",\n]/.test(s)
  const escaped = s.replace(/"/g, '""')
  return needsQuotes ? `"${escaped}"` : escaped
}

function computeCsv() {
  const cols: string[] = [...exportColumns.value]
  const valueOf = (it: Item, col: string) => exportCellValue(it, col, referencedRows.value)

  if (format.value === 'yaml') {
    // A YAML list of objects with the chosen columns; values are JSON-quoted strings for safety.
    const yamlLines: string[] = []
    for (const it of items.value) {
      cols.forEach((col, i) => {
        yamlLines.push(`${i === 0 ? '- ' : '  '}${col}: ${JSON.stringify(valueOf(it, col))}`)
      })
    }
    csvText.value = yamlLines.join('\n')
    return
  }

  const lines: string[] = []
  if (includeHeaders.value && cols.length > 0) {
    lines.push(cols.map(escapeCsv).join(','))
  }
  for (const it of items.value) {
    lines.push(cols.map(col => escapeCsv(valueOf(it, col))).join(','))
  }
  const csv = lines.join('\n')
  csvText.value = format.value === 'plaintext' ? csv.split('\n').map(l => `- ${l}`).join('\n') : csv
}

/**
 * Rows of the tables this table's relation columns refer to, so exports name them (as the table
 * view does) instead of writing their ids.
 */
async function loadReferencedRows(st: GetTableStructureResponse): Promise<ReferencedRowsByColumn> {
  const own = (st.foreignKeys ?? []).filter(fk => (fk.tableTcName || fk.tableName) === tableId.value)
  const entries = await Promise.all(own.map(async (fk) => {
    try {
      const res = await client.listItems({ tcName: fk.referencedTableTcName || fk.referencedTable })
      return [fk.columnName, new Map((res.items ?? []).map(row => [String(row.id), row]))] as const
    } catch {
      // Unreadable (e.g. no access): the column is exported as ids.
      return [fk.columnName, new Map<string, Item>()] as const
    }
  }))
  return new Map(entries)
}

onMounted(async () => {
  loading.value = true
  error.value = null
  try {
    // Load columns/structure
    structure.value = await client.getTableStructure({ pageId: tableId.value })

    // Load views and initialize selection
    try {
      const viewsRes = await client.getTableViews({ tableName: tableId.value })
      tableViews.value = (viewsRes.views || []).map(view => ({
        id: view.id,
        tableName: view.tableName,
        viewName: view.viewName,
        isDefault: view.isDefault,
        columns: view.columns.map(col => ({
          columnName: col.columnName,
          isVisible: col.isVisible,
          columnOrder: col.columnOrder,
          sortOrder: col.sortOrder
        }))
      }))
      // The table's default export view (table structure page), else its default view.
      selectedViewId.value = initialExportViewId(tableViews.value, structure.value?.defaultExportViewId)
    } catch (e) {
      selectedViewId.value = -1
    }

    // Parse where filters from query (expects JSON string in query.where)
    let where: Record<string, string> = {}
    const qWhere = route.query.where
    if (typeof qWhere === 'string' && qWhere.trim() !== '') {
      try {
        const parsed = JSON.parse(qWhere)
        if (parsed && typeof parsed === 'object') {
          for (const [k, v] of Object.entries(parsed)) {
            if (v != null) where[k] = String(v)
          }
        }
      } catch {
        // ignore parse error; treat as no filter
      }
    }

    // Fetch items (unpaginated) with optional filters
    const [list, referenced] = await Promise.all([
      client.listItems({ tcName: tableId.value, where }),
      loadReferencedRows(structure.value),
    ])
    items.value = Array.isArray(list.items) ? list.items : []
    referencedRows.value = referenced

    computeCsv()
  } catch (e: any) {
    error.value = String(e?.message || e)
  } finally {
    loading.value = false
  }
})

watch(selectedViewId, () => {
  computeCsv()
})

watch(format, () => {
  computeCsv()
})
watch(includeHeaders, () => {
  computeCsv()
})


async function copyToClipboard() {
  try {
    await navigator.clipboard.writeText(csvText.value)
  } catch (e) {
    // Fallback for environments without clipboard API
    const textarea = document.createElement('textarea')
    textarea.value = csvText.value
    textarea.setAttribute('readonly', '')
    textarea.style.position = 'absolute'
    textarea.style.left = '-9999px'
    document.body.appendChild(textarea)
    textarea.select()
    document.execCommand('copy')
    document.body.removeChild(textarea)
  }
}
</script>

<template>
  <Section :title="`Export: ${tableId}`" :icon="Download01Icon">
    <template #toolbar>
      <div class="actions">
        <label for="format-select">Format:</label>
        <select id="format-select" v-model="format" class="view-dropdown">
          <option value="csv">CSV</option>
          <option value="plaintext">Plaintext list</option>
          <option value="yaml">YAML</option>
        </select>
        <label for="export-view-select">View:</label>
        <select
          id="export-view-select"
          v-model="selectedViewId"
          class="view-dropdown"
        >
          <option v-for="view in viewOptions" :key="view.id" :value="view.id">
            {{ view.viewName }}
          </option>
        </select>
        <label class="plaintext-toggle">
          <input type="checkbox" v-model="includeHeaders" />
          Column headers
        </label>
        <router-link
          :to="`/table/${tableId}`"
          class="button inline-icon"
        >
          <HugeiconsIcon :icon="ArrowLeft01Icon" width="1em" height="1em" aria-hidden="true" />
          <span>{{ backToTableLabel }}</span>
        </router-link>
      </div>
    </template>
    <div class="section-content">
      <div v-if="loading">Loading…</div>
      <NotificationBlock v-else-if="error" type="error" :message="error" />
      <div v-else>
        <pre class="csv" aria-label="CSV export" role="textbox" @click="copyToClipboard" title="Click to copy">{{ csvText }}</pre>
        <div class="copy-actions">
          <button class="button" @click="copyToClipboard">Copy to clipboard</button>
        </div>
      </div>
    </div>
  </Section>

</template>

<style scoped>
.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
}
.plaintext-toggle {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  margin-right: 0.5rem;
}
.view-dropdown {
  padding: 0.4rem 0.75rem;
  border: 1px solid #ddd;
  border-radius: 4px;
  background: white;
  font-size: 1rem;
  cursor: pointer;
  min-width: 150px;
  margin-right: 0.5rem;
}
.csv {
  white-space: pre;
  overflow: auto;
  background: #f8f9fa;
  border: 1px solid #e9ecef;
  border-radius: 6px;
  padding: 0.75rem;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
}
.copy-actions {
  margin-top: 0.5rem;
  display: flex;
  justify-content: flex-end;
}
</style>
