<script setup lang="ts">
// Quick add (utils/quickAdd.ts): an inline row under a related rows panel for entering child rows
// one after another, e.g. a recipe's ingredients. Enter adds the row, clears the inputs and goes
// back to the first one. Relation columns are typed by name, picked from the referenced table's
// rows, or chosen with Browse… (RowPickerDialog); with createMissing, a name that matches no row
// creates it.
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { Add01Icon, Search01Icon } from '@hugeicons/core-free-icons'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'
import { showNotificationPopup } from 'picocrank/vue/composables/useNotificationPopups.js'
import { createApiClient } from '../stores/api'
import RowPickerDialog from './RowPickerDialog.vue'
import { useRowPicker } from '../composables/useRowPicker'
import type { RowPickerOption } from '../utils/rowPicker'
import type { ForeignKey, Item } from '../gen/sickrock_pb'
import {
  matchReferencedRow,
  nameColumnOf,
  quickAddColumns,
  quickAddInputType,
  quickAddValue,
  hasQuickAddValue,
  referenceText,
  type QuickAddConfig,
  type QuickAddField,
} from '../utils/quickAdd'
import { rowName } from '../utils/tickList'

const props = defineProps<{
  /** Table configuration the rows are added to (the related table). */
  tableName: string
  /** Its column referencing the parent row, and the parent's key. */
  linkColumn: string
  linkValue: string
  config: QuickAddConfig
}>()

/** added: a row was added; createdReferences: rows were also created in linked tables. */
const emit = defineEmits<{ added: [createdReferences: boolean] }>()

type Reference = { table: string; rows: Item[]; nameColumn: string; rowName: string }

const client = createApiClient()
const fields = ref<QuickAddField[]>([])
const foreignKeys = ref<ForeignKey[]>([])
const references = ref<Record<string, Reference>>({})
const values = ref<Record<string, string | number | boolean>>({})
const saving = ref(false)
const errorMessage = ref('')
const formEl = ref<HTMLFormElement | null>(null)
const uid = Math.random().toString(36).slice(2)

const columns = computed(() => quickAddColumns(props.config, fields.value, props.linkColumn))

// Browse…: the picker dialog for one relation column at a time.
const rowPicker = ref<InstanceType<typeof RowPickerDialog> | null>(null)
const pickerColumn = ref('')
// Rows created from the dialog since the last add, so the panel reloads relation names after it.
let createdFromPicker = false
const pickerTable = computed(() => references.value[pickerColumn.value]?.table ?? '')
const picker = useRowPicker(pickerTable)
const pickerTitle = computed(() => {
  const field = columns.value.find(entry => entry.name === pickerColumn.value)
  return field ? `Choose ${labelOf(field).toLowerCase()}` : 'Choose'
})
// The row the typed text already picks, marked in the dialog.
const pickerSelected = computed(() => {
  const ref = referenceOf(pickerColumn.value)
  const typed = String(values.value[pickerColumn.value] ?? '')
  return ref ? String(matchReferencedRow(ref.rows, typed)?.id ?? '') : ''
})

function referenceOf(column: string): Reference | undefined {
  return references.value[column]
}

function labelOf(field: QuickAddField): string {
  return field.displayLabel?.trim() || field.name.replace(/_/g, ' ').replace(/^./, c => c.toUpperCase())
}

/** What a typed relation value will do: '' (matches a row), 'new' (creates one) or 'unknown'. */
function referenceState(column: string): '' | 'new' | 'unknown' {
  const ref = referenceOf(column)
  const typed = String(values.value[column] ?? '').trim()
  if (!ref || !typed || matchReferencedRow(ref.rows, typed)) return ''
  return props.config.createMissing && ref.nameColumn ? 'new' : 'unknown'
}

async function load() {
  const structure = await client.getTableStructure({ pageId: props.tableName })
  fields.value = (structure.fields ?? []).map(field => ({ name: field.name, type: field.type, displayLabel: field.displayLabel }))
  foreignKeys.value = structure.foreignKeys ?? []
  const shown = new Set(columns.value.map(field => field.name))
  const entries = await Promise.all(foreignKeys.value
    .filter(fk => shown.has(fk.columnName) && (fk.tableTcName || props.tableName) === props.tableName)
    .map(async (fk) => {
      const table = fk.referencedTableTcName || fk.referencedTable
      const [refStructure, refItems] = await Promise.all([
        client.getTableStructure({ pageId: table }),
        client.listItems({ tcName: table }),
      ])
      const refFields = (refStructure.fields ?? []).map(field => ({ name: field.name, type: field.type }))
      return [fk.columnName, {
        table,
        rows: refItems.items ?? [],
        nameColumn: nameColumnOf(refFields),
        rowName: refStructure.rowName?.trim() || table,
      }] as const
    }))
  references.value = Object.fromEntries(entries)
  clearValues()
}

function clearValues() {
  values.value = Object.fromEntries(columns.value.map(field => [field.name, quickAddInputType(field.type) === 'checkbox' ? false : '']))
}

/** Opens Browse… for a relation column, searching for what has been typed in it. */
async function openPicker(column: string) {
  const ref = referenceOf(column)
  if (!ref) return
  pickerColumn.value = column
  const typed = String(values.value[column] ?? '').trim()
  await nextTick()
  rowPicker.value?.open(matchReferencedRow(ref.rows, typed) ? '' : typed)
  await picker.load(ref.rows)
}

/** Fills the column with the chosen row and moves on to the next input. */
function choose(column: string, rowId: string) {
  const ref = referenceOf(column)
  const row = ref?.rows.find(entry => String(entry.id) === rowId)
  if (!ref || !row) return
  values.value[column] = referenceText(ref.rows, row)
  picker.remember(rowId)
  nextTick(() => {
    const inputs = [...(formEl.value?.querySelectorAll<HTMLInputElement>('input') ?? [])]
    const current = inputs.findIndex(input => input.dataset.column === column)
    ;(inputs[current + 1] ?? formEl.value?.querySelector<HTMLButtonElement>('button[type="submit"]'))?.focus()
  })
}

function onPickerPick(option: RowPickerOption) {
  choose(pickerColumn.value, option.value)
}

/** A row created from the dialog joins the column's rows and is chosen. */
async function onPickerCreated(item: Item | undefined) {
  const column = pickerColumn.value
  const ref = referenceOf(column)
  if (!ref || !item) return
  try {
    ref.rows = await picker.reloadRows()
  } catch {
    ref.rows = [...ref.rows, item]
  }
  createdFromPicker = true
  choose(column, String(item.id))
}

function focusFirst() {
  nextTick(() => formEl.value?.querySelector<HTMLInputElement>('input')?.focus())
}

/** The id for a relation column's typed name, creating the referenced row when allowed. */
async function resolveReference(column: string, typed: string): Promise<{ id: string; created: boolean }> {
  const ref = referenceOf(column)!
  const match = matchReferencedRow(ref.rows, typed)
  if (match) return { id: String(match.id), created: false }
  if (!props.config.createMissing || !ref.nameColumn) {
    throw new Error(`No ${ref.rowName.toLowerCase()} called "${typed}".`)
  }
  const res = await client.createItem({ pageId: ref.table, additionalFields: { [ref.nameColumn]: typed } })
  if (!res.item) throw new Error(`Could not create ${ref.rowName.toLowerCase()} "${typed}".`)
  ref.rows = [...ref.rows, res.item]
  return { id: String(res.item.id), created: true }
}

async function add() {
  if (saving.value) return
  if (!columns.value.some(field => hasQuickAddValue(values.value[field.name]))) return
  saving.value = true
  errorMessage.value = ''
  try {
    const additionalFields: Record<string, string> = { [props.linkColumn]: props.linkValue }
    const created: string[] = []
    let label = ''
    for (const field of columns.value) {
      const raw = values.value[field.name]
      if (referenceOf(field.name) && typeof raw === 'string' && raw.trim()) {
        const resolved = await resolveReference(field.name, raw.trim())
        additionalFields[field.name] = resolved.id
        if (resolved.created) created.push(raw.trim())
        label ||= raw.trim()
        continue
      }
      const value = quickAddValue(field.type, raw ?? '')
      if (value !== '') additionalFields[field.name] = value
      if (typeof raw !== 'boolean') label ||= String(raw ?? '').trim()
    }
    await client.createItem({ pageId: props.tableName, additionalFields })
    showNotificationPopup({
      message: created.length > 0 ? `Added ${label} (new: ${created.join(', ')})` : `Added ${label}`,
      class: 'good',
    })
    clearValues()
    emit('added', created.length > 0 || createdFromPicker)
    createdFromPicker = false
    focusFirst()
  } catch (e: unknown) {
    errorMessage.value = e instanceof Error ? e.message : 'Could not add the row.'
  } finally {
    saving.value = false
  }
}

// Reload only when the table or settings really change, not when the panel re-renders (which would
// clear what is being typed).
watch(() => `${props.tableName}|${JSON.stringify(props.config)}`, load)
onMounted(load)
</script>

<template>
  <div>
  <form ref="formEl" class="quick-add-row padding" :aria-label="`Quick add to ${tableName}`" @submit.prevent="add">
    <div class="quick-add-fields">
      <label v-for="field in columns" :key="field.name" class="quick-add-field">
        <span class="quick-add-label">{{ labelOf(field) }}</span>
        <template v-if="referenceOf(field.name)">
          <span class="quick-add-reference">
            <input
              v-model="values[field.name] as string"
              type="text"
              :list="`quick-add-${uid}-${field.name}`"
              :data-column="field.name"
              autocomplete="off"
              :disabled="saving"
              placeholder="Type a name"
            />
            <button
              type="button"
              class="neutral inline-icon"
              :aria-label="`Browse ${labelOf(field).toLowerCase()}`"
              title="Browse…"
              :disabled="saving"
              @click="openPicker(field.name)"
            >
              <HugeiconsIcon :icon="Search01Icon" width="1em" height="1em" aria-hidden="true" />
            </button>
          </span>
          <datalist :id="`quick-add-${uid}-${field.name}`">
            <option v-for="row in referenceOf(field.name)!.rows" :key="String(row.id)" :value="rowName(row)" />
          </datalist>
          <small v-if="referenceState(field.name) === 'new'" class="quick-add-hint">New {{ referenceOf(field.name)!.rowName.toLowerCase() }}: created when added</small>
          <small v-else-if="referenceState(field.name) === 'unknown'" class="quick-add-hint quick-add-unknown">No matching {{ referenceOf(field.name)!.rowName.toLowerCase() }}</small>
        </template>
        <input
          v-else-if="quickAddInputType(field.type) === 'checkbox'"
          :data-column="field.name"
          v-model="values[field.name] as boolean"
          type="checkbox"
          :disabled="saving"
        />
        <input
          v-else
          v-model="values[field.name] as string"
          :data-column="field.name"
          :type="quickAddInputType(field.type)"
          :step="quickAddInputType(field.type) === 'number' ? 'any' : undefined"
          :disabled="saving"
        />
      </label>
      <button type="submit" class="button inline-icon good quick-add-submit" :disabled="saving">
        <HugeiconsIcon :icon="Add01Icon" width="1em" height="1em" aria-hidden="true" />
        <span>{{ saving ? 'Adding…' : 'Add' }}</span>
      </button>
    </div>
    <NotificationBlock v-if="errorMessage" type="error" :message="errorMessage" />
  </form>
  <!-- Outside the form: the dialog can hold a create form of its own. -->
  <RowPickerDialog
    ref="rowPicker"
    :title="pickerTitle"
    :options="picker.options.value"
    :layout="picker.layout.value"
    :selected-value="pickerSelected"
    :recent-values="picker.recent.value"
    :loading="picker.loading.value"
    :create-label="picker.createLabel.value"
    :create-table-id="pickerTable"
    :create-field-defs="picker.settings.value.fieldDefs"
    @pick="onPickerPick"
    @created="onPickerCreated"
  />
  </div>
</template>

<style scoped>
.quick-add-row {
  border-top: 1px solid var(--border-color, #dee2e6);
}

.quick-add-fields {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  gap: 0.5rem 1rem;
}

.quick-add-field {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  min-width: 8rem;
  flex: 1 1 8rem;
  font-weight: normal;
}

.quick-add-label {
  font-size: 0.85em;
  font-weight: 600;
}

.quick-add-reference {
  display: flex;
  gap: 0.25rem;
}

.quick-add-reference input {
  flex: 1 1 auto;
  min-width: 0;
}

.quick-add-reference button {
  flex: none;
  margin: 0;
}

.quick-add-hint {
  font-size: 0.8em;
}

.quick-add-unknown {
  color: var(--karma-warning, inherit);
}

.quick-add-submit {
  flex: 0 0 auto;
}
</style>
