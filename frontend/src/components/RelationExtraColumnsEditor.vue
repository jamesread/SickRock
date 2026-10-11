<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import FormField from 'picocrank/vue/components/FormField.vue'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'
import Table from 'picocrank/vue/components/Table.vue'
import type { createApiClient } from '../stores/api'
import type { Field, ForeignKey, GetTableStructureResponse, RelationPullThroughField } from '../gen/sickrock_pb'

const props = withDefaults(defineProps<{
  sourceTableId: string
  constraintName: string
  pullThroughFields?: RelationPullThroughField[]
  displayName?: string
  client: ReturnType<typeof createApiClient>
  disabled?: boolean
  showDisplayNameField?: boolean
  idPrefix?: string
}>(), {
  pullThroughFields: () => [],
  displayName: '',
  disabled: false,
  showDisplayNameField: false,
  idPrefix: 'rel-extra',
})

const emit = defineEmits<{
  saved: []
}>()

const loading = ref(true)
const saving = ref(false)
const error = ref<string | null>(null)
const displayNameDraft = ref('')
type PullThroughRow = { rowId: number; fkColumn: string; field: string }
type CalculatedRow = { rowId: number; displayLabel: string; expr: string }

let nextPullThroughRowId = 0
let nextCalculatedRowId = 0

const pullThroughDraft = ref<PullThroughRow[]>([])
const calculatedDraft = ref<CalculatedRow[]>([])

const pullThroughHeaders = computed(() => {
  const headers = [
    { key: 'fkColumn', label: 'Foreign key column', sortable: false },
    { key: 'field', label: 'Field on linked row', sortable: false },
  ]
  if (!props.disabled) {
    headers.push({ key: 'actions', label: '', sortable: false })
  }
  return headers
})

const calculatedHeaders = computed(() => {
  const headers = [
    { key: 'displayLabel', label: 'Column label', sortable: false },
    { key: 'expr', label: 'Expression', sortable: false },
  ]
  if (!props.disabled) {
    headers.push({ key: 'actions', label: '', sortable: false })
  }
  return headers
})
const childFields = ref<Field[]>([])
const childForeignKeys = ref<ForeignKey[]>([])
const referencedFieldsByFk = ref<Record<string, Field[]>>({})
const structureCache = ref<Record<string, GetTableStructureResponse>>({})

function fieldId(suffix: string): string {
  return `${props.idPrefix}-${props.constraintName}-${suffix}`
}

function applyPullThroughFields(columns: RelationPullThroughField[]) {
  nextPullThroughRowId = 0
  nextCalculatedRowId = 0
  pullThroughDraft.value = columns
    .filter(col => !col.expr?.trim())
    .map(col => ({
      rowId: ++nextPullThroughRowId,
      fkColumn: col.fkColumn,
      field: col.field,
    }))
  calculatedDraft.value = columns
    .filter(col => col.expr?.trim())
    .map(col => ({
      rowId: ++nextCalculatedRowId,
      displayLabel: col.displayLabel?.trim() ?? '',
      expr: col.expr.trim(),
    }))
}

async function ensureStructure(tcName: string): Promise<GetTableStructureResponse> {
  if (structureCache.value[tcName]) {
    return structureCache.value[tcName]
  }
  const structure = await props.client.getTableStructure({ pageId: tcName })
  structureCache.value[tcName] = structure
  return structure
}

async function loadContext() {
  loading.value = true
  error.value = null
  try {
    displayNameDraft.value = props.displayName ?? ''
    applyPullThroughFields(props.pullThroughFields ?? [])
    const childStructure = await ensureStructure(props.sourceTableId)
    childFields.value = childStructure.fields ?? []
    childForeignKeys.value = (childStructure.foreignKeys ?? []).filter(fk => {
      const source = (fk.tableTcName?.trim() || fk.tableName).trim()
      return source === props.sourceTableId.trim()
    })
    referencedFieldsByFk.value = {}
    for (const row of pullThroughDraft.value) {
      if (!row.fkColumn || referencedFieldsByFk.value[row.fkColumn]) {
        continue
      }
      const childFk = childForeignKeys.value.find(entry => entry.columnName === row.fkColumn)
      if (!childFk) {
        continue
      }
      const refTc = childFk.referencedTableTcName || childFk.referencedTable
      const refStructure = await ensureStructure(refTc)
      referencedFieldsByFk.value[row.fkColumn] = refStructure.fields ?? []
    }
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

function pullThroughRowIndex(row: PullThroughRow): number {
  return pullThroughDraft.value.findIndex(entry => entry.rowId === row.rowId)
}

function calculatedRowIndex(row: CalculatedRow): number {
  return calculatedDraft.value.findIndex(entry => entry.rowId === row.rowId)
}

async function onPullThroughFkChange(row: PullThroughRow) {
  if (!row.fkColumn) {
    return
  }
  row.field = ''
  const childFk = childForeignKeys.value.find(entry => entry.columnName === row.fkColumn)
  if (!childFk) {
    return
  }
  const refTc = childFk.referencedTableTcName || childFk.referencedTable
  const refStructure = await ensureStructure(refTc)
  referencedFieldsByFk.value = {
    ...referencedFieldsByFk.value,
    [row.fkColumn]: refStructure.fields ?? [],
  }
}

function addPullThroughRow() {
  pullThroughDraft.value.push({ rowId: ++nextPullThroughRowId, fkColumn: '', field: '' })
}

function removePullThroughRow(row: PullThroughRow) {
  const index = pullThroughRowIndex(row)
  if (index >= 0) {
    pullThroughDraft.value.splice(index, 1)
  }
}

function addCalculatedRow() {
  calculatedDraft.value.push({ rowId: ++nextCalculatedRowId, displayLabel: '', expr: '' })
}

function removeCalculatedRow(row: CalculatedRow) {
  const index = calculatedRowIndex(row)
  if (index >= 0) {
    calculatedDraft.value.splice(index, 1)
  }
}

async function save() {
  if (props.disabled || saving.value || !props.constraintName) {
    return
  }
  saving.value = true
  error.value = null
  try {
    const pullThroughFields: RelationPullThroughField[] = [
      ...pullThroughDraft.value
        .filter(row => row.fkColumn.trim() && row.field.trim())
        .map(row => ({
          fkColumn: row.fkColumn.trim(),
          field: row.field.trim(),
          displayLabel: '',
          expr: '',
        })),
      ...calculatedDraft.value
        .filter(row => row.displayLabel.trim() && row.expr.trim())
        .map(row => ({
          fkColumn: '',
          field: '',
          displayLabel: row.displayLabel.trim(),
          expr: row.expr.trim(),
        })),
    ]
    const res = await props.client.updateTableRelation({
      constraintName: props.constraintName,
      displayName: props.showDisplayNameField ? displayNameDraft.value.trim() : (props.displayName ?? ''),
      pullThroughFields,
    })
    if (!res.success) {
      throw new Error(res.message || 'Failed to update relation')
    }
    emit('saved')
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    saving.value = false
  }
}

watch(
  () => [props.constraintName, props.pullThroughFields] as const,
  () => {
    void loadContext()
  },
)

onMounted(() => {
  void loadContext()
})
</script>

<template>
  <div class="relation-extra-columns-editor">
    <p v-if="loading" class="muted">Loading relation fields…</p>
    <template v-else>
      <FormField
        v-if="showDisplayNameField"
        label="Friendly name (related list title)"
        :for="fieldId('display-name')"
        :disabled="disabled || saving"
      >
        <input
          :id="fieldId('display-name')"
          v-model="displayNameDraft"
          type="text"
          placeholder="e.g. Line items"
          autocomplete="off"
          :disabled="disabled || saving"
        />
      </FormField>

      <h4 class="relation-extra-subheading">Pull-through columns</h4>
      <p class="muted relation-extra-hint">
        Show fields from rows linked by foreign keys on this table in the main table list and related embeds.
      </p>
      <p v-if="pullThroughDraft.length === 0" class="muted relation-extra-empty">No pull-through columns yet.</p>
      <Table
        v-else
        :data="pullThroughDraft"
        :headers="pullThroughHeaders"
        row-key="rowId"
        :filterable="false"
        :column-options="false"
        class="relation-extra-grid"
      >
        <template #cell-fkColumn="{ row }">
          <select
            :id="fieldId(`pt-fk-${row.rowId}`)"
            v-model="row.fkColumn"
            class="grid-input"
            :disabled="disabled || saving"
            @change="onPullThroughFkChange(row)"
          >
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
          <select
            :id="fieldId(`pt-field-${row.rowId}`)"
            v-model="row.field"
            class="grid-input"
            :disabled="disabled || saving || !row.fkColumn"
          >
            <option value="">— Select —</option>
            <option
              v-for="field in (referencedFieldsByFk[row.fkColumn] ?? [])"
              :key="field.name"
              :value="field.name"
            >
              {{ field.displayLabel || field.name }}
            </option>
          </select>
        </template>
        <template v-if="!disabled" #cell-actions="{ row }">
          <button
            type="button"
            class="neutral inline-icon"
            :disabled="saving"
            @click="removePullThroughRow(row)"
          >
            Remove
          </button>
        </template>
      </Table>
      <button
        v-if="!disabled"
        type="button"
        class="neutral inline-icon relation-extra-add"
        :disabled="saving || childForeignKeys.length === 0"
        @click="addPullThroughRow"
      >
        Add pull-through column
      </button>

      <h4 class="relation-extra-subheading">Calculated columns</h4>
      <p class="muted relation-extra-hint">
        Expressions use <code>{table.column}</code> placeholders and are evaluated on the server for list views.
      </p>
      <p v-if="calculatedDraft.length === 0" class="muted relation-extra-empty">No calculated columns yet.</p>
      <Table
        v-else
        :data="calculatedDraft"
        :headers="calculatedHeaders"
        row-key="rowId"
        :filterable="false"
        :column-options="false"
        class="relation-extra-grid"
      >
        <template #cell-displayLabel="{ row }">
          <input
            :id="fieldId(`calc-label-${row.rowId}`)"
            v-model="row.displayLabel"
            type="text"
            class="grid-input"
            placeholder="e.g. Line total"
            autocomplete="off"
            :disabled="disabled || saving"
          />
        </template>
        <template #cell-expr="{ row }">
          <input
            :id="fieldId(`calc-expr-${row.rowId}`)"
            v-model="row.expr"
            type="text"
            class="grid-input"
            placeholder="{skus.value} * {quote_lines.quantity}"
            autocomplete="off"
            :disabled="disabled || saving"
          />
        </template>
        <template v-if="!disabled" #cell-actions="{ row }">
          <button
            type="button"
            class="neutral inline-icon"
            :disabled="saving"
            @click="removeCalculatedRow(row)"
          >
            Remove
          </button>
        </template>
      </Table>
      <button
        v-if="!disabled"
        type="button"
        class="neutral inline-icon relation-extra-add"
        :disabled="saving"
        @click="addCalculatedRow"
      >
        Add calculated column
      </button>

      <NotificationBlock v-if="error" type="error" :message="error" />
      <div v-if="!disabled" class="relation-extra-actions">
        <button type="button" class="good" :disabled="saving" @click="save">
          {{ saving ? 'Saving…' : 'Save table view columns' }}
        </button>
      </div>
    </template>
  </div>
</template>

<style scoped>
.relation-extra-subheading {
  margin: 1rem 0 0.25rem;
  font-size: 1em;
}

.relation-extra-hint {
  margin: 0 0 0.5rem;
}

.relation-extra-empty {
  margin: 0 0 0.5rem;
  font-size: 0.95em;
}

.relation-extra-grid {
  margin-bottom: 0.5rem;
}

.relation-extra-add {
  margin-bottom: 0.75rem;
}

.grid-input {
  width: 100%;
  min-width: 8rem;
  box-sizing: border-box;
}

.relation-extra-actions {
  margin-top: 1rem;
}
</style>
