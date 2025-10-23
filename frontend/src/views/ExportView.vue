<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { createApiClient } from '../stores/api'
import type { GetTableStructureResponse } from '../gen/sickrock_pb'
import Section from 'picocrank/vue/components/Section.vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { ArrowLeft01Icon } from '@hugeicons/core-free-icons'

const route = useRoute()
const router = useRouter()
const tableId = computed(() => route.params.tableName as string)

const client = createApiClient()

const loading = ref(true)
const error = ref<string | null>(null)
const csvText = ref<string>('')
const items = ref<any[]>([])
const format = ref<'csv' | 'plaintext' | 'yaml'>('csv')
const includeHeaders = ref<boolean>(true)

const structure = ref<GetTableStructureResponse | null>(null)

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
    // Only include explicitly visible columns; if none defined, return none.
    return []
  }
  return current.columns
    .filter(c => c.isVisible)
    .sort((a, b) => a.columnOrder - b.columnOrder)
    .map(c => c.columnName)
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
  // Determine final columns: only include columns explicitly visible in the selected view.
  // Do not force-add id or sr_created unless they are visible in the view.
  const cols: string[] = [...visibleColumnsFromSelection.value]

  const header = cols.map(escapeCsv).join(',')
  const lines: string[] = []
  if (includeHeaders.value && cols.length > 0) {
    lines.push(header)
  }
  for (const it of items.value) {
    const row: string[] = []
    for (const col of cols) {
      let value: unknown = ''
      if (col === 'id') {
        value = (it as any).id
      } else if (col === 'sr_created') {
        const ts = (it as any).srCreated
        const num = typeof ts === 'bigint' ? Number(ts) : Number(ts)
        value = Number.isFinite(num) && num > 0 ? new Date(num * 1000).toISOString() : ''
      } else if ((it as any).additionalFields && (it as any).additionalFields[col] !== undefined) {
        value = (it as any).additionalFields[col]
      } else {
        value = (it as any)[col]
      }
      row.push(escapeCsv(value))
    }
    lines.push(row.join(','))
  }
  const csv = lines.join('\n')
  if (format.value === 'plaintext') {
    csvText.value = csv.split('\n').map(l => `- ${l}`).join('\n')
  } else if (format.value === 'yaml') {
    // Render YAML list of objects using selected columns
    const yamlLines: string[] = []
    const makeYamlValue = (v: unknown): string => {
      // Always stringify to a JSON-quoted string for safety
      return JSON.stringify(String(v ?? ''))
    }
    for (const it of items.value) {
      if (cols.length === 0) continue
      const firstCol = cols[0]
      let firstVal: unknown = ''
      if (firstCol === 'id') firstVal = (it as any).id
      else if (firstCol === 'sr_created') {
        const ts = (it as any).srCreated
        const num = typeof ts === 'bigint' ? Number(ts) : Number(ts)
        firstVal = Number.isFinite(num) && num > 0 ? new Date(num * 1000).toISOString() : ''
      } else if ((it as any).additionalFields && (it as any).additionalFields[firstCol] !== undefined) {
        firstVal = (it as any).additionalFields[firstCol]
      } else {
        firstVal = (it as any)[firstCol]
      }
      yamlLines.push(`- ${firstCol}: ${makeYamlValue(firstVal)}`)
      for (let i = 1; i < cols.length; i++) {
        const col = cols[i]
        let value: unknown = ''
        if (col === 'id') value = (it as any).id
        else if (col === 'sr_created') {
          const ts = (it as any).srCreated
          const num = typeof ts === 'bigint' ? Number(ts) : Number(ts)
          value = Number.isFinite(num) && num > 0 ? new Date(num * 1000).toISOString() : ''
        } else if ((it as any).additionalFields && (it as any).additionalFields[col] !== undefined) {
          value = (it as any).additionalFields[col]
        } else {
          value = (it as any)[col]
        }
        yamlLines.push(`  ${col}: ${makeYamlValue(value)}`)
      }
    }
    csvText.value = yamlLines.join('\n')
  } else {
    csvText.value = csv
  }
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
      const defaultView = tableViews.value.find(v => v.isDefault)
      selectedViewId.value = defaultView ? defaultView.id : (tableViews.value[0]?.id ?? -1)
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
    const list = await client.listItems({ tcName: tableId.value, where })
    items.value = Array.isArray(list.items) ? list.items : []

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
  <Section :title="`Export: ${tableId}`">
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
          class="button"
        >
          <HugeiconsIcon :icon="ArrowLeft01Icon" width="16" height="16" />
          Back to Table
        </router-link>
      </div>
    </template>
    <div class="section-content">
      <div v-if="loading">Loading…</div>
      <div v-else-if="error" class="error">{{ error }}</div>
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
.actions .button {
  padding: 0.4rem 0.75rem;
  border: 1px solid #ccc;
  border-radius: 4px;
  background: #f9f9f9;
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
.error {
  color: #b00020;
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
