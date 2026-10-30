<script setup lang="ts">
// Picks a row of another table from tiles showing its name and image, with a search on the name.
// Call open(); the chosen option is emitted as "pick". With createLabel and createTableId set, a
// create button swaps the tiles for that table's insert form (name filled in from the search),
// and the new row is emitted as "created".
import { computed, nextTick, ref } from 'vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { Add01Icon, Cancel01Icon, Image01Icon, Search01Icon } from '@hugeicons/core-free-icons'
import ImageThumb from './ImageThumb.vue'
import InsertRow from './InsertRow.vue'
import type { Item } from '../gen/sickrock_pb'
import type { InsertFieldDef } from '../utils/insertRowViewFields'
import { filterPickerOptions, type WeeklyPlanPickerOption } from '../utils/weeklyPlan'

const props = defineProps<{
  title: string
  options: WeeklyPlanPickerOption[]
  createLabel?: string
  createTableId?: string
  createFieldDefs?: InsertFieldDef[]
}>()

const emit = defineEmits<{
  pick: [option: WeeklyPlanPickerOption]
  created: [item: Item | undefined]
}>()

const dialogEl = ref<HTMLDialogElement | null>(null)
const searchInput = ref<HTMLInputElement | null>(null)
const query = ref('')
const creating = ref(false)
// The search text open() started with; it is a filter, not a name for a new row.
const initialSearch = ref('')

const matches = computed(() => filterPickerOptions(props.options, query.value))
// Names the fields the search covers: the name, plus the tiles' details.
const searchPlaceholder = computed(() => {
  const labels = [...new Set(props.options.flatMap(option => (option.details ?? []).map(detail => detail.label.toLowerCase())))]
  return `Search ${['name', ...labels].join(', ')}…`
})
const canCreate = computed(() => Boolean(props.createLabel && props.createTableId))
// The search text becomes the new row's name, when the table has a name column.
const createInitialValues = computed(() => {
  const name = query.value.trim()
  return name && name !== initialSearch.value.trim() && props.createFieldDefs?.some(field => field.name === 'name') ? { name } : null
})

/** Opens the tiles, with the search already holding initialQuery (e.g. a slot's keywords). */
async function open(initialQuery = '') {
  query.value = initialQuery
  initialSearch.value = initialQuery
  creating.value = false
  dialogEl.value?.showModal()
  await nextTick()
  searchInput.value?.focus()
  // Selected, so typing replaces the keywords; arrow keys keep them to refine.
  searchInput.value?.select()
}

function close() {
  dialogEl.value?.close()
}

function pick(option: WeeklyPlanPickerOption) {
  close()
  emit('pick', option)
}

function startCreating() {
  creating.value = true
}

async function stopCreating() {
  creating.value = false
  await nextTick()
  searchInput.value?.focus()
}

function onCreated(item: Item | undefined) {
  close()
  emit('created', item)
}

/** Enter in the search box picks the first match. */
function pickFirst() {
  const first = matches.value[0]
  if (first) pick(first)
}

defineExpose({ open, close })
</script>

<template>
  <dialog ref="dialogEl" class="dialog row-picker-dialog">
    <div class="row-picker-titlebar">
      <h2>{{ creating ? createLabel : title }}</h2>
      <button type="button" class="button inline-icon neutral" aria-label="Close" @click="close">
        <HugeiconsIcon :icon="Cancel01Icon" width="1.25em" height="1.25em" aria-hidden="true" />
      </button>
    </div>

    <template v-if="creating && createTableId">
      <!-- Cancel returns to the tiles. -->
      <InsertRow
        class="row-picker-create"
        :table-id="createTableId"
        :field-defs="createFieldDefs ?? []"
        :initial-values="createInitialValues"
        cancellable
        @created="onCreated"
        @cancelled="stopCreating"
      />
    </template>
    <template v-else>
      <div class="row-picker-toolbar">
        <label class="row-picker-search">
          <HugeiconsIcon :icon="Search01Icon" width="1em" height="1em" aria-hidden="true" />
          <span class="visually-hidden">{{ searchPlaceholder }}</span>
          <input
            ref="searchInput"
            v-model="query"
            type="text"
            enterkeyhint="search"
            :placeholder="searchPlaceholder"
            @keydown.enter.prevent="pickFirst"
          />
        </label>
        <button v-if="canCreate" type="button" class="button inline-icon good" @click="startCreating">
          <HugeiconsIcon :icon="Add01Icon" width="1em" height="1em" aria-hidden="true" />
          <span>{{ createLabel }}</span>
        </button>
      </div>

      <ul v-if="matches.length > 0" class="row-picker-tiles">
        <li v-for="option in matches" :key="option.value">
          <button type="button" class="row-picker-tile" @click="pick(option)">
            <ImageThumb v-if="option.imageId" :image-id="option.imageId" :alt="option.label" size="tile" />
            <span v-else class="row-picker-no-image" aria-hidden="true">
              <HugeiconsIcon :icon="Image01Icon" width="2rem" height="2rem" />
            </span>
            <span class="row-picker-tile-label">{{ option.label }}</span>
            <span v-if="option.details?.length" class="row-picker-tile-details">
              <span v-for="detail in option.details" :key="detail.label" :title="detail.label">{{ detail.value }}</span>
            </span>
          </button>
        </li>
      </ul>
      <p v-else class="subtle">{{ options.length === 0 ? 'There is nothing to choose from yet.' : `Nothing matches “${query.trim()}”.` }}</p>
    </template>
  </dialog>
</template>

<style scoped>
.row-picker-dialog {
  width: min(52rem, 96vw);
  max-width: none;
  max-height: 90vh;
}

.row-picker-titlebar {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.row-picker-titlebar h2 {
  margin: 0;
}

.row-picker-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem 1rem;
  margin-block: 0.75rem;
}

.row-picker-toolbar > .button {
  margin: 0;
}

.row-picker-create {
  margin-top: 0.75rem;
}

.row-picker-search {
  display: flex;
  flex: 1 1 16rem;
  align-items: center;
  gap: 0.5rem;
}

.row-picker-search input {
  flex: 1;
  min-width: 0;
  margin: 0;
}

.row-picker-tiles {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(9rem, 1fr));
  gap: 0.75rem;
  margin: 0;
  padding: 0;
  list-style: none;
}

.row-picker-tile {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  margin: 0;
  padding: 0.4rem;
  gap: 0.4rem;
  text-align: start;
  border: 1px solid color-mix(in srgb, currentColor 20%, transparent);
  border-radius: 0.5rem;
  background: none;
  color: inherit;
  cursor: pointer;
}

.row-picker-tile:hover,
.row-picker-tile:focus-visible {
  border-color: currentColor;
}

.row-picker-no-image {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  aspect-ratio: 4 / 3;
  border-radius: 0.25rem;
  background: color-mix(in srgb, currentColor 8%, transparent);
  opacity: 0.6;
}

.row-picker-tile-label {
  font-weight: 600;
  overflow-wrap: anywhere;
}

/* Just the values (the field names are left out), one per line. */
.row-picker-tile-details {
  display: flex;
  flex-direction: column;
  gap: 0.1rem;
  font-size: 0.8rem;
  opacity: 0.75;
  overflow-wrap: anywhere;
}

.visually-hidden {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
}
</style>
