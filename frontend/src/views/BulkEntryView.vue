<script setup lang="ts">
// Bulk entry: scan (or type) codes in quick succession. Each entry adds to the quantity of the row with
// the same code and match columns, or creates that row (see table_configurations.bulk_entry_json).
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import Section from 'picocrank/vue/components/Section.vue'
import FormLayout from 'picocrank/vue/components/FormLayout.vue'
import FormField from 'picocrank/vue/components/FormField.vue'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { ArrowLeft01Icon, BarCode01Icon, Camera01Icon, Undo02Icon } from '@hugeicons/core-free-icons'
import { createApiClient } from '../stores/api'
import type { BulkEntryConfig, Field } from '../gen/sickrock_pb'
import { fieldDisplayLabel } from '../utils/fieldColumnDisplay'
import { tableListLinkLabel } from '../utils/rowInsertLabel'
import { bulkEntryInputType, buildBulkEntryValues, orderBulkEntryColumns } from '../utils/bulkEntry'
import { useInsertFormViews } from '../composables/useInsertFormViews'
import { useBarcodeCamera } from '../composables/useBarcodeCamera'

type LogEntry = {
  id: string
  code: string
  label: string
  added: number
  total: number | null
  created: boolean
  undone?: boolean
}

const route = useRoute()
const tableName = route.params.tableName as string
const client = createApiClient()

const config = ref<BulkEntryConfig | null>(null)
const fieldDefs = ref<Field[]>([])
const backLabel = ref('Back')
const loaded = ref(false)

const code = ref('')
const quantity = ref(1)
const values = ref<Record<string, string>>({})
const knownItem = ref<boolean | null>(null)
const saving = ref(false)
const error = ref<string | null>(null)
const log = ref<LogEntry[]>([])

// Function refs: these elements are rendered inside the column v-for.
let codeInput: HTMLInputElement | null = null
let video: HTMLVideoElement | null = null
const setCodeInput = (el: unknown) => { codeInput = el instanceof HTMLInputElement ? el : null }
const setVideo = (el: unknown) => { video = el instanceof HTMLVideoElement ? el : null }

// Same view selection as the insert form: fields follow the chosen view's column order.
const { tableViews, selectedViewId, viewOptions, loadViews } = useInsertFormViews(tableName)

const saveOnScanKey = `sickrock.bulkEntry.saveOnScan.${tableName}`
const saveOnScan = ref(false)

const field = (name: string) => fieldDefs.value.find(f => f.name === name)
const label = (name: string) => fieldDisplayLabel(field(name), name)
const orderedColumns = computed(() => (config.value
  ? orderBulkEntryColumns(config.value, fieldDefs.value, tableViews.value, selectedViewId.value)
  : []))
const inputId = (column: string) => {
  if (column === config.value?.scanColumn) return 'bulk-code'
  if (column === config.value?.quantityColumn) return 'bulk-quantity'
  return `bulk-${column}`
}
const hasQuantity = computed(() => !!config.value?.quantityColumn)

const camera = useBarcodeCamera((scanned) => {
  code.value = scanned
  void onCodeEntered()
})

onMounted(async () => {
  try {
    const structure = await client.getTableStructure({ pageId: tableName })
    config.value = structure.bulkEntry ?? null
    fieldDefs.value = structure.fields
    backLabel.value = tableListLinkLabel(structure.rowNamePlural, structure.rowName)
    saveOnScan.value = readSaveOnScan(structure.bulkEntry?.saveOnScan ?? false)
    await loadViews()
  } catch (e) {
    error.value = String(e)
  } finally {
    loaded.value = true
  }
  await focusCode()
})

watch(saveOnScan, (value) => {
  try {
    localStorage.setItem(saveOnScanKey, value ? '1' : '0')
  } catch {
    // Remembering the choice is a convenience only.
  }
})

function readSaveOnScan(fallback: boolean): boolean {
  try {
    const stored = localStorage.getItem(saveOnScanKey)
    return stored == null ? fallback : stored === '1'
  } catch {
    return fallback
  }
}

async function focusCode() {
  await nextTick()
  codeInput?.focus()
  codeInput?.select()
}

/** The column after the code in form order (wrapping to the first), or null when there is none. */
function columnAfterCode(): string | null {
  const columns = orderedColumns.value
  const index = columns.indexOf(config.value?.scanColumn ?? '')
  const others = [...columns.slice(index + 1), ...columns.slice(0, index)]
  return others[0] ?? null
}

/** After a code is scanned or typed: fill in known fields, then save or move on to the other fields. */
async function onCodeEntered() {
  const scanned = code.value.trim()
  if (!scanned || !config.value) return
  await lookUpKnownItem(scanned)
  const next = columnAfterCode()
  if (saveOnScan.value || !next) {
    await save()
  } else {
    await nextTick()
    document.getElementById(inputId(next))?.focus()
  }
}

/** Fills the non-matching fields (e.g. title) from the newest row with this code. */
async function lookUpKnownItem(scanned: string) {
  const cfg = config.value
  if (!cfg) return
  try {
    const res = await client.listItems({ tcName: tableName, where: { [cfg.scanColumn]: scanned } })
    const newest = res.items[0]
    knownItem.value = !!newest
    for (const column of cfg.fields) {
      values.value[column] = newest?.additionalFields?.[column] ?? ''
    }
  } catch {
    knownItem.value = null
  }
}

async function save() {
  const cfg = config.value
  const scanned = code.value.trim()
  if (!cfg || !scanned || saving.value) return
  const amount = Math.trunc(Number(quantity.value))
  if (hasQuantity.value && (!Number.isFinite(amount) || amount <= 0)) {
    error.value = 'Quantity must be a whole number above zero.'
    feedback(false)
    return
  }

  saving.value = true
  error.value = null
  const { match, fields } = buildBulkEntryValues(cfg, scanned, values.value, fieldDefs.value)
  const entryLabel = fields[cfg.fields[0] ?? ''] || scanned
  try {
    if (hasQuantity.value) {
      const res = await client.addItemQuantity({
        pageId: tableName,
        match,
        quantityColumn: cfg.quantityColumn,
        quantity: BigInt(amount),
        fields,
      })
      addLog({ id: res.item?.id ?? '', code: scanned, label: entryLabel, added: amount, total: Number(res.quantity), created: res.created })
    } else {
      const res = await client.createItem({ pageId: tableName, additionalFields: { ...fields, ...match } })
      addLog({ id: res.item?.id ?? '', code: scanned, label: entryLabel, added: 1, total: null, created: true })
    }
    feedback(true)
    code.value = ''
    knownItem.value = null
    for (const column of cfg.fields) values.value[column] = ''
  } catch (e) {
    error.value = String(e)
    feedback(false)
  } finally {
    saving.value = false
  }
  await focusCode()
}

function addLog(entry: LogEntry) {
  log.value = [entry, ...log.value].slice(0, 50)
}

/** Reverses the newest entry: deletes a row it created, or takes its quantity back off. */
async function undoLast() {
  const cfg = config.value
  const entry = log.value.find(e => !e.undone)
  if (!cfg || !entry || saving.value) return
  saving.value = true
  error.value = null
  try {
    if (entry.created) {
      await client.deleteItem({ pageId: tableName, id: entry.id })
    } else {
      const res = await client.getItem({ pageId: tableName, id: entry.id })
      const fields = res.item?.additionalFields ?? {}
      const match: Record<string, string> = { [cfg.scanColumn]: entry.code }
      for (const column of cfg.matchColumns) match[column] = fields[column] ?? ''
      await client.addItemQuantity({ pageId: tableName, match, quantityColumn: cfg.quantityColumn, quantity: BigInt(-entry.added) })
    }
    entry.undone = true
  } catch (e) {
    error.value = String(e)
  } finally {
    saving.value = false
  }
  await focusCode()
}

let audio: AudioContext | null = null

/** A short tone (and vibration where supported), since people watch the item, not the screen. */
function feedback(ok: boolean) {
  try {
    navigator.vibrate?.(ok ? 40 : [80, 60, 80])
    audio ??= new AudioContext()
    const osc = audio.createOscillator()
    const gain = audio.createGain()
    osc.frequency.value = ok ? 1200 : 220
    gain.gain.value = 0.08
    osc.connect(gain).connect(audio.destination)
    osc.start()
    osc.stop(audio.currentTime + (ok ? 0.08 : 0.3))
  } catch {
    // Feedback is best effort.
  }
}

function toggleCamera() {
  if (camera.active.value) {
    camera.stop()
  } else if (video) {
    void camera.start(video)
  }
}

function logText(entry: LogEntry): string {
  if (entry.total == null) return 'created'
  return entry.created ? `created, quantity ${entry.total}` : `+${entry.added} → ${entry.total}`
}
</script>

<template>
  <Section title="Bulk entry" :icon="BarCode01Icon">
    <template #toolbar>
      <label for="bulk-view-select">View:</label>
      <select id="bulk-view-select" v-model="selectedViewId">
        <option v-for="view in viewOptions" :key="view.id" :value="view.id">
          {{ view.viewName }}
        </option>
      </select>
      <router-link :to="`/table/${tableName}`" class="button inline-icon neutral">
        <HugeiconsIcon :icon="ArrowLeft01Icon" width="1em" height="1em" aria-hidden="true" />
        <span>{{ backLabel }}</span>
      </router-link>
    </template>

    <p v-if="loaded && !config">
      Bulk entry is not set up for this table. Set <code>bulk_entry_json</code> on its table configuration,
      e.g. <code>{"scan_column":"barcode","quantity_column":"quantity","match_columns":["expirydate"],"fields":["title"]}</code>.
    </p>

    <template v-else-if="config">
      <FormLayout @submit.prevent="save">
        <template v-for="column in orderedColumns" :key="column">
          <FormField
            v-if="column === config.scanColumn"
            :label="label(column)"
            for="bulk-code"
            :description="knownItem === true ? 'Known item' : knownItem === false ? 'New item' : 'Scan or type a code, then press Enter'"
          >
            <input
              id="bulk-code"
              :ref="setCodeInput"
              v-model="code"
              type="text"
              inputmode="text"
              autocomplete="off"
              autocapitalize="off"
              spellcheck="false"
              :disabled="saving"
              @keydown.enter.prevent="onCodeEntered"
            >
            <button
              v-if="camera.supported"
              type="button"
              class="button inline-icon neutral"
              :disabled="camera.starting.value"
              :aria-pressed="camera.active.value"
              @click="toggleCamera"
            >
              <HugeiconsIcon :icon="Camera01Icon" width="1em" height="1em" aria-hidden="true" />
              <span>{{ camera.active.value ? 'Stop camera' : 'Scan with camera' }}</span>
            </button>
            <video v-show="camera.active.value" :ref="setVideo" muted playsinline style="width: 100%; max-width: 32rem" />
            <NotificationBlock v-if="camera.error.value" type="warning" :message="camera.error.value" />
          </FormField>
          <FormField v-else-if="column === config.quantityColumn" :label="label(column)" for="bulk-quantity">
            <input id="bulk-quantity" v-model.number="quantity" type="number" min="1" step="1" :disabled="saving">
          </FormField>
          <FormField
            v-else
            :label="label(column)"
            :for="`bulk-${column}`"
            :description="config.matchColumns.includes(column) ? 'Kept between scans; a different value starts a new row' : undefined"
          >
            <input
              :id="`bulk-${column}`"
              v-model="values[column]"
              :type="bulkEntryInputType(field(column)?.type)"
              autocomplete="off"
              :disabled="saving"
            >
          </FormField>
        </template>

        <FormField label="Save as soon as a code is scanned" for="bulk-save-on-scan">
          <input id="bulk-save-on-scan" v-model="saveOnScan" type="checkbox">
        </FormField>

        <div role="toolbar">
          <button type="submit" class="button good" :disabled="saving || !code.trim()">Save</button>
          <button
            type="button"
            class="button inline-icon neutral"
            :disabled="saving || !log.some(e => !e.undone)"
            @click="undoLast"
          >
            <HugeiconsIcon :icon="Undo02Icon" width="1em" height="1em" aria-hidden="true" />
            <span>Undo last</span>
          </button>
        </div>
      </FormLayout>

      <NotificationBlock v-if="error" type="error" :message="error" />

      <ul v-if="log.length">
        <li v-for="entry in log" :key="`${entry.id}-${entry.added}-${entry.total}`">
          <s v-if="entry.undone">{{ entry.label }}: {{ logText(entry) }}</s>
          <template v-else>
            <router-link v-if="entry.id" :to="`/table/${tableName}/${entry.id}`">{{ entry.label }}</router-link>
            <span v-else>{{ entry.label }}</span>: {{ logText(entry) }}
          </template>
        </li>
      </ul>
    </template>
  </Section>
</template>
