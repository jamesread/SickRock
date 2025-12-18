<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import Calendar, { type CalendarEvent } from 'picocrank/vue/components/Calendar.vue'
import Section from 'picocrank/vue/components/Section.vue'
import { createApiClient } from '../stores/api'
import ViewsButton from './ViewsButton.vue'

const props = defineProps<{
  tableId: string
}>()

const emit = defineEmits<{
  'view-changed': [viewType: string]
}>()

const router = useRouter()

type Item = Record<string, unknown>

const items = ref<Item[]>([])
const loading = ref(false)
const error = ref<string | null>(null)

// Inline editing state
const editingItem = ref<{ id: string; value: string } | null>(null)
const editingValue = ref('')
const saving = ref(false)

const selectedDate = ref('')

// Context menu state
const contextMenu = ref<{ visible: boolean; x: number; y: number; item: Item | null }>({
  visible: false,
  x: 0,
  y: 0,
  item: null
})
const deleting = ref(false)

// Quick add state
const quickAdd = ref<{ visible: boolean; date: Date | null; name: string; icon: string }>({
  visible: false,
  date: null,
  name: '',
  icon: ''
})
const creating = ref(false)

// Calendar state
const currentMonth = ref(new Date().getMonth())
const currentYear = ref(new Date().getFullYear())

// Table structure state
const tableStructure = ref<any>(null)
const tableTitle = ref<string>('')

// Check if table has icon field
const hasIconField = computed(() => {
  return tableStructure.value?.fields?.some((f: any) => f.name === 'icon')
})

// Computed property for the section title
const sectionTitle = computed(() => {
  return tableTitle.value || `Table: ${props.tableId}`
})

// Transport handled by authenticated client
const client = createApiClient()

// Helper function to get item value for a column
function getItemValue(item: any, column: string): any {
  // Check standard fields first (id, sr_created, and sr_updated are static now)
  if (column === 'id' || column === 'sr_created' || column === 'sr_updated') {
    return item[column]
  }
  // Check additional fields from protobuf (all other fields including name are dynamic)
  if (item.additionalFields && item.additionalFields[column] !== undefined) {
    return item.additionalFields[column]
  }
  // Fallback to direct property access
  return item[column]
}

// Helper function to parse MySQL datetime/date strings as local time
function parseServerDateTime(dateStr: any): Date | null {
  if (!dateStr) return null

  const str = String(dateStr).trim()

  // Match MySQL datetime format: YYYY-MM-DD HH:MM:SS or YYYY-MM-DD
  const datetimeMatch = str.match(/^(\d{4})-(\d{2})-(\d{2})(?: (\d{2}):(\d{2}):(\d{2}))?$/)

  if (!datetimeMatch) {
    // Fallback to standard Date parsing
    const date = new Date(str)
    return isNaN(date.getTime()) ? null : date
  }

  const [, year, month, day, hour = '0', minute = '0', second = '0'] = datetimeMatch

  // Create date in local timezone by using the Date constructor with components
  const date = new Date(
    parseInt(year, 10),
    parseInt(month, 10) - 1, // Month is 0-indexed
    parseInt(day, 10),
    parseInt(hour, 10),
    parseInt(minute, 10),
    parseInt(second, 10)
  )

  return isNaN(date.getTime()) ? null : date
}

// Convert SickRock items to CalendarEvents
const calendarEvents = computed<CalendarEvent[]>(() => {
  return items.value.map(item => {
    const id = getItemValue(item, 'id')
    const name = getItemValue(item, 'name')
    const starts = getItemValue(item, 'starts')
    const finishes = getItemValue(item, 'finishes')
    const calendarDate = getItemValue(item, 'calendar_date')
    const srCreated = getItemValue(item, 'sr_created')

    let startDate: Date | null = null
    let endDate: Date | null = null
    let date: Date | null = null

    if (calendarDate) {
      date = parseServerDateTime(calendarDate)
    } else if (starts) {
      startDate = parseServerDateTime(starts)
      if (finishes) {
        endDate = parseServerDateTime(finishes)
      }
    } else if (srCreated) {
      date = new Date(Number(srCreated) * 1000)
    }


    return {
      id: String(id),
      title: name ? String(name) : `Item ${id || 'Unknown'}`,
      startDate,
      endDate,
      date,
      _originalItem: item // Keep reference to original item for callbacks
    }
  })
})

const upcomingVisibleEventsCount = computed(() => {
  const now = new Date()
  const monthStart = new Date(currentYear.value, currentMonth.value, 1)
  const nextMonthStart = new Date(currentYear.value, currentMonth.value + 1, 1)

  const nowTime = now.getTime()
  const monthStartTime = monthStart.getTime()
  const nextMonthStartTime = nextMonthStart.getTime()

  return calendarEvents.value.filter(event => {
    const { start, end } = getEventDateRange(event)
    const eventStart = start || (event.date instanceof Date ? event.date : null)
    if (!eventStart) return false

    const eventEnd = end || eventStart
    const eventStartTime = eventStart.getTime()
    const eventEndTime = eventEnd.getTime()

    const isUpcoming = eventEndTime >= nowTime
    const overlapsVisibleMonth =
      eventEndTime >= monthStartTime && eventStartTime < nextMonthStartTime

    return isUpcoming && overlapsVisibleMonth
  }).length
})

const upcomingEventsMessage = computed(() => {
  const count = upcomingVisibleEventsCount.value
  if (count === 0) return 'No upcoming events'
  if (count === 1) return '1 upcoming event'
  return `${count} upcoming events`
})

function getEventTooltip(event: CalendarEvent): string {
  const { start, end } = getEventDateRange(event)
  const reference = start || end
  if (!reference) return ''

  const eventDate = new Date(reference)
  eventDate.setHours(0, 0, 0, 0)

  const now = new Date()
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate())

  const msPerDay = 24 * 60 * 60 * 1000
  const diffDays = Math.max(0, Math.floor((eventDate.getTime() - today.getTime()) / msPerDay))

  return `in ${diffDays} ${diffDays === 1 ? 'day' : 'days'}`
}

// Load data (initial load - resets to today)
async function load() {
  const now = new Date()
  const newMonth = now.getMonth()
  const newYear = now.getFullYear()

  currentMonth.value = newMonth
  currentYear.value = newYear

  selectedDate.value = `${newYear}-${String(newMonth + 1).padStart(2, '0')}`

  await reloadItems()
}

// Reload items without changing the calendar view
async function reloadItems() {
  loading.value = true
  error.value = null
  try {
    // Load items
    const res = await client.listItems({ tcName: props.tableId })
    items.value = Array.isArray(res.items) ? (res.items as Item[]) : []

    // Load table structure (only on first load or if not already loaded)
    if (!tableStructure.value) {
      const structureRes = await client.getTableStructure({ pageId: props.tableId })
      tableStructure.value = structureRes
    }

    // Load table configuration title
    if (!tableTitle.value) {
      try {
        const configs = await client.getTableConfigurations({})
        const config = configs.pages?.find(p => p.id === props.tableId)
        if (config && config.title) {
          tableTitle.value = config.title
        }
      } catch (e) {
        console.warn('Failed to load table configuration title:', e)
      }
    }
  } catch (e) {
    error.value = String(e)
  } finally {
    loading.value = false
  }
}

// Custom date range extractor to avoid date re-parsing issues
function getEventDateRange(event: CalendarEvent): { start: Date | null; end: Date | null } {
  // Return the Date objects directly without re-parsing
  return {
    start: event.startDate instanceof Date ? event.startDate : (event.date instanceof Date ? event.date : null),
    end: event.endDate instanceof Date ? event.endDate : null
  }
}

// Custom event time formatter
function formatEventTime(event: CalendarEvent, date: Date): string {
  const { start, end } = getEventDateRange(event)

  if (!start) return 'No time'

  // Check if start time is midnight (00:00)
  if (start.getHours() === 0 && start.getMinutes() === 0) {
    return 'No time'
  }

  // For events with both start and end (date ranges)
  if (end) {
    const startDateOnly = new Date(start)
    startDateOnly.setHours(0, 0, 0, 0)
    const endDateOnly = new Date(end)
    endDateOnly.setHours(0, 0, 0, 0)
    const targetDateOnly = new Date(date)
    targetDateOnly.setHours(0, 0, 0, 0)

    // Multi-day event logic
    if (startDateOnly.getTime() !== endDateOnly.getTime()) {
      if (targetDateOnly.getTime() === startDateOnly.getTime()) {
        // Start day - show start time
        return start.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
      } else if (targetDateOnly.getTime() === endDateOnly.getTime()) {
        // End day - show end time
        if (end.getHours() === 0 && end.getMinutes() === 0) {
          return 'No time'
        }
        return end.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
      } else {
        // Middle day
        return 'All day'
      }
    }
  }

  // Single-time event - show the start time
  return start.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

// Event handlers
function handleEventClick(event: CalendarEvent) {
  const item = (event as any)._originalItem
  const itemId = getItemValue(item, 'id')
  if (itemId) {
    window.location.href = `/table/${props.tableId}/${itemId}`
  }
}

function handleDateClick(date: Date) {
  showQuickAdd(date)
}

function handleMonthChange(month: number, year: number) {
  currentMonth.value = month
  currentYear.value = year
}

function handleEventContextMenu(event: CalendarEvent, mouseEvent: MouseEvent) {
  const item = (event as any)._originalItem
  contextMenu.value = {
    visible: true,
    x: mouseEvent.clientX,
    y: mouseEvent.clientY,
    item: item
  }
}

// Quick add functions
function showQuickAdd(date: Date) {
  quickAdd.value = {
    visible: true,
    date: date,
    name: '',
    icon: ''
  }
  // Focus the input after the DOM updates
  setTimeout(() => {
    const input = document.querySelector('.quick-add-input') as HTMLInputElement
    if (input) {
      input.focus()
    }
  }, 100)
}

function hideQuickAdd() {
  quickAdd.value = {
    visible: false,
    date: null,
    name: '',
    icon: ''
  }
}

async function saveItem() {
  if (!quickAdd.value.date || !quickAdd.value.name.trim()) return

  creating.value = true
  try {
    // Create the item with the selected date
    const additionalFields: Record<string, string> = {
      name: quickAdd.value.name.trim()
    }

    // Add icon field if provided and table has icon field
    if (quickAdd.value.icon.trim() && hasIconField.value) {
      additionalFields.icon = quickAdd.value.icon.trim()
    }

    // Add calendar_date field if it exists in the table structure
    const structureRes = await client.getTableStructure({ pageId: props.tableId })
    const hasCalendarDateField = structureRes.fields?.some(f => f.name === 'calendar_date' && f.type === 'date')

    if (hasCalendarDateField) {
      // Convert to MySQL date format (YYYY-MM-DD)
      const year = quickAdd.value.date.getFullYear()
      const month = String(quickAdd.value.date.getMonth() + 1).padStart(2, '0')
      const day = String(quickAdd.value.date.getDate()).padStart(2, '0')
      additionalFields.calendar_date = `${year}-${month}-${day}`
    } else {
      // Fallback to starts field if calendar_date doesn't exist
      const hasStartsField = structureRes.fields?.some(f => f.name === 'starts' && f.type === 'datetime')
      if (hasStartsField) {
        // Convert to MySQL datetime format
        const year = quickAdd.value.date.getFullYear()
        const month = String(quickAdd.value.date.getMonth() + 1).padStart(2, '0')
        const day = String(quickAdd.value.date.getDate()).padStart(2, '0')
        const hours = String(quickAdd.value.date.getHours()).padStart(2, '0')
        const minutes = String(quickAdd.value.date.getMinutes()).padStart(2, '0')
        const seconds = String(quickAdd.value.date.getSeconds()).padStart(2, '0')
        additionalFields.starts = `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`
      }
    }

    await client.createItem({
      pageId: props.tableId,
      additionalFields: additionalFields
    })

    // Reload the calendar data to show the new item (without resetting view)
    await reloadItems()

    // Close the modal
    hideQuickAdd()
  } catch (e) {
    console.error('Failed to create item:', e)
    error.value = `Failed to create item: ${e}`
  } finally {
    creating.value = false
  }
}

async function saveAndEdit() {
  if (!quickAdd.value.date || !quickAdd.value.name.trim()) return

  creating.value = true
  try {
    // Create the item with the selected date
    const additionalFields: Record<string, string> = {
      name: quickAdd.value.name.trim()
    }

    // Add icon field if provided and table has icon field
    if (quickAdd.value.icon.trim() && hasIconField.value) {
      additionalFields.icon = quickAdd.value.icon.trim()
    }

    // Add calendar_date field if it exists in the table structure
    const structureRes = await client.getTableStructure({ pageId: props.tableId })
    const hasCalendarDateField = structureRes.fields?.some(f => f.name === 'calendar_date' && f.type === 'date')

    if (hasCalendarDateField) {
      // Convert to MySQL date format (YYYY-MM-DD)
      const year = quickAdd.value.date.getFullYear()
      const month = String(quickAdd.value.date.getMonth() + 1).padStart(2, '0')
      const day = String(quickAdd.value.date.getDate()).padStart(2, '0')
      additionalFields.calendar_date = `${year}-${month}-${day}`
    } else {
      // Fallback to starts field if calendar_date doesn't exist
      const hasStartsField = structureRes.fields?.some(f => f.name === 'starts' && f.type === 'datetime')
      if (hasStartsField) {
        // Convert to MySQL datetime format
        const year = quickAdd.value.date.getFullYear()
        const month = String(quickAdd.value.date.getMonth() + 1).padStart(2, '0')
        const day = String(quickAdd.value.date.getDate()).padStart(2, '0')
        const hours = String(quickAdd.value.date.getHours()).padStart(2, '0')
        const minutes = String(quickAdd.value.date.getMinutes()).padStart(2, '0')
        const seconds = String(quickAdd.value.date.getSeconds()).padStart(2, '0')
        additionalFields.starts = `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`
      }
    }

    const res = await client.createItem({
      pageId: props.tableId,
      additionalFields: additionalFields
    })

    // Navigate to edit page
    const itemId = getItemValue(res.item, 'id')
    if (itemId) {
      window.location.href = `/table/${props.tableId}/${itemId}/edit`
    }
  } catch (e) {
    console.error('Failed to create item:', e)
    error.value = `Failed to create item: ${e}`
  } finally {
    creating.value = false
  }
}

function goToToday() {
  const now = new Date()
  const newMonth = now.getMonth()
  const newYear = now.getFullYear()

  currentMonth.value = newMonth
  currentYear.value = newYear

  updateDatePicker(newMonth, newYear)
}

function prevMonth() {
  const newMonth = currentMonth.value === 0 ? 11 : currentMonth.value - 1
  const newYear = currentMonth.value === 0 ? currentYear.value - 1 : currentYear.value

  currentMonth.value = newMonth
  currentYear.value = newYear

  updateDatePicker(newMonth, newYear)
}

function nextMonth() {
  const newMonth = currentMonth.value === 11 ? 0 : currentMonth.value + 1
  const newYear = currentMonth.value === 11 ? currentYear.value + 1 : currentYear.value

  currentMonth.value = newMonth
  currentYear.value = newYear

  updateDatePicker(newMonth, newYear)
}

function goToSelectedDate() {
  if (!selectedDate.value) return

  const [year, month] = selectedDate.value.split('-').map(Number)

  currentMonth.value = month - 1 // Convert from 1-indexed to 0-indexed
  currentYear.value = year
}

function updateDatePicker(month: number, year: number) {
  const monthX = String(month + 1).padStart(2, '0')
  selectedDate.value = `${year}-${monthX}`
}

// Context menu functions
function hideContextMenu() {
  contextMenu.value.visible = false
  contextMenu.value.item = null
}

async function deleteItem() {
  if (!contextMenu.value.item) {
    console.log('No item in context menu')
    return
  }

  const itemId = getItemValue(contextMenu.value.item, 'id')
  if (!itemId) {
    console.log('No item ID found')
    return
  }

  console.log('Deleting item with ID:', itemId)
  deleting.value = true
  try {
    await client.deleteItem({
      id: String(itemId),
      pageId: props.tableId
    })

    // Reload the data to reflect changes (without resetting view)
    await reloadItems()
    hideContextMenu()
  } catch (e) {
    console.error('Failed to delete item:', e)
    error.value = `Failed to delete item: ${e}`
  } finally {
    deleting.value = false
  }
}

onMounted(load)
</script>

<template>
  <Section :title="sectionTitle" :padding="false">
    <template #toolbar>
      <div class="toolbar">
        <ViewsButton
          :table-id="props.tableId"
          :show-view-create="true"
          :show-view-edit="true"
          @view-changed="(viewType: string) => {
            emit('view-changed', viewType)
            // Reload if still on calendar view
            if (viewType === 'calendar') {
              reloadItems()
            }
          }"
        />
        <router-link :to="`/table/${props.tableId}/column-types`" class="button neutral">Structure</router-link>
        <button @click="goToToday" class="button neutral">Today</button>
        <button @click="prevMonth" class="button neutral">‹</button>
         <div class="date-picker-container">
           <input
             id="goto-date"
             type="month"
             v-model="selectedDate"
             @change="goToSelectedDate"
             class="date-picker-input"
           />
         </div>
        <button @click="nextMonth" class="button neutral">›</button>
      </div>
    </template>

    <div class="calendar-content">
      <Calendar
        :events="calendarEvents"
        :loading="loading"
        :error="error"
        :current-month="currentMonth"
        :show-navigation="false"
        :current-year="currentYear"
        :get-event-date-range="getEventDateRange"
        :format-event-time="formatEventTime"
        @event-click="handleEventClick"
        @date-click="handleDateClick"
        @month-change="handleMonthChange"
        @event-context-menu="handleEventContextMenu"
      >
        <template #event="{ event, date, position }">
          <div class="event-content" :title="getEventTooltip(event)">
            <div class="event-title">
              {{ event.title }}
              <span v-if="position !== 'single'" class="multi-day-indicator">
                {{ position === 'start' ? '▶' : position === 'end' ? '◀' : position === 'middle' ? '▬' : '' }}
              </span>
            </div>
            <div class="event-time">
              {{ formatEventTime(event, date) }}
            </div>
          </div>
        </template>
      </Calendar>
    </div>

    <!-- Context Menu -->
    <div
      v-if="contextMenu.visible"
      class="context-menu"
      :style="{ left: contextMenu.x + 'px', top: contextMenu.y + 'px' }"
      @click.stop
    >
      <div
        class="context-menu-item"
        :class="{ 'disabled': deleting }"
        @click="!deleting && deleteItem()"
      >
        <span class="context-menu-icon">🗑️</span>
        <span>{{ deleting ? 'Deleting...' : 'Delete Item' }}</span>
      </div>
    </div>

    <!-- Backdrop to close context menu -->
    <div
      v-if="contextMenu.visible"
      class="context-menu-backdrop"
      @click="hideContextMenu"
    ></div>

    <!-- Quick Add Modal -->
    <div
      v-if="quickAdd.visible"
      class="quick-add-modal"
      @click.stop
    >
      <div class="quick-add-content">
        <h3>Quick Add Item</h3>
        <p class="quick-add-date">
          {{ quickAdd.date?.toLocaleDateString() }}
        </p>
        <div class="quick-add-form">
          <input
            v-model="quickAdd.name"
            type="text"
            placeholder="Item name"
            class="quick-add-input"
            @keydown.enter="saveItem"
            @keydown.escape="hideQuickAdd"
            ref="quickAddInput"
          />
          <input
            v-if="hasIconField"
            v-model="quickAdd.icon"
            type="text"
            placeholder="Icon (optional)"
            class="quick-add-input"
            @keydown.enter="saveItem"
            @keydown.escape="hideQuickAdd"
          />
          <div class="quick-add-actions">
            <button @click="hideQuickAdd" class="button neutral" :disabled="creating">
              Cancel
            </button>
            <button @click="saveItem" class="button primary" :disabled="creating || !quickAdd.name.trim()">
              {{ creating ? 'Saving...' : 'Save' }}
            </button>
            <button @click="saveAndEdit" class="button secondary" :disabled="creating || !quickAdd.name.trim()">
              {{ creating ? 'Saving...' : 'Save & Edit' }}
            </button>
          </div>
        </div>
      </div>
    </div>

    <p class="padding" style="margin-top: 0">
      <small>{{ upcomingEventsMessage }}</small>
    </p>

    <!-- Backdrop to close quick add -->
    <div
      v-if="quickAdd.visible"
      class="quick-add-backdrop"
      @click="hideQuickAdd"
    ></div>
  </Section>
</template>

<style scoped>

.event-title {
  font-weight: bold;
}

.event-time {
  font-size: 0.8em;
  color: #666;
}

.date-picker-container {
  display: flex;
  align-items: center;
}

.calendar-wrapper {
  border-radius: 0;
}

.date-picker-input {
  padding: 0.5rem;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-size: 1rem;
  outline: none;
  cursor: pointer;
  min-width: 150px;
}

.date-picker-input:focus {
  border-color: #007bff;
  box-shadow: 0 0 0 2px rgba(0, 123, 255, 0.25);
}

.date-picker-input:hover {
  border-color: #999;
}

.toolbar {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

/* Context Menu Styles */
.context-menu {
  position: fixed;
  background: white;
  border: 1px solid #ddd;
  border-radius: 6px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  z-index: 1002;
  min-width: 150px;
  overflow: hidden;
}

.context-menu-item {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.75rem 1rem;
  cursor: pointer;
  transition: background-color 0.2s;
  font-size: 0.9rem;
}

.context-menu-item:hover:not(.disabled) {
  background: #f8f9fa;
}

.context-menu-item.disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.context-menu-icon {
  font-size: 1rem;
}

.context-menu-backdrop {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 1001;
  background: transparent;
}

/* Quick Add Modal */
.quick-add-modal {
  position: fixed;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  background: white;
  border-radius: 8px;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.15);
  z-index: 1001;
  min-width: 300px;
  max-width: 500px;
}

.quick-add-content {
  padding: 1.5rem;
}

.quick-add-content h3 {
  margin: 0 0 0.5rem 0;
  color: #333;
}

.quick-add-date {
  margin: 0 0 1rem 0;
  color: #666;
  font-size: 0.9rem;
}

.quick-add-form {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.quick-add-input {
  padding: 0.75rem;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-size: 1rem;
  outline: none;
}

.quick-add-input:focus {
  border-color: #007bff;
  box-shadow: 0 0 0 2px rgba(0, 123, 255, 0.25);
}

.quick-add-actions {
  display: flex;
  gap: 0.75rem;
  justify-content: flex-end;
  flex-wrap: wrap;
}

.quick-add-backdrop {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  z-index: 1000;
}

@media (max-width: 768px) {
  .calendar-content {
    padding: 0.5rem;
  }
}
</style>
