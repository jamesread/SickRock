<script setup lang="ts">
// Picks a row of another table, as image tiles (name, image and details) or as a list (name, a few
// columns and the id), with a search over the name and details. Call open(); the chosen option is
// emitted as "pick" (allowNone adds a None choice, emitted with an empty value). With an empty
// search, recentValues are offered first. With createLabel and createTableId set, a create button
// swaps the options for that table's insert form (name filled in from the search), and the new row
// is emitted as "created". See utils/rowPicker.ts and composables/useRowPicker.ts.
import { computed, defineAsyncComponent, nextTick, ref } from 'vue'
import Dialog from 'picocrank/vue/components/Dialog.vue'
import Table from 'picocrank/vue/components/Table.vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { Add01Icon, Image01Icon } from '@hugeicons/core-free-icons'
import FormLayout from 'picocrank/vue/components/FormLayout.vue'
import FormField from 'picocrank/vue/components/FormField.vue'
import ImageThumb from './ImageThumb.vue'
import type { DialogApi } from '../utils/dialog'
import type { Item } from '../gen/sickrock_pb'
import type { InsertFieldDef } from '../utils/insertRowViewFields'
import { filterPickerOptions, recentOptions, type RowPickerLayout, type RowPickerOption } from '../utils/rowPicker'

// InsertRow uses this dialog for its relation fields, so it is loaded lazily to break the cycle.
const InsertRow = defineAsyncComponent(() => import('./InsertRow.vue'))

const props = withDefaults(defineProps<{
  title: string
  options: RowPickerOption[]
  layout?: RowPickerLayout
  /** The current choice, marked in the options. */
  selectedValue?: string
  /** Offers a None choice (emitted with an empty value), for optional fields. */
  allowNone?: boolean
  /** Values picked recently, newest first; offered first while the search is empty. */
  recentValues?: string[]
  loading?: boolean
  createLabel?: string
  createTableId?: string
  createFieldDefs?: InsertFieldDef[]
}>(), {
  layout: 'tiles',
  selectedValue: '',
  allowNone: false,
  recentValues: () => [],
  loading: false,
  createLabel: '',
  createTableId: '',
  createFieldDefs: () => [],
})

const emit = defineEmits<{
  pick: [option: RowPickerOption]
  created: [item: Item | undefined]
}>()

/** Most tiles shown at once; past this, the search narrows them down. */
const MAX_TILES = 200
/** Rows per page of the list layout. */
const LIST_PAGE_SIZE = 50

const NONE_OPTION: RowPickerOption = { value: '', label: 'None', imageId: '' }

const dialogEl = ref<DialogApi | null>(null)
const searchId = `row-picker-search-${Math.random().toString(36).slice(2)}`
const searchInput = ref<HTMLInputElement | null>(null)
const query = ref('')
const creating = ref(false)
// The search text open() started with; it is a filter, not a name for a new row.
const initialSearch = ref('')
// The option arrow keys have moved to, which Enter picks.
const activeIndex = ref(0)
// The list layout's column filters and page (picocrank Table), and the rows its filters leave.
const listFilters = ref<Record<string, unknown>>({})
const listPage = ref(1)
const listFiltered = ref<ListRow[] | null>(null)
const listFiltersActive = ref(false)

const isList = computed(() => props.layout === 'list')
const searchMatches = computed(() => filterPickerOptions(props.options, query.value))
const optionsByValue = computed(() => new Map(props.options.map(option => [option.value, option])))
// The options left by the search and, in the list layout, the column filters.
const matches = computed(() => {
  if (!isList.value || !listFiltered.value) return searchMatches.value
  return listFiltered.value.flatMap(row => optionsByValue.value.get(row.value) ?? [])
})
// Narrowing by search or by column filter hides None and the recent picks.
const searching = computed(() => query.value.trim() !== '' || (isList.value && listFiltersActive.value))
const recent = computed(() => (searching.value ? [] : recentOptions(props.options, props.recentValues)))
const noneChoice = computed(() => (props.allowNone && !searching.value ? [NONE_OPTION] : []))
const tiles = computed(() => matches.value.slice(0, MAX_TILES))
// Everything Enter and the arrow keys move through, in display order.
const choices = computed(() => [...noneChoice.value, ...recent.value, ...(isList.value ? matches.value : tiles.value)])
const activeValue = computed(() => choices.value[activeIndex.value]?.value)
// The tiles in display order: None first, then the recent picks and everything else under
// headings, or just everything when nothing was picked recently.
const tileGroups = computed(() => (recent.value.length
  ? [
      { key: 'recent', heading: 'Recent', items: [...noneChoice.value, ...recent.value] },
      { key: 'all', heading: 'All', items: tiles.value },
    ]
  : [{ key: 'all', heading: '', items: [...noneChoice.value, ...tiles.value] }]
).filter(group => group.items.length > 0))
const countText = computed(() => {
  const total = props.options.length
  if (!isList.value && matches.value.length > MAX_TILES) {
    return `Showing ${MAX_TILES} of ${matches.value.length} matches: keep typing to narrow them down.`
  }
  if (searching.value) return `${matches.value.length} of ${total} match.`
  return total === 1 ? '1 to choose from.' : `${total} to choose from.`
})
// Names the fields the search covers: the name, plus the options' details.
const searchPlaceholder = computed(() => {
  const labels = [...new Set(props.options.flatMap(option => (option.details ?? []).map(detail => detail.label.toLowerCase())))]
  return `Search ${['name', ...labels].join(', ')}…`
})
const canCreate = computed(() => Boolean(props.createLabel && props.createTableId))
// The search text becomes the new row's name, when the table has a name column.
const createInitialValues = computed(() => {
  const name = query.value.trim()
  return name && name !== initialSearch.value.trim() && props.createFieldDefs.some(field => field.name === 'name') ? { name } : null
})

// The list layout's columns: the name, each detail label, and the id.
const detailLabels = computed(() => [...new Set(props.options.flatMap(option => (option.details ?? []).map(detail => detail.label)))])
const listHeaders = computed(() => [
  { key: 'label', label: 'Name', sortable: false },
  ...detailLabels.value.map((label, index) => ({ key: `detail${index}`, label, sortable: false })),
  { key: 'value', label: 'ID', sortable: false },
])

type ListRow = Record<string, string> & { value: string; label: string }

function listRows(options: readonly RowPickerOption[]): ListRow[] {
  return options.map(option => {
    const row: ListRow = { value: option.value, label: option.label }
    detailLabels.value.forEach((label, index) => {
      row[`detail${index}`] = option.details?.find(detail => detail.label === label)?.value ?? ''
    })
    return row
  })
}

function optionFor(row: { value: string }): RowPickerOption {
  return optionsByValue.value.get(row.value) ?? NONE_OPTION
}

// Every search match goes to the list's Table, which applies its own column filters.
const listAllRows = computed(() => listRows(searchMatches.value))

function onListFiltered(rows: ListRow[], { filtersActive }: { filtersActive: boolean }) {
  listFiltered.value = rows
  listFiltersActive.value = filtersActive
}

function listRowClass({ row }: { row: ListRow }) {
  return {
    'row-picker-active': row.value === activeValue.value,
    'row-picker-selected': row.value === props.selectedValue,
  }
}

/** Opens the options, with the search already holding initialQuery (e.g. what was typed in the field). */
async function open(initialQuery = '') {
  query.value = initialQuery
  initialSearch.value = initialQuery
  creating.value = false
  activeIndex.value = 0
  listFilters.value = {}
  listPage.value = 1
  dialogEl.value?.show()
  await nextTick()
  searchInput.value?.focus()
  // Selected, so typing replaces it; arrow keys keep it to refine.
  searchInput.value?.select()
}

function close() {
  dialogEl.value?.close()
}

function pick(option: RowPickerOption) {
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

function onQueryInput() {
  activeIndex.value = 0
}

function onListFilters(filters: Record<string, unknown>) {
  listFilters.value = filters
  activeIndex.value = 0
}

/** Enter in the search box picks the highlighted option. */
function pickActive() {
  const option = choices.value[activeIndex.value]
  if (option) pick(option)
}

async function moveActive(step: number) {
  const count = choices.value.length
  if (count === 0) return
  activeIndex.value = (activeIndex.value + step + count) % count
  // In the list, turn to the page holding the highlighted row.
  const listIndex = activeIndex.value - noneChoice.value.length - recent.value.length
  if (isList.value && listIndex >= 0) listPage.value = Math.floor(listIndex / LIST_PAGE_SIZE) + 1
  await nextTick()
  document.querySelector('.row-picker-dialog .row-picker-active')?.scrollIntoView({ block: 'nearest' })
}

defineExpose({ open, close })
</script>

<template>
  <Dialog
    ref="dialogEl"
    :title="creating ? createLabel : title"
    size="large"
    narrow-layout="fullscreen"
    class="row-picker-dialog"
  >

    <template v-if="creating && createTableId">
      <!-- Cancel returns to the options. -->
      <InsertRow
        class="row-picker-create"
        :table-id="createTableId"
        :field-defs="createFieldDefs"
        :initial-values="createInitialValues"
        cancellable
        @created="onCreated"
        @cancelled="stopCreating"
      />
    </template>
    <template v-else>
      <!-- Labelled fields: anything the caller asks for alongside the pick (FormFields in the
           before-search slot, e.g. who a planned meal is for), then the search. -->
      <FormLayout class="row-picker-fields" @submit.prevent>
        <slot name="before-search" />
        <FormField label="Search" :for="searchId">
          <div class="row-picker-toolbar">
          <input
            :id="searchId"
            ref="searchInput"
            v-model="query"
            type="text"
            enterkeyhint="search"
            autocomplete="off"
            :placeholder="searchPlaceholder"
            @input="onQueryInput"
            @keydown.enter.prevent="pickActive"
            @keydown.down.prevent="moveActive(1)"
            @keydown.up.prevent="moveActive(-1)"
          />
          <button v-if="canCreate" type="button" class="button inline-icon good" @click="startCreating">
            <HugeiconsIcon :icon="Add01Icon" width="1em" height="1em" aria-hidden="true" />
            <span>{{ createLabel }}</span>
          </button>
          </div>
        </FormField>
      </FormLayout>
      <p class="subtle row-picker-count" aria-live="polite">{{ loading ? 'Loading…' : countText }}</p>

      <template v-if="!loading">
        <!-- Tiles: name, image and details. -->
        <template v-if="layout === 'tiles'">
          <template v-for="group in tileGroups" :key="group.key">
            <h3 v-if="group.heading" class="row-picker-heading">{{ group.heading }}</h3>
            <ul class="row-picker-tiles">
              <li v-for="option in group.items" :key="`${group.key}-${option.value}`">
                <button
                  type="button"
                  class="row-picker-tile"
                  :class="{ 'row-picker-active': option.value === activeValue, 'row-picker-selected': option.value === selectedValue }"
                  :aria-current="option.value === selectedValue ? 'true' : undefined"
                  @click="pick(option)"
                >
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
          </template>
        </template>

        <!-- List: name, a few columns and the id, paged, with picocrank's column filters and options. -->
        <template v-else>
          <div v-if="noneChoice.length" class="row-picker-none">
            <button
              type="button"
              class="neutral"
              :class="{ 'row-picker-active': activeValue === '' }"
              :aria-current="selectedValue === '' ? 'true' : undefined"
              @click="pick(NONE_OPTION)"
            >
              None
            </button>
          </div>
          <template v-if="recent.length">
            <h3 class="row-picker-heading">Recent</h3>
            <Table
              class="row-picker-list"
              :data="listRows(recent)"
              :headers="listHeaders"
              row-key="value"
              :filterable="false"
              :column-options="false"
              :show-pagination="false"
              :row-class="listRowClass"
              @row-click="(row: ListRow) => pick(optionFor(row))"
            >
              <template #cell-label="{ row }">
                <button
                  type="button"
                  class="row-picker-list-pick"
                  :aria-current="row.value === selectedValue ? 'true' : undefined"
                  @click="pick(optionFor(row))"
                >
                  {{ row.label }}
                </button>
              </template>
              <template #cell-value="{ row }">
                <span class="subtle">#{{ row.value }}</span>
              </template>
            </Table>
            <h3 class="row-picker-heading">All</h3>
          </template>
          <!-- Kept while a column filter hides every row, so the filter can be cleared. -->
          <Table
            class="row-picker-list"
            v-if="searchMatches.length > 0"
            v-model:page="listPage"
            :data="listAllRows"
            :headers="listHeaders"
            row-key="value"
            :filters="listFilters"
            :page-size="LIST_PAGE_SIZE"
            :show-pagination="searchMatches.length > LIST_PAGE_SIZE"
            :row-class="listRowClass"
            @row-click="(row: ListRow) => pick(optionFor(row))"
            @update:filters="onListFilters"
            @filtered-rows-change="onListFiltered"
          >
              <template #cell-label="{ row }">
                <button
                  type="button"
                  class="row-picker-list-pick"
                  :aria-current="row.value === selectedValue ? 'true' : undefined"
                  @click="pick(optionFor(row))"
                >
                  {{ row.label }}
                </button>
              </template>
              <template #cell-value="{ row }">
                <span class="subtle">#{{ row.value }}</span>
              </template>
          </Table>
        </template>

        <p v-if="isList ? searchMatches.length === 0 : choices.length === 0" class="subtle">
          {{ options.length === 0 ? 'There is nothing to choose from yet.' : 'Nothing matches.' }}
        </p>
      </template>
    </template>
  </Dialog>
</template>

<style scoped>
.row-picker-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem 1rem;
  margin-block: 0 0.25rem;
}

.row-picker-toolbar > .button {
  margin: 0;
}

.row-picker-count {
  margin: 0 0 0.75rem;
  font-size: 0.85rem;
}

.row-picker-none {
  margin: 0 0 0.5rem;
}

.row-picker-none .row-picker-active {
  outline: 2px solid currentColor;
  outline-offset: 1px;
}

.row-picker-heading {
  margin: 0.75rem 0 0.5rem;
  font-size: 0.95rem;
}

.row-picker-create {
  margin-top: 0.75rem;
}

.row-picker-fields {
  margin-block-end: 0.25rem;
}

.row-picker-toolbar input {
  flex: 1 1 12rem;
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
.row-picker-tile:focus-visible,
.row-picker-tile.row-picker-active {
  border-color: currentColor;
}

.row-picker-tile.row-picker-selected {
  outline: 2px solid currentColor;
  outline-offset: 1px;
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

/* The name reads as text but is a button, so the rows can be reached with Tab. */
.row-picker-list-pick {
  margin: 0;
  padding: 0;
  border: 0;
  background: none;
  color: inherit;
  font: inherit;
  font-weight: 600;
  text-align: start;
  cursor: pointer;
}

/* The row the arrow keys are on: the theme's menu highlight, on the cells (the table colours those). */
.row-picker-list :deep(tr.row-picker-active > td) {
  background-color: var(--menu-item-hover-background-color, var(--hover-background-color));
}

.row-picker-list :deep(tr.row-picker-selected) .row-picker-list-pick::after {
  content: ' ✓';
}

.visually-hidden {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
}
</style>
