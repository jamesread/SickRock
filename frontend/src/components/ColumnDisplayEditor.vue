<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import Section from 'picocrank/vue/components/Section.vue'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'
import Table from 'picocrank/vue/components/Table.vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { Edit03Icon, GridIcon } from '@hugeicons/core-free-icons'
import type { createApiClient } from '../stores/api'
import { useDesignMode } from '../composables/useDesignMode'

const { designModeEnabled } = useDesignMode()

type DisplayRow = {
  columnName: string
  displayLabel: string
  valuePrefix: string
  valueSuffix: string
}

const props = defineProps<{
  tableId: string
  columnNames: string[]
  client: ReturnType<typeof createApiClient>
}>()

const loading = ref(true)
const saving = ref(false)
const error = ref<string | null>(null)
const saveMessage = ref<string | null>(null)
const rows = ref<DisplayRow[]>([])

/** Arrow up/down moves to the same field in the previous/next row, like a spreadsheet. */
function moveToAdjacentRow(event: KeyboardEvent, direction: -1 | 1) {
  const input = event.target as HTMLInputElement
  const row = input.closest('tr')
  const target = direction < 0 ? row?.previousElementSibling : row?.nextElementSibling
  const next = target?.querySelector<HTMLInputElement>(`input[data-field="${input.dataset.field}"]`)
  if (!next) return
  event.preventDefault()
  next.focus()
  next.select()
}

const tableHeaders = [
  { key: 'columnName', label: 'Column', sortable: true },
  { key: 'displayLabel', label: 'Display label', sortable: false },
  { key: 'valuePrefix', label: 'Prefix', sortable: false },
  { key: 'valueSuffix', label: 'Suffix', sortable: false },
]

const tableData = computed(() => rows.value)

async function load() {
  loading.value = true
  error.value = null
  saveMessage.value = null
  try {
    const structure = await props.client.getTableStructure({ pageId: props.tableId })
    const byName = new Map((structure.fields ?? []).map(f => [f.name, f]))
    const displayByName = new Map(
      (structure.columnDisplaySettings ?? []).map(setting => [setting.columnName, setting]),
    )
    rows.value = props.columnNames.map(name => {
      const field = byName.get(name)
      const display = displayByName.get(name)
      return {
        columnName: name,
        displayLabel: display?.displayLabel ?? field?.displayLabel ?? '',
        valuePrefix: display?.valuePrefix ?? field?.valuePrefix ?? '',
        valueSuffix: display?.valueSuffix ?? field?.valueSuffix ?? '',
      }
    })
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void load()
})

async function save() {
  saving.value = true
  error.value = null
  saveMessage.value = null
  try {
    const res = await props.client.setColumnDisplaySettings({
      tableConfiguration: props.tableId,
      settings: rows.value.map(r => ({
        columnName: r.columnName,
        displayLabel: r.displayLabel.trim(),
        valuePrefix: r.valuePrefix,
        valueSuffix: r.valueSuffix,
      })),
    })
    if (!res.success) {
      throw new Error(res.message || 'Failed to save column display settings')
    }
    saveMessage.value = res.message || 'Saved'
    await load()
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <Section
    title="Column labels & value formatting"
    :icon="Edit03Icon"
    :padding="false"
  >
    <template #toolbar>
      <button
        type="button"
        class="button inline-icon good"
        :disabled="!designModeEnabled || loading || saving"
        @click="save"
      >
        <HugeiconsIcon :icon="GridIcon" width="1em" height="1em" aria-hidden="true" />
        <span>{{ saving ? 'Saving…' : 'Save display settings' }}</span>
      </button>
    </template>

    <div v-if="error" class="list-banner-pad">
      <NotificationBlock type="error" :message="error" />
    </div>
    <div v-if="saveMessage" class="list-banner-pad">
      <NotificationBlock type="success" :message="saveMessage" />
    </div>
    <p v-if="!designModeEnabled" class="list-banner-pad inline-notification note">
      Design mode is off. Turn on <strong>Design</strong> in the header to edit labels and formatting.
    </p>
    <p class="list-banner-pad subtle">
      Set friendly labels for fields. Prefix and suffix wrap the formatted value in tables and item views.
      Use <code>{column_name}</code> in prefix or suffix to insert another field from the same row (for example
      <code>{currency}</code> before an amount).
    </p>
    <div v-if="loading" class="list-banner-pad muted">Loading display settings…</div>
    <Table
      v-else
      :data="tableData"
      :headers="tableHeaders"
      row-key="columnName"
      :filterable="false"
      :column-options="false"
    >
      <template #cell-columnName="{ row }">
        <code>{{ row.columnName }}</code>
      </template>
      <template #cell-displayLabel="{ row }">
        <input v-model="row.displayLabel" type="text" class="display-input" data-field="displayLabel" @keydown.down="moveToAdjacentRow($event, 1)" @keydown.up="moveToAdjacentRow($event, -1)" :placeholder="row.columnName" :disabled="!designModeEnabled" />
      </template>
      <template #cell-valuePrefix="{ row }">
        <input v-model="row.valuePrefix" type="text" class="display-input" data-field="valuePrefix" @keydown.down="moveToAdjacentRow($event, 1)" @keydown.up="moveToAdjacentRow($event, -1)" placeholder="e.g. {currency} " :disabled="!designModeEnabled" />
      </template>
      <template #cell-valueSuffix="{ row }">
        <input v-model="row.valueSuffix" type="text" class="display-input" data-field="valueSuffix" @keydown.down="moveToAdjacentRow($event, 1)" @keydown.up="moveToAdjacentRow($event, -1)" placeholder="e.g. / {unit}" :disabled="!designModeEnabled" />
      </template>
    </Table>
  </Section>
</template>

<style scoped>
.subtle {
  color: var(--muted-text-color, #666);
  font-size: 0.95rem;
}

.display-input {
  width: 100%;
  min-width: 8rem;
  box-sizing: border-box;
}
</style>
