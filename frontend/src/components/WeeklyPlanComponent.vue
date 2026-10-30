<script setup lang="ts">
// The WeeklyPlan view type: a week of days split into the view's slots, with the table's rows
// placed in them (see utils/weeklyPlan.ts). Entries can be added to a slot, removed, and dragged
// to another slot. For a relation, new entries are picked in a dialog of image tiles.
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import Section from 'picocrank/vue/components/Section.vue'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { Add01Icon, Calendar03Icon, Cancel01Icon, Copy01Icon, Delete02Icon, LinkSquare02Icon } from '@hugeicons/core-free-icons'
import { createApiClient } from '../stores/api'
import { useTableAccess } from '../composables/useTableAccess'
import { useDesignMode } from '../composables/useDesignMode'
import ViewsButton from './ViewsButton.vue'
import ImageThumb from './ImageThumb.vue'
import RowPickerDialog from './RowPickerDialog.vue'
import ContextMenu from './ContextMenu.vue'
import type { ForeignKey, Item } from '../gen/sickrock_pb'
import { getItemNameFieldValue } from '../utils/itemViewSectionTitle'
import { hasImageValue, isImageFieldType } from '../utils/fieldTypes'
import {
  WEEKLY_PLAN_VIEW_TYPE,
  addDays,
  cellKey,
  dayKey,
  daysOfWeek,
  groupIntoCells,
  parseWeeklyPlanConfig,
  placementFields,
  startOfWeek,
  weeklyPlanConfigProblems,
  type WeeklyPlanConfig,
  type WeeklyPlanPickerOption,
} from '../utils/weeklyPlan'

const props = defineProps<{
  tableId: string
  viewId?: number
  viewName?: string | null
}>()

const emit = defineEmits<{
  'view-changed': [viewType: string, viewId?: number]
}>()

const client = createApiClient()
const router = useRouter()
const { designModeEnabled } = useDesignMode()
const { access } = useTableAccess(computed(() => props.tableId))

const config = ref<WeeklyPlanConfig>(parseWeeklyPlanConfig(''))
const rows = ref<Item[]>([])
const itemForeignKey = ref<ForeignKey | null>(null)
const referencedRows = ref<Item[]>([])
const referencedImageColumn = ref('')
const tableColumns = ref<string[]>([])
const loading = ref(true)
const busy = ref(false)
const error = ref<string | null>(null)

const weekStart = ref(startOfWeek(new Date(), 1))
const addingCell = ref<string | null>(null)
const addValue = ref('')
const dragOverCell = ref<string | null>(null)
const picker = ref<InstanceType<typeof RowPickerDialog> | null>(null)
const pickingFor = ref<{ day: Date; slotIndex: number } | null>(null)
const entryMenu = ref<InstanceType<typeof ContextMenu> | null>(null)
const menuRow = ref<Item | null>(null)
let draggedRowId: string | null = null

const problems = computed(() => weeklyPlanConfigProblems(config.value))
const days = computed(() => daysOfWeek(weekStart.value))
const todayKey = dayKey(new Date())
const referencedTable = computed(() => itemForeignKey.value?.referencedTableTcName || itemForeignKey.value?.referencedTable || '')
const cells = computed(() => groupIntoCells(config.value, weekStart.value, rows.value, columnValue))
const plannedCount = computed(() => [...cells.value.values()].reduce((total, cell) => total + cell.length, 0))
const referencedById = computed(() => new Map(referencedRows.value.map(row => [String(row.id), row])))
const pickerOptions = computed<WeeklyPlanPickerOption[]>(() => referencedRows.value
  .map(row => ({ value: String(row.id), label: referencedLabel(row), imageId: referencedImageId(row) }))
  .sort((a, b) => a.label.localeCompare(b.label)))
const pickerTitle = computed(() => {
  const target = pickingFor.value
  if (!target) return 'Choose'
  return `Choose for ${config.value.slots[target.slotIndex]?.label ?? ''}, ${dayHeading(target.day)}`
})

const weekLabel = computed(() => {
  const first = days.value[0]
  const last = days.value[6]
  const short: Intl.DateTimeFormatOptions = { day: 'numeric', month: 'short' }
  return `${first.toLocaleDateString(undefined, short)} – ${last.toLocaleDateString(undefined, { ...short, year: 'numeric' })}`
})

function columnValue(row: Item, column: string): unknown {
  if (column === 'id') return row.id
  return row.additionalFields?.[column]
}

function referencedLabel(row: Item): string {
  const fields = row.additionalFields ?? {}
  return getItemNameFieldValue(row as unknown as Record<string, unknown>)
    || fields.title?.trim()
    || `#${row.id}`
}

function referencedImageId(row: Item | undefined): string {
  const image = row && referencedImageColumn.value ? row.additionalFields?.[referencedImageColumn.value] : ''
  return hasImageValue(image) ? String(image) : ''
}

/** What an entry shows: the referenced row (for a relation) or the item column's own value. */
function entryOf(row: Item) {
  const value = columnValue(row, config.value.itemColumn)
  if (!referencedTable.value) {
    return { label: String(value ?? '').trim() || `#${row.id}`, to: `/table/${props.tableId}/${row.id}`, imageId: '' }
  }
  const target = referencedById.value.get(String(value ?? ''))
  return {
    label: target ? referencedLabel(target) : `#${value ?? '?'}`,
    to: target ? `/table/${referencedTable.value}/${target.id}` : `/table/${props.tableId}/${row.id}`,
    imageId: referencedImageId(target),
  }
}

async function loadConfig() {
  const res = await client.getTableViews({ tableName: props.tableId })
  const view = res.views.find(v => v.id === props.viewId)
    ?? res.views.find(v => v.viewType === WEEKLY_PLAN_VIEW_TYPE)
  config.value = parseWeeklyPlanConfig(view?.layoutJson)
  weekStart.value = startOfWeek(weekStart.value, config.value.weekStartsOn)
}

async function loadReferencedTable() {
  itemForeignKey.value = null
  referencedRows.value = []
  referencedImageColumn.value = ''
  const structure = await client.getTableStructure({ pageId: props.tableId })
  tableColumns.value = (structure.fields ?? []).map(field => field.name)
  const fk = structure.foreignKeys?.find(f => f.columnName === config.value.itemColumn) ?? null
  itemForeignKey.value = fk
  const refTable = fk ? (fk.referencedTableTcName || fk.referencedTable) : ''
  if (!refTable) return
  const [refStructure, refItems] = await Promise.all([
    client.getTableStructure({ pageId: refTable }),
    client.listItems({ tcName: refTable }),
  ])
  referencedImageColumn.value = refStructure.fields?.find(f => isImageFieldType(f.type))?.name ?? ''
  referencedRows.value = refItems.items ?? []
}

async function loadRows() {
  const res = await client.listItems({ tcName: props.tableId })
  rows.value = res.items ?? []
}

async function load() {
  loading.value = true
  error.value = null
  try {
    await loadConfig()
    if (problems.value.length === 0) {
      await Promise.all([loadReferencedTable(), loadRows()])
    }
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

/** Runs a change, then reloads the plan's rows; failures are shown above the grid. */
async function change(action: () => Promise<unknown>) {
  busy.value = true
  error.value = null
  try {
    await action()
    await loadRows()
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    busy.value = false
  }
}

function startAdding(day: Date, slotIndex: number) {
  if (referencedTable.value) {
    pickingFor.value = { day, slotIndex }
    picker.value?.open()
    return
  }
  addingCell.value = cellKey(day, slotIndex)
  addValue.value = ''
}

function cancelAdding() {
  addingCell.value = null
  addValue.value = ''
}

async function addEntry(day: Date, slotIndex: number, value = addValue.value.trim()) {
  if (!value) return
  cancelAdding()
  await change(() => client.createItem({
    pageId: props.tableId,
    additionalFields: { ...placementFields(config.value, day, slotIndex), [config.value.itemColumn]: value },
  }))
}

async function onPicked(option: WeeklyPlanPickerOption) {
  const target = pickingFor.value
  pickingFor.value = null
  if (target) await addEntry(target.day, target.slotIndex, option.value)
}

// Columns the database fills in itself, so a duplicate leaves them out.
const GENERATED_COLUMNS = new Set(['id', 'sr_created', 'sr_updated'])

/** Adds a copy of the row (same day, slot and contents), which appears in the same cell. */
async function duplicateEntry(row: Item) {
  const fields = Object.fromEntries(tableColumns.value
    .filter(column => !GENERATED_COLUMNS.has(column))
    .map(column => [column, row.additionalFields?.[column] ?? ''])
    .filter(([, value]) => value !== ''))
  await change(() => client.createItem({ pageId: props.tableId, additionalFields: fields }))
}

function openEntryMenu(row: Item, event: MouseEvent) {
  menuRow.value = row
  entryMenu.value?.open(event)
}

async function removeEntry(row: Item) {
  await change(() => client.deleteItem({ pageId: props.tableId, id: String(row.id) }))
}

function onDragStart(row: Item, event: DragEvent) {
  draggedRowId = String(row.id)
  event.dataTransfer?.setData('text/plain', draggedRowId)
  if (event.dataTransfer) event.dataTransfer.effectAllowed = 'move'
}

function onDragOver(key: string, event: DragEvent) {
  if (!draggedRowId) return
  event.preventDefault()
  dragOverCell.value = key
}

async function onDrop(day: Date, slotIndex: number) {
  const rowId = draggedRowId
  draggedRowId = null
  dragOverCell.value = null
  if (!rowId) return
  if (cells.value.get(cellKey(day, slotIndex))?.some(row => String(row.id) === rowId)) return
  await change(() => client.editItem({
    pageId: props.tableId,
    id: rowId,
    additionalFields: placementFields(config.value, day, slotIndex),
  }))
}

function onDragEnd() {
  draggedRowId = null
  dragOverCell.value = null
}

function dayHeading(day: Date): string {
  return day.toLocaleDateString(undefined, { weekday: 'short', day: 'numeric', month: 'short' })
}

/** Today, weekend and past days are shaded as in the calendar view. */
function dayClasses(day: Date) {
  const key = dayKey(day)
  return {
    today: key === todayKey,
    weekend: day.getDay() === 0 || day.getDay() === 6,
    past: key < todayKey,
  }
}

watch(() => [props.tableId, props.viewId], load)
onMounted(load)
</script>

<template>
  <Section :title="viewName || tableId" :icon="Calendar03Icon" :padding="false" class="weekly-plan-section">
    <template #toolbar>
      <ViewsButton
        :table-id="tableId"
        :show-view-create="access.canEdit"
        :show-view-edit="access.canEdit"
        @view-changed="(viewType: string, viewId?: number) => emit('view-changed', viewType, viewId)"
      />
      <router-link
        v-if="designModeEnabled"
        :to="`/table/${tableId}/column-types`"
        class="button neutral design-only"
      >Structure</router-link>
      <div class="weekly-plan-toolbar-nav flex-row g1" role="group" aria-label="Week navigation">
        <span v-if="busy" class="subtle">Saving…</span>
        <button type="button" class="button neutral" @click="weekStart = startOfWeek(new Date(), config.weekStartsOn)">Today</button>
        <button type="button" class="button neutral" title="Previous week" aria-label="Previous week" @click="weekStart = addDays(weekStart, -7)">‹</button>
        <strong class="weekly-plan-week-label">{{ weekLabel }}</strong>
        <button type="button" class="button neutral" title="Next week" aria-label="Next week" @click="weekStart = addDays(weekStart, 7)">›</button>
      </div>
    </template>

    <p v-if="loading" class="padding">Loading…</p>
    <div v-else-if="problems.length > 0" class="padding">
      <NotificationBlock type="warning" message="This weekly plan view is not set up yet." />
      <ul>
        <li v-for="problem in problems" :key="problem">{{ problem }}</li>
      </ul>
      <router-link v-if="viewId && viewId > 0 && access.canEdit" :to="`/table/${tableId}/edit-view/${viewId}`" class="button neutral">
        Edit view settings
      </router-link>
    </div>
    <template v-else>
      <div v-if="error" class="padding">
        <NotificationBlock type="error" :message="error" />
      </div>
      <!-- Wide screens: slots are rows and days are columns, placed with --row/--col.
           Phones: the same elements in source order, one day after another. -->
      <div class="weekly-plan-container">
        <div class="weekly-plan">
          <div class="weekly-plan-corner" aria-hidden="true"></div>
          <div
            v-for="(slot, slotIndex) in config.slots"
            :key="`slot-${slotIndex}`"
            class="weekly-plan-slot-heading"
            :style="{ '--row': slotIndex + 2 }"
          >
            <span class="weekly-plan-slot-name">{{ slot.label }}</span>
            <span v-if="slot.time" class="weekly-plan-slot-time">{{ slot.time }}</span>
          </div>

          <template v-for="(day, dayIndex) in days" :key="dayKey(day)">
            <h3 class="weekly-plan-day-heading" :class="dayClasses(day)" :style="{ '--col': dayIndex + 2 }">
              <span class="weekly-plan-weekday">{{ day.toLocaleDateString(undefined, { weekday: 'short' }) }}</span>
              <span class="weekly-plan-date">{{ day.toLocaleDateString(undefined, { day: 'numeric', month: 'short' }) }}</span>
            </h3>
            <div
              v-for="(slot, slotIndex) in config.slots"
              :key="`${dayKey(day)}-${slotIndex}`"
              class="weekly-plan-cell"
              :class="{ ...dayClasses(day), 'drop-target': dragOverCell === cellKey(day, slotIndex) }"
              :style="{ '--col': dayIndex + 2, '--row': slotIndex + 2 }"
              @dragover="onDragOver(cellKey(day, slotIndex), $event)"
              @dragleave="dragOverCell = null"
              @drop.prevent="onDrop(day, slotIndex)"
            >
              <div class="weekly-plan-cell-slot">{{ slot.label }}</div>
              <div
                v-for="row in cells.get(cellKey(day, slotIndex)) ?? []"
                :key="String(row.id)"
                class="weekly-plan-entry"
                :draggable="access.canEdit"
                @dragstart="onDragStart(row, $event)"
                @dragend="onDragEnd"
                @contextmenu="openEntryMenu(row, $event)"
              >
                <!-- Images and links drag themselves by default; the whole entry should move instead. -->
                <ImageThumb
                  v-if="entryOf(row).imageId"
                  :image-id="entryOf(row).imageId"
                  size="tile"
                  class="weekly-plan-entry-image"
                  draggable="false"
                />
                <router-link :to="entryOf(row).to" class="weekly-plan-entry-label" draggable="false">{{ entryOf(row).label }}</router-link>
                <button
                  v-if="access.canDelete"
                  type="button"
                  class="weekly-plan-remove"
                  :disabled="busy"
                  :title="`Remove ${entryOf(row).label} from ${slot.label}`"
                  @click="removeEntry(row)"
                >
                  <HugeiconsIcon :icon="Cancel01Icon" width="0.9em" height="0.9em" aria-hidden="true" />
                </button>
              </div>

              <form
                v-if="addingCell === cellKey(day, slotIndex)"
                class="weekly-plan-add"
                @submit.prevent="addEntry(day, slotIndex)"
              >
                <input
                  v-model="addValue"
                  type="text"
                  :aria-label="`Add to ${slot.label}, ${dayHeading(day)}`"
                  @keyup.escape="cancelAdding"
                />
                <button type="button" class="button neutral" @click="cancelAdding">Cancel</button>
              </form>
              <button
                v-else-if="access.canInsert"
                type="button"
                class="weekly-plan-add-button"
                :disabled="busy"
                :title="`Add to ${slot.label}, ${dayHeading(day)}`"
                @click="startAdding(day, slotIndex)"
              >
                <HugeiconsIcon :icon="Add01Icon" width="1.25em" height="1.25em" aria-hidden="true" />
              </button>
            </div>
          </template>
        </div>
      </div>
      <div class="weekly-plan-footer subtle">
        <span>{{ plannedCount }} {{ plannedCount === 1 ? 'entry' : 'entries' }} planned this week.</span>
        <span v-if="access.canEdit">Drag an entry to move it, or right-click it to duplicate or remove it.</span>
      </div>
      <ContextMenu ref="entryMenu" label="Entry actions">
        <template v-if="menuRow">
          <li v-if="access.canInsert">
            <button type="button" role="menuitem" class="inline-icon" :disabled="busy" @click="duplicateEntry(menuRow)">
              <HugeiconsIcon :icon="Copy01Icon" width="1em" height="1em" aria-hidden="true" />
              <span>Duplicate</span>
            </button>
          </li>
          <li>
            <button type="button" role="menuitem" class="inline-icon" @click="router.push(entryOf(menuRow).to)">
              <HugeiconsIcon :icon="LinkSquare02Icon" width="1em" height="1em" aria-hidden="true" />
              <span>Open</span>
            </button>
          </li>
          <li v-if="access.canDelete">
            <button type="button" role="menuitem" class="inline-icon" :disabled="busy" @click="removeEntry(menuRow)">
              <HugeiconsIcon :icon="Delete02Icon" width="1em" height="1em" aria-hidden="true" />
              <span>Remove</span>
            </button>
          </li>
        </template>
      </ContextMenu>
      <RowPickerDialog ref="picker" :title="pickerTitle" :options="pickerOptions" @pick="onPicked" />
    </template>
  </Section>
</template>

<style scoped>
/* Follows the calendar view: picocrank's --calendar-* colours, borders and shading. */
/* On narrow screens the week navigation drops below the title rather than overlapping it. */
.weekly-plan-section :deep(.section-header),
.weekly-plan-section :deep([role='toolbar']) {
  flex-wrap: wrap;
}

.weekly-plan-section :deep([role='toolbar'] > .weekly-plan-toolbar-nav) {
  flex: 0 0 auto;
  width: fit-content;
  align-items: center;
}

.weekly-plan-week-label {
  white-space: nowrap;
}

/* Flush with the section: its header's bottom border is the grid's top edge, and the
   section's own frame is the outer edge, so the grid only draws the lines between cells. */
.weekly-plan-container {
  margin: 0;
  overflow-x: auto;
  background: var(--calendar-surface-bg);
  border: 0;
  border-radius: 0;
}

/* The 1px gaps over a border-coloured background draw the grid lines. */
.weekly-plan {
  display: grid;
  grid-template-columns: max-content repeat(7, minmax(8.5rem, 1fr));
  gap: 1px;
  background: var(--calendar-border);
}

.weekly-plan > * {
  margin: 0;
  background: var(--calendar-surface-bg);
}

.weekly-plan-corner,
.weekly-plan-day-heading,
.weekly-plan-slot-heading {
  background: var(--calendar-chrome-bg);
  color: var(--calendar-muted-fg);
}

.weekly-plan-corner {
  grid-row: 1;
  grid-column: 1;
}

.weekly-plan-day-heading {
  grid-row: 1;
  grid-column: var(--col);
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 0.6rem 0.5rem;
  font-size: 1rem;
  font-weight: 600;
}

.weekly-plan-date {
  font-size: 0.8rem;
  font-weight: normal;
}

.weekly-plan-day-heading.today {
  background: var(--calendar-today-bg);
  color: var(--calendar-day-fg);
}

.weekly-plan-slot-heading {
  grid-row: var(--row);
  grid-column: 1;
  display: flex;
  flex-direction: column;
  padding: 0.5rem 0.75rem;
  font-weight: 600;
}

.weekly-plan-slot-time {
  font-size: 0.75rem;
  font-weight: normal;
}

.weekly-plan-cell {
  grid-row: var(--row);
  grid-column: var(--col);
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  min-height: 6rem;
  padding: 0.45rem;
  transition: background-color 0.2s ease;
}

.weekly-plan-cell:hover {
  background: var(--calendar-day-hover-bg);
}

.weekly-plan-cell.weekend {
  background: var(--calendar-day-muted-bg);
}

.weekly-plan-cell.weekend:hover {
  background: var(--calendar-day-muted-hover-bg);
}

.weekly-plan-cell.today {
  background: var(--calendar-today-bg);
}

.weekly-plan-cell.today:hover {
  background: var(--calendar-today-hover-bg);
}

.weekly-plan-cell.past,
.weekly-plan-day-heading.past {
  opacity: 0.6;
}

.weekly-plan-cell.past:hover {
  opacity: 0.8;
}

.weekly-plan-cell.drop-target {
  background: var(--calendar-range-bg-subtle);
  outline: 2px dashed var(--calendar-range-accent);
  outline-offset: -3px;
}

/* Below the grid, so the section's rounded bottom corners show. */
.weekly-plan-footer {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: 0.25rem 1rem;
  padding: 0.6rem 1rem;
  border-top: 1px solid var(--calendar-border);
  font-size: 0.85rem;
}

/* The slot name inside a cell is only needed when the slot headings are hidden (phones). */
.weekly-plan-cell-slot {
  display: none;
  font-size: 0.8rem;
  color: var(--calendar-muted-fg);
}

/* An entry looks like a calendar event, with its image (if any) across the top. */
.weekly-plan-entry {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.25rem;
  padding: 0.25rem 0.5rem;
  overflow: hidden;
  background: var(--calendar-event-bg);
  border: 1px solid var(--calendar-event-border);
  border-radius: 4px;
  box-shadow: var(--calendar-event-shadow);
  transition: background-color 0.2s, box-shadow 0.2s;
}

.weekly-plan-entry:hover {
  background: var(--calendar-event-hover-bg);
  box-shadow: var(--calendar-event-hover-shadow);
}

.weekly-plan-entry[draggable="true"] {
  cursor: grab;
}

.weekly-plan-entry[draggable="true"]:active {
  cursor: grabbing;
}

.weekly-plan-entry-image {
  grid-column: 1 / -1;
  width: calc(100% + 1rem);
  max-height: 8rem;
  margin: -0.25rem -0.5rem 0;
  border-radius: 0;
}

.weekly-plan-entry-label {
  min-width: 0;
  overflow-wrap: anywhere;
  font-size: 0.85rem;
  font-weight: bold;
  color: var(--calendar-event-title-fg);
  text-decoration: none;
}

.weekly-plan-entry-label:hover {
  text-decoration: underline;
}

.weekly-plan-remove {
  margin: 0;
  padding: 0.15rem 0.35rem;
  line-height: 1;
}

.weekly-plan-remove {
  background: none;
  border: 0;
  color: var(--calendar-event-title-fg);
  opacity: 0.7;
}

.weekly-plan-remove:hover {
  opacity: 1;
}

/* A full-width target at the foot of the cell, so the buttons line up along each slot row. */
.weekly-plan-add-button {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  margin: auto 0 0;
  padding: 0.5rem;
  line-height: 1;
  font-size: 1.1rem;
  background: none;
  border: 1px dashed var(--calendar-border);
  border-radius: 4px;
  color: var(--calendar-muted-fg);
  cursor: pointer;
  opacity: 0.7;
}

.weekly-plan-add-button:hover:not(:disabled) {
  background: var(--calendar-range-bg-subtle);
  border-color: var(--calendar-range-accent);
  color: var(--calendar-range-accent);
}

.weekly-plan-cell:hover .weekly-plan-add-button,
.weekly-plan-add-button:focus-visible {
  opacity: 1;
}

.weekly-plan-add {
  display: flex;
  flex-wrap: wrap;
  gap: 0.25rem;
}

.weekly-plan-add input {
  flex: 1;
  min-width: 0;
}

/* Phones: one day after another, each day's heading followed by its slots. */
@media (max-width: 800px) {
  .weekly-plan {
    grid-template-columns: minmax(0, 1fr);
  }

  .weekly-plan-corner,
  .weekly-plan-slot-heading {
    display: none;
  }

  .weekly-plan-day-heading,
  .weekly-plan-cell {
    grid-row: auto;
    grid-column: auto;
  }

  .weekly-plan-day-heading {
    flex-direction: row;
    justify-content: center;
    gap: 0.4rem;
    align-items: baseline;
  }

  .weekly-plan-cell {
    min-height: 3.5rem;
  }

  .weekly-plan-cell-slot {
    display: block;
  }
}

@media (prefers-reduced-motion: reduce) {
  .weekly-plan-cell,
  .weekly-plan-entry {
    transition: none;
  }
}
</style>
