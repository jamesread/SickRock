<script setup lang="ts">
// Adds a column to a table: either a standard field (a well-known column, added in one step) or a
// custom field with a name and data type, including enhanced types such as images and barcodes.
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import FormLayout from 'picocrank/vue/components/FormLayout.vue'
import FormField from 'picocrank/vue/components/FormField.vue'
import RadioGroup from 'picocrank/vue/components/RadioGroup.vue'
import { YES_NO_OPTIONS } from '../utils/fieldTypes'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'

import { createApiClient } from '../stores/api'
import {
  COLUMN_TYPE_OPTIONS,
  columnNameTaken,
  columnTypeOption,
  missingStandardFields,
  type NewColumnType,
  type StandardField,
} from '../utils/addColumn'

const props = withDefaults(defineProps<{
  tableId: string
  initialName?: string
  initialType?: NewColumnType
  navigateOnSuccess?: boolean
}>(), {
  navigateOnSuccess: true,
})
const emit = defineEmits<{ added: [] }>()

const router = useRouter()
const client = createApiClient()

type Mode = 'standard' | 'custom'
const mode = ref<Mode>(props.initialName || props.initialType ? 'custom' : 'standard')

const existingColumns = ref<string[]>([])
const bulkEntryScanColumn = ref('')
const loadingStructure = ref(true)

const name = ref(props.initialName ?? '')
const type = ref<NewColumnType>(props.initialType ?? 'string')
const required = ref(false)
const defaultToCurrentTimestamp = ref(false)
const useForBulkEntry = ref(true)
const saving = ref(false)
const error = ref<string | null>(null)

const standardChoices = computed(() => missingStandardFields(existingColumns.value))
const basicTypes = COLUMN_TYPE_OPTIONS.filter(o => !o.enhanced)
const enhancedTypes = COLUMN_TYPE_OPTIONS.filter(o => o.enhanced)
const nameTaken = computed(() => columnNameTaken(name.value, existingColumns.value))

const bulkEntryDescription = computed(() => bulkEntryScanColumn.value
  ? `The table's Bulk entry scans into "${bulkEntryScanColumn.value}" now; this column would replace it.`
  : 'Adds a Bulk entry button to the table, for entering rows quickly by scanning barcodes.')

onMounted(async () => {
  try {
    const res = await client.getTableStructure({ pageId: props.tableId })
    existingColumns.value = (res.fields ?? []).map(f => f.name)
    bulkEntryScanColumn.value = res.bulkEntry?.scanColumn ?? ''
    useForBulkEntry.value = !bulkEntryScanColumn.value
  } catch (e) {
    error.value = `Could not load the table's columns: ${e instanceof Error ? e.message : String(e)}`
  } finally {
    loadingStructure.value = false
  }
  if (mode.value === 'standard' && standardChoices.value.length === 0 && !error.value) {
    mode.value = 'custom'
  }
})

async function addColumn(field: { name: string, type: NewColumnType, required: boolean, defaultToCurrentTimestamp: boolean }, bulkEntry: boolean) {
  if (saving.value) return
  saving.value = true
  error.value = null
  try {
    await client.addTableColumn({
      pageId: props.tableId,
      field,
      useForBulkEntry: field.type === 'barcode' && bulkEntry,
    })
    emit('added')
    if (props.navigateOnSuccess) {
      router.push({ name: 'after-insert', params: { tableName: props.tableId } })
    }
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    saving.value = false
  }
}

function addStandard(field: StandardField) {
  void addColumn({
    name: field.name,
    type: field.type,
    required: false,
    defaultToCurrentTimestamp: !!field.defaultToCurrentTimestamp,
  }, !!field.bulkEntryScan && !bulkEntryScanColumn.value)
}

function addCustom() {
  const trimmed = name.value.trim()
  if (!trimmed || nameTaken.value) return
  void addColumn({
    name: trimmed,
    type: type.value,
    required: required.value,
    defaultToCurrentTimestamp: type.value === 'datetime' && defaultToCurrentTimestamp.value,
  }, useForBulkEntry.value)
}
</script>

<template>
  <div class="add-column">
    <RadioGroup
      v-model="mode"
      name="add-column-mode"
      aria-label="Kind of column"
      :options="[
        { value: 'standard', label: 'Standard field' },
        { value: 'custom', label: 'Custom field' },
      ]"
    />

    <template v-if="mode === 'standard'">
      <p class="subtle">Common columns, set up for you in one step.</p>
      <p v-if="loadingStructure" class="subtle">Loading…</p>
      <p v-else-if="standardChoices.length === 0" class="inline-notification note">
        This table already has every standard field.
      </p>
      <ul v-else class="standard-fields">
        <li v-for="field in standardChoices" :key="field.name">
          <div class="standard-field-info">
            <strong>{{ field.label }}</strong>
            <code>{{ field.name }}</code>
            <span v-if="field.recommended" class="tag fg-good">Recommended</span>
            <p class="subtle">{{ field.description }}</p>
          </div>
          <button
            type="button"
            class="button good"
            :disabled="saving"
            :aria-label="`Add the ${field.label} field`"
            @click="addStandard(field)"
          >
            Add
          </button>
        </li>
      </ul>
    </template>

    <template v-else>
      <p class="subtle">A column with your own name and data type.</p>
      <FormLayout id="add-column-form" @submit.prevent="addCustom">
        <FormField
          label="Column name"
          for="addcol-name"
          description="The column's identifier in the database."
        >
          <input
            id="addcol-name"
            v-model="name"
            type="text"
            placeholder="e.g. expiry_date"
            autocomplete="off"
            required
            :disabled="saving"
          />
          <NotificationBlock v-if="nameTaken" type="warning" :message="`This table already has a column called &quot;${name.trim()}&quot;.`" />
        </FormField>

        <FormField label="Data type" for="addcol-type" :description="columnTypeOption(type).description">
          <select id="addcol-type" v-model="type" :disabled="saving">
            <optgroup label="Basic">
              <option v-for="o in basicTypes" :key="o.value" :value="o.value">{{ o.label }}</option>
            </optgroup>
            <optgroup label="Enhanced">
              <option v-for="o in enhancedTypes" :key="o.value" :value="o.value">{{ o.label }}</option>
            </optgroup>
          </select>
        </FormField>

        <FormField
          v-if="type === 'barcode'"
          label="Use for bulk entry"
          component-has-label
          :description="bulkEntryDescription"
        >
          <RadioGroup
            variant="boolean"
            :options="YES_NO_OPTIONS"
            v-model="useForBulkEntry"
            :disabled="saving"
            aria-label="Use for bulk entry"
          />
        </FormField>

        <FormField
          v-if="type === 'datetime'"
          label="Default to now"
          component-has-label
          description="New rows get the current date and time."
        >
          <RadioGroup
            variant="boolean"
            :options="YES_NO_OPTIONS"
            v-model="defaultToCurrentTimestamp"
            :disabled="saving"
            aria-label="Default to now"
          />
        </FormField>

        <FormField label="Required" component-has-label description="The column cannot be left empty.">
          <RadioGroup
            variant="boolean"
            :options="YES_NO_OPTIONS"
            v-model="required"
            :disabled="saving"
            aria-label="Required"
          />
        </FormField>

        <template #actions>
          <button
            id="addcol-submit"
            type="submit"
            class="good"
            :disabled="saving || !name.trim() || nameTaken"
          >
            {{ saving ? 'Adding…' : 'Add column' }}
          </button>
        </template>
      </FormLayout>
    </template>

    <NotificationBlock v-if="error" type="error" :message="error" />
  </div>
</template>

<style scoped>
.add-column {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.add-column > p {
  margin: 0;
}

.standard-fields {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.standard-fields li {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.75rem 1rem;
  border: 1px solid var(--border-color);
  border-radius: 4px;
}

.standard-field-info {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 0.25rem 0.5rem;
  min-width: 0;
}

.standard-field-info p {
  flex-basis: 100%;
  margin: 0;
}
</style>
