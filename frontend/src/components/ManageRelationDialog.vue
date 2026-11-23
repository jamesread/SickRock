<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { Delete01Icon } from '@hugeicons/core-free-icons'
import Dialog from 'picocrank/vue/components/Dialog.vue'
import Tabs from 'picocrank/vue/components/Tabs.vue'
import Table from 'picocrank/vue/components/Table.vue'
import FormField from 'picocrank/vue/components/FormField.vue'
import FormLayout from 'picocrank/vue/components/FormLayout.vue'
import RadioGroup from 'picocrank/vue/components/RadioGroup.vue'
import CheckGroup from 'picocrank/vue/components/CheckGroup.vue'
import type { Field, ForeignKey, GetTableStructureResponse, RelationPullThroughField } from '../gen/sickrock_pb'
import type { createApiClient } from '../stores/api'
import type { DialogApi } from '../utils/dialog'
import { YES_NO_OPTIONS } from '../utils/fieldTypes'
import { defaultQuickAddConfig, parseQuickAddConfig, quickAddCandidateColumns, serializeQuickAddConfig } from '../utils/quickAdd'
import { relationReferencedTableTcName, relationSourceTableTcName } from '../utils/rowViewRelatedRelations'

/**
 * Settings for one relation's related rows on an item page: its name and layout (General), the
 * quick add row (Quick add), and extra pull-through and calculated columns (Columns).
 */
const props = defineProps<{
  client: ReturnType<typeof createApiClient>
  /** Loads (and may cache) a table configuration's structure. */
  loadStructure: (tcName: string) => Promise<GetTableStructureResponse>
}>()

const emit = defineEmits<{ saved: [] }>()

/** A relation as the item page holds it (the message's fields, without protobuf metadata). */
export type ManagedRelation = Omit<ForeignKey, '$typeName' | '$unknown' | 'tableTcName' | 'referencedTableTcName'> & {
  tableTcName?: string
  referencedTableTcName?: string
}

type PullThroughRow = { rowId: number; fkColumn: string; field: string }
type CalculatedRow = { rowId: number; displayLabel: string; expr: string }

const LAYOUT_OPTIONS = [
  { value: false, label: 'Own section' },
  { value: true, label: 'Inside this item' },
]

const dialog = ref<DialogApi | null>(null)
const fk = ref<ManagedRelation | null>(null)
const displayName = ref('')
const absorb = ref(false)
const absorbType = ref('first_row')
const quickAdd = ref(defaultQuickAddConfig())
const pullThrough = ref<PullThroughRow[]>([])
const calculated = ref<CalculatedRow[]>([])
const childFields = ref<Field[]>([])
const childForeignKeys = ref<ForeignKey[]>([])
const referencedFieldsByFk = ref<Record<string, Field[]>>({})
const loading = ref(false)
const saving = ref(false)
const error = ref<string | null>(null)
let nextRowId = 0
// Remounts the tabs on each open, so the dialog always starts on General.
const openCount = ref(0)

const sourceLabel = computed(() => fk.value ? `${relationSourceTableTcName(fk.value)}.${fk.value.columnName}` : '')
const destinationLabel = computed(() => fk.value ? `${relationReferencedTableTcName(fk.value)}.${fk.value.referencedColumn}` : '')

const quickAddOptions = computed(() => quickAddCandidateColumns(childFields.value, fk.value?.columnName ?? '')
  .map(field => ({ value: field.name, label: field.displayLabel?.trim() || field.name })))

const columnCount = computed(() => pullThrough.value.length + calculated.value.length)

const tabs = computed(() => [
  { id: 'general', label: 'General' },
  { id: 'quick-add', label: quickAdd.value.enabled ? 'Quick add (on)' : 'Quick add' },
  { id: 'columns', label: columnCount.value ? `Columns (${columnCount.value})` : 'Columns' },
])

const pullThroughHeaders = [
  { key: 'fkColumn', label: 'Linked by', sortable: false },
  { key: 'field', label: 'Field', sortable: false },
  { key: 'actions', label: '', ariaLabel: 'Remove', sortable: false },
]

const calculatedHeaders = [
  { key: 'displayLabel', label: 'Label', sortable: false },
  { key: 'expr', label: 'Expression', sortable: false },
  { key: 'actions', label: '', ariaLabel: 'Remove', sortable: false },
]

async function referencedFields(fkColumn: string): Promise<Field[]> {
  const childFk = childForeignKeys.value.find(entry => entry.columnName === fkColumn)
  if (!childFk) return []
  const structure = await props.loadStructure(childFk.referencedTableTcName || childFk.referencedTable)
  return structure.fields ?? []
}

async function loadContext() {
  if (!fk.value) return
  loading.value = true
  try {
    const structure = await props.loadStructure(relationSourceTableTcName(fk.value))
    childFields.value = structure.fields ?? []
    childForeignKeys.value = structure.foreignKeys ?? []
    const byFk: Record<string, Field[]> = {}
    for (const row of pullThrough.value) {
      if (row.fkColumn && !byFk[row.fkColumn]) byFk[row.fkColumn] = await referencedFields(row.fkColumn)
    }
    referencedFieldsByFk.value = byFk
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

/** Opens the dialog for a relation, with its related list's current title. */
function open(relation: ManagedRelation, currentDisplayName?: string) {
  fk.value = relation
  displayName.value = currentDisplayName ?? relation.displayName ?? ''
  absorb.value = !!relation.absorbIntoItem
  absorbType.value = relation.absorbType || 'first_row'
  quickAdd.value = parseQuickAddConfig(relation.quickAddJson)
  const columns = relation.pullThroughFields ?? []
  pullThrough.value = columns
    .filter(col => !col.expr?.trim())
    .map(col => ({ rowId: ++nextRowId, fkColumn: col.fkColumn, field: col.field }))
  calculated.value = columns
    .filter(col => col.expr?.trim())
    .map(col => ({ rowId: ++nextRowId, displayLabel: col.displayLabel?.trim() ?? '', expr: col.expr.trim() }))
  childFields.value = []
  childForeignKeys.value = []
  referencedFieldsByFk.value = {}
  error.value = null
  openCount.value++
  nextTick(() => {
    dialog.value?.show()
    void loadContext()
  })
}

async function onPullThroughFkChange(row: PullThroughRow) {
  row.field = ''
  if (!row.fkColumn) return
  const fields = await referencedFields(row.fkColumn)
  referencedFieldsByFk.value = { ...referencedFieldsByFk.value, [row.fkColumn]: fields }
}

function removeRow<T extends { rowId: number }>(rows: T[], row: T) {
  const index = rows.findIndex(entry => entry.rowId === row.rowId)
  if (index >= 0) rows.splice(index, 1)
}

function draftPullThroughFields(): RelationPullThroughField[] {
  return [
    ...pullThrough.value
      .filter(row => row.fkColumn.trim() && row.field.trim())
      .map(row => ({ fkColumn: row.fkColumn.trim(), field: row.field.trim(), displayLabel: '', expr: '' })),
    ...calculated.value
      .filter(row => row.displayLabel.trim() && row.expr.trim())
      .map(row => ({ fkColumn: '', field: '', displayLabel: row.displayLabel.trim(), expr: row.expr.trim() })),
  ] as RelationPullThroughField[]
}

async function save() {
  if (!fk.value || saving.value) return
  saving.value = true
  error.value = null
  try {
    const res = await props.client.updateTableRelation({
      constraintName: fk.value.constraintName,
      displayName: displayName.value.trim(),
      pullThroughFields: draftPullThroughFields(),
      absorbIntoItem: absorb.value,
      absorbType: absorb.value ? absorbType.value : '',
      quickAddJson: serializeQuickAddConfig(quickAdd.value),
    })
    if (!res.success) throw new Error(res.message || 'Failed to update relation')
    dialog.value?.close()
    emit('saved')
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    saving.value = false
  }
}

defineExpose({ open })
</script>

<template>
  <Dialog ref="dialog" title="Manage relation" @close="fk = null">
    <template v-if="fk">
      <p class="muted relation-summary">
        <code>{{ sourceLabel }}</code> → <code>{{ destinationLabel }}</code>
      </p>
      <Tabs :key="openCount" :tabs="tabs" default-tab="general" :padding="true">
        <template #tab-general>
          <FormLayout @submit.prevent="save">
            <FormField label="Title" for="relation-display-name" :disabled="saving">
              <input
                id="relation-display-name"
                v-model="displayName"
                type="text"
                placeholder="e.g. Orders, Tasks, Line items"
                autocomplete="off"
                :disabled="saving"
              />
            </FormField>
            <FormField
              label="Show related rows"
              component-has-label
              :disabled="saving"
              :description="absorb
                ? 'The first related row\'s fields appear in this item\'s main section.'
                : 'Related rows are listed in a section of their own.'"
            >
              <RadioGroup v-model="absorb" :options="LAYOUT_OPTIONS" :disabled="saving" aria-label="Show related rows" />
            </FormField>
          </FormLayout>
          <details class="relation-technical">
            <summary class="muted">Technical details</summary>
            <dl>
              <dt>Relation ID</dt>
              <dd><code>{{ fk.relationId || '—' }}</code></dd>
              <dt>Constraint</dt>
              <dd><code>{{ fk.constraintName }}</code></dd>
            </dl>
          </details>
        </template>

        <template #tab-quick-add>
          <p class="muted">An entry row under the related list, for typing in rows one after another (e.g. a recipe's ingredients).</p>
          <FormLayout @submit.prevent="save">
            <FormField label="Quick add" component-has-label :disabled="saving">
              <RadioGroup
                v-model="quickAdd.enabled"
                variant="boolean"
                :options="YES_NO_OPTIONS"
                :disabled="saving"
                aria-label="Quick add"
              />
            </FormField>
            <template v-if="quickAdd.enabled">
              <FormField
                label="Columns"
                component-has-label
                :disabled="saving"
                description="In the order you tick them; none ticked shows them all."
              >
                <p v-if="loading" class="muted">Loading columns…</p>
                <CheckGroup
                  v-else
                  v-model="quickAdd.columns"
                  :options="quickAddOptions"
                  aria-label="Quick add columns"
                  :disabled="saving"
                />
              </FormField>
              <FormField
                label="Create missing linked rows"
                component-has-label
                :disabled="saving"
                description="Typing a name that matches nothing (e.g. a new ingredient) creates it."
              >
                <RadioGroup
                  v-model="quickAdd.createMissing"
                  variant="boolean"
                  :options="YES_NO_OPTIONS"
                  :disabled="saving"
                  aria-label="Create missing linked rows"
                />
              </FormField>
            </template>
          </FormLayout>
        </template>

        <template #tab-columns>
          <p v-if="loading" class="muted">Loading columns…</p>
          <div v-else class="relation-columns">
            <h4 class="relation-columns-heading">From linked rows</h4>
            <p class="muted">Fields of rows the related rows link to, e.g. each ingredient's aisle.</p>
            <Table
              v-if="pullThrough.length"
              :data="pullThrough"
              :headers="pullThroughHeaders"
              row-key="rowId"
              :filterable="false"
              :column-options="false"
              :show-pagination="false"
            >
              <template #cell-fkColumn="{ row }">
                <select v-model="row.fkColumn" aria-label="Linked by" :disabled="saving" @change="onPullThroughFkChange(row)">
                  <option value="">— Select —</option>
                  <option
                    v-for="childFk in childForeignKeys"
                    :key="childFk.constraintName || childFk.columnName"
                    :value="childFk.columnName"
                  >
                    {{ childFk.columnName }} → {{ childFk.referencedTableTcName || childFk.referencedTable }}
                  </option>
                </select>
              </template>
              <template #cell-field="{ row }">
                <select v-model="row.field" aria-label="Field" :disabled="saving || !row.fkColumn">
                  <option value="">— Select —</option>
                  <option v-for="field in (referencedFieldsByFk[row.fkColumn] ?? [])" :key="field.name" :value="field.name">
                    {{ field.displayLabel || field.name }}
                  </option>
                </select>
              </template>
              <template #cell-actions="{ row }">
                <button type="button" class="neutral inline-icon" aria-label="Remove" title="Remove" :disabled="saving" @click="removeRow(pullThrough, row)">
                  <HugeiconsIcon :icon="Delete01Icon" width="1em" height="1em" aria-hidden="true" />
                </button>
              </template>
            </Table>
            <button
              type="button"
              class="neutral inline-icon"
              :disabled="saving || childForeignKeys.length === 0"
              @click="pullThrough.push({ rowId: ++nextRowId, fkColumn: '', field: '' })"
            >
              Add linked field
            </button>

            <h4 class="relation-columns-heading">Calculated</h4>
            <p class="muted">
              Worked out on the server, using <code>{table.column}</code> placeholders,
              e.g. <code>{skus.value} * {quote_lines.quantity}</code>.
            </p>
            <Table
              v-if="calculated.length"
              :data="calculated"
              :headers="calculatedHeaders"
              row-key="rowId"
              :filterable="false"
              :column-options="false"
              :show-pagination="false"
            >
              <template #cell-displayLabel="{ row }">
                <input v-model="row.displayLabel" type="text" aria-label="Label" placeholder="e.g. Line total" autocomplete="off" :disabled="saving" />
              </template>
              <template #cell-expr="{ row }">
                <input v-model="row.expr" type="text" aria-label="Expression" placeholder="{skus.value} * {quote_lines.quantity}" autocomplete="off" :disabled="saving" />
              </template>
              <template #cell-actions="{ row }">
                <button type="button" class="neutral inline-icon" aria-label="Remove" title="Remove" :disabled="saving" @click="removeRow(calculated, row)">
                  <HugeiconsIcon :icon="Delete01Icon" width="1em" height="1em" aria-hidden="true" />
                </button>
              </template>
            </Table>
            <button
              type="button"
              class="neutral inline-icon"
              :disabled="saving"
              @click="calculated.push({ rowId: ++nextRowId, displayLabel: '', expr: '' })"
            >
              Add calculated column
            </button>
          </div>
        </template>
      </Tabs>
      <p v-if="error" class="inline-notification error">{{ error }}</p>
    </template>
    <template #actions>
      <button type="button" class="good" :disabled="saving || !fk" @click="save">
        {{ saving ? 'Saving…' : 'Save' }}
      </button>
    </template>
  </Dialog>
</template>

<style scoped>
.relation-summary {
  margin-top: 0;
}

.relation-columns-heading {
  margin: 1rem 0 0.25rem;
}

.relation-columns-heading:first-child {
  margin-top: 0;
}

.relation-columns :deep(select),
.relation-columns :deep(input) {
  width: 100%;
  min-width: 0;
  box-sizing: border-box;
}

.relation-technical dl {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 0.25rem 1rem;
  margin: 0.5rem 0 0;
}

.relation-technical dd {
  margin: 0;
}
</style>
