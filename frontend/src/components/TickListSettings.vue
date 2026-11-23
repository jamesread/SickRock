<script setup lang="ts">
// Settings for a Tick List view (stored in the view's layout_json, see utils/tickList.ts). Every
// choice is a column path: one of this table's columns, or a column of the row a relation column
// references (e.g. ingredient → aisle).
import { computed, ref, watch } from 'vue'
import { createApiClient } from '../stores/api'
import type { ForeignKey } from '../gen/sickrock_pb'
import { columnChoiceLabel } from '../utils/fieldColumnDisplay'
import { isImageFieldType } from '../utils/fieldTypes'
import { MAX_TICK_LIST_DETAILS, MAX_TICK_LIST_SORTS, type TickListConfig } from '../utils/tickList'

const props = defineProps<{
  fields: Array<{ name: string; type: string; displayLabel?: string }>
  foreignKeys: ForeignKey[]
}>()

const config = defineModel<TickListConfig>({ required: true })

const client = createApiClient()
// Columns of each referenced table (name and type), by relation column.
const referencedFields = ref<Record<string, Array<{ name: string; type: string }>>>({})

type PathChoice = { path: string; label: string }

const ownChoices = computed<PathChoice[]>(() => props.fields.map(field => {
  const name = columnChoiceLabel(field.name, field.displayLabel)
  const fk = props.foreignKeys.find(f => f.columnName === field.name)
  return { path: field.name, label: fk ? `${name} (name of ${fk.referencedTableTcName || fk.referencedTable})` : name }
}))

const referencedChoices = computed<PathChoice[]>(() => Object.entries(referencedFields.value).flatMap(
  ([column, fields]) => fields.map(field => ({ path: `${column}.${field.name}`, label: `${column} → ${field.name}` })),
))

// Image columns of this table and of the rows its relation columns reference.
const imageChoices = computed<PathChoice[]>(() => [
  ...props.fields.filter(field => isImageFieldType(field.type))
    .map(field => ({ path: field.name, label: columnChoiceLabel(field.name, field.displayLabel) })),
  ...Object.entries(referencedFields.value).flatMap(([column, fields]) => fields
    .filter(field => isImageFieldType(field.type))
    .map(field => ({ path: `${column}.${field.name}`, label: `${column} → ${field.name}` }))),
])

const allChoices = computed(() => [...ownChoices.value, ...referencedChoices.value])
const detailChoices = computed(() => allChoices.value.filter(choice => choice.path !== config.value.labelColumn))
const sortSlots = computed(() => {
  const filled = config.value.sortColumns.length
  return Math.min(MAX_TICK_LIST_SORTS, filled + 1)
})

async function loadReferencedFields() {
  const entries = await Promise.all(props.foreignKeys.map(async (fk) => {
    try {
      const res = await client.getTableStructure({ pageId: fk.referencedTableTcName || fk.referencedTable })
      return [fk.columnName, (res.fields ?? []).filter(field => field.name !== 'id').map(field => ({ name: field.name, type: field.type }))] as const
    } catch {
      return [fk.columnName, []] as const
    }
  }))
  referencedFields.value = Object.fromEntries(entries)
}

function update(patch: Partial<TickListConfig>) {
  config.value = { ...config.value, ...patch }
}

function toggleDetail(path: string, checked: boolean) {
  const details = config.value.detailColumns.filter(column => column !== path)
  update({ detailColumns: checked ? [...details, path] : details })
}

function detailDisabled(path: string): boolean {
  return !config.value.detailColumns.includes(path) && config.value.detailColumns.length >= MAX_TICK_LIST_DETAILS
}

function setSort(index: number, path: string) {
  const sorts = [...config.value.sortColumns]
  if (path) sorts[index] = path
  else sorts.splice(index)
  update({ sortColumns: sorts.filter(Boolean) })
}

watch(() => props.foreignKeys, loadReferencedFields, { immediate: true })
</script>

<template>
  <div class="tick-list-settings">
    <label for="tick-list-label">Label</label>
    <div>
      <select id="tick-list-label" :value="config.labelColumn" @change="update({ labelColumn: ($event.target as HTMLSelectElement).value })">
        <option value="">Automatic (name or title)</option>
        <option v-for="choice in allChoices" :key="choice.path" :value="choice.path">{{ choice.label }}</option>
      </select>
      <small>A relation column shows the name of the row it refers to.</small>
    </div>

    <span class="fake-label">Details</span>
    <div>
      <div class="tick-list-settings-details">
        <label v-for="choice in detailChoices" :key="choice.path" class="tick-list-settings-detail">
          <input
            type="checkbox"
            :checked="config.detailColumns.includes(choice.path)"
            :disabled="detailDisabled(choice.path)"
            @change="toggleDetail(choice.path, ($event.target as HTMLInputElement).checked)"
          />
          <span>{{ choice.label }}</span>
        </label>
      </div>
      <small>Up to {{ MAX_TICK_LIST_DETAILS }}, shown under the label in the order you tick them (e.g. quantity then unit: "550 g").</small>
    </div>

    <label for="tick-list-image">Image</label>
    <div>
      <select id="tick-list-image" :value="config.imageColumn" @change="update({ imageColumn: ($event.target as HTMLSelectElement).value })">
        <option value="">No image</option>
        <option v-for="choice in imageChoices" :key="choice.path" :value="choice.path">{{ choice.label }}</option>
      </select>
      <small v-if="imageChoices.length === 0">Neither this table nor the rows it refers to have an image column yet.</small>
      <small v-else>A picture on each tile, e.g. the ingredient's photo on a shopping list.</small>
    </div>

    <span class="fake-label">Sort by</span>
    <div class="tick-list-settings-sorts">
      <select
        v-for="index in sortSlots"
        :key="index"
        :value="config.sortColumns[index - 1] ?? ''"
        :aria-label="index === 1 ? 'Sort by' : `Then by (${index})`"
        @change="setSort(index - 1, ($event.target as HTMLSelectElement).value)"
      >
        <option value="">{{ index === 1 ? 'Table order' : 'No more' }}</option>
        <option v-for="choice in allChoices" :key="choice.path" :value="choice.path">{{ choice.label }}</option>
      </select>
    </div>

    <label for="tick-list-group">Group by</label>
    <div>
      <select id="tick-list-group" :value="config.groupColumn" @change="update({ groupColumn: ($event.target as HTMLSelectElement).value })">
        <option value="">No groups</option>
        <option v-for="choice in allChoices" :key="choice.path" :value="choice.path">{{ choice.label }}</option>
      </select>
      <small>Tiles appear under a heading for each value, e.g. by aisle.</small>
    </div>

    <span class="fake-label">Progress</span>
    <div>
      <label class="tick-list-settings-detail">
        <input
          type="checkbox"
          :checked="config.showProgress"
          @change="update({ showProgress: ($event.target as HTMLInputElement).checked })"
        />
        <span>Show how many items are ticked</span>
      </label>
      <small>"12 of 30 ticked", with a progress bar above the tiles.</small>
    </div>
  </div>
</template>

<style scoped>
/* Sits in one cell of the view form, so it lays out its own label / control rows. */
.tick-list-settings {
  display: grid;
  grid-template-columns: max-content minmax(0, 1fr);
  gap: 0.75rem 1rem;
  align-items: start;
}

.tick-list-settings > div > select {
  display: block;
  max-width: 100%;
}

.tick-list-settings small {
  display: block;
  margin-top: 0.25rem;
}

.tick-list-settings-details,
.tick-list-settings-sorts {
  display: flex;
  flex-wrap: wrap;
  gap: 0.25rem 1rem;
}

.tick-list-settings-detail {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  font-weight: normal;
}

.tick-list-settings-detail input {
  margin: 0;
}
</style>
