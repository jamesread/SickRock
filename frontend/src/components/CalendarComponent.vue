<script setup lang="ts">
import { onMounted, ref, computed, watch } from 'vue'

import { createApiClient } from '../stores/api'
import { SickRock } from '../gen/sickrock_pb'
import RowActionsDropdown from '../components/RowActionsDropdown.vue'

const props = defineProps<{
  tableId: string
}>()

type Item = Record<string, unknown>

const items = ref<Item[]>([])
const loading = ref(false)
const error = ref<string | null>(null)

// Inline editing state
const editingItem = ref<{ id: string; value: string } | null>(null)
const editingValue = ref('')
const saving = ref(false)

// Context menu state
const contextMenu = ref<{ visible: boolean; x: number; y: number; item: Item | null }>({
  visible: false,
  x: 0,
  y: 0,
  item: null
})
const deleting = ref(false)

// Quick add state
const quickAdd = ref<{ visible: boolean; date: Date | null; name: string }>({
  visible: false,
  date: null,
  name: ''
})
const creating = ref(false)

// Calendar state
const currentDate = ref(new Date())
const currentMonth = computed(() => currentDate.value.getMonth())
const currentYear = computed(() => currentDate.value.getFullYear())

// Transport handled by authenticated client
const client = createApiClient()

// Helper function to get item value for a column
function getItemValue(item: any, column: string): any {
  // Check standard fields first (only id and sr_created are static now)
  if (column === 'id' || column === 'sr_created') {
    return item[column]
  }
  // Check additional fields from protobuf (all other fields including name are dynamic)
  if (item.additionalFields && item.additionalFields[column] !== undefined) {
    return item.additionalFields[column]
  }
  // Fallback to direct property access
  return item[column]
}

// Get items for a specific date using starts/finishes fields
function getItemsForDate(date: Date): Item[] {
  const targetDate = new Date(date)
  targetDate.setHours(0, 0, 0, 0)

  return items.value.filter(item => {
    const starts = getItemValue(item, 'starts')
    const finishes = getItemValue(item, 'finishes')

    // If we have starts field, use it for date checking
    if (starts) {
      const startDate = new Date(starts)

      // Check if the start date matches the target date (ignoring time)
      const startDateOnly = new Date(startDate)
      startDateOnly.setHours(0, 0, 0, 0)

      // If we also have finishes, check if the target date falls within the range
      if (finishes) {
        const endDate = new Date(finishes)
        const endDateOnly = new Date(endDate)
        endDateOnly.setHours(0, 0, 0, 0)

        return targetDate >= startDateOnly && targetDate <= endDateOnly
      } else {
        // Only starts field - show on the start date
        return targetDate.getTime() === startDateOnly.getTime()
      }
    }

    // Fallback to sr_created for backward compatibility
    const createdAt = getItemValue(item, 'sr_created')
    if (createdAt) {
      const itemDate = new Date(Number(createdAt) * 1000)
      itemDate.setHours(0, 0, 0, 0)
      return itemDate.getTime() === targetDate.getTime()
    }

    // If no date fields at all, don't show the item
    return false
  })
}

// Check if an item is a multi-day event
function isMultiDayEvent(item: Item): boolean {
  const starts = getItemValue(item, 'starts')
  const finishes = getItemValue(item, 'finishes')

  if (!starts || !finishes) return false

  const startDate = new Date(starts)
  const endDate = new Date(finishes)

  // Set times to start/end of day for proper comparison
  startDate.setHours(0, 0, 0, 0)
  endDate.setHours(0, 0, 0, 0)

  return startDate.getTime() !== endDate.getTime()
}

// Get the position of an item within a multi-day event for a specific date
function getMultiDayPosition(item: Item, date: Date): 'start' | 'middle' | 'end' | 'single' {
  const starts = getItemValue(item, 'starts')
  const finishes = getItemValue(item, 'finishes')

  if (!starts || !finishes) return 'single'

  const startDate = new Date(starts)
  const endDate = new Date(finishes)
  const targetDate = new Date(date)

  // Set times to start/end of day for proper comparison
  startDate.setHours(0, 0, 0, 0)
  endDate.setHours(0, 0, 0, 0)
  targetDate.setHours(0, 0, 0, 0)

  if (startDate.getTime() === endDate.getTime()) return 'single'
  if (targetDate.getTime() === startDate.getTime()) return 'start'
  if (targetDate.getTime() === endDate.getTime()) return 'end'
  if (targetDate > startDate && targetDate < endDate) return 'middle'

  return 'single'
}

// Format event time based on position in multi-day event
function formatEventTime(item: Item, date: Date): string {
  const starts = getItemValue(item, 'starts')
  const finishes = getItemValue(item, 'finishes')
  const position = getMultiDayPosition(item, date)

  if (!starts || !finishes) return 'No time'

  const startDate = new Date(starts)
  const endDate = new Date(finishes)
  const targetDate = new Date(date)

  // Set times to start/end of day for proper comparison
  startDate.setHours(0, 0, 0, 0)
  endDate.setHours(0, 0, 0, 0)
  targetDate.setHours(0, 0, 0, 0)

  if (position === 'start') {
    return new Date(starts).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  } else if (position === 'end') {
    return new Date(finishes).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  } else if (position === 'middle') {
    return 'All day'
  }

  return 'All day'
}

// Calendar generation
const calendarDays = computed(() => {
  const year = currentYear.value
  const month = currentMonth.value

  // Get first day of month and number of days
  const firstDay = new Date(year, month, 1)
  const lastDay = new Date(year, month + 1, 0)
  const daysInMonth = lastDay.getDate()

  // Get starting day of week (0 = Sunday, 1 = Monday, etc.)
  // Adjust for Monday as first day: Sunday (0) becomes 6, Monday (1) becomes 0, etc.
  const startDay = (firstDay.getDay() + 6) % 7

  const days = []

  // Add days from previous month to fill the first row
  const prevMonth = month === 0 ? 11 : month - 1
  const prevYear = month === 0 ? year - 1 : year
  const prevMonthLastDay = new Date(prevYear, prevMonth + 1, 0).getDate()

  for (let i = 0; i < startDay; i++) {
    const day = prevMonthLastDay - startDay + i + 1
    const date = new Date(prevYear, prevMonth, day)
    days.push({
      date,
      items: getItemsForDate(date)
    })
  }

  // Add days of the month
  for (let day = 1; day <= daysInMonth; day++) {
    const date = new Date(year, month, day)
    days.push({
      date,
      items: getItemsForDate(date)
    })
  }

  // Add days from next month to complete the last row
  // Calculate how many more days we need to reach 42 total cells (6 rows × 7 columns)
  const totalCells = 42
  const remainingCells = totalCells - days.length

  for (let day = 1; day <= remainingCells; day++) {
    const date = new Date(year, month + 1, day)
    days.push({
      date,
      items: getItemsForDate(date)
    })
  }

  return days
})

// Navigation functions
function previousMonth() {
  currentDate.value = new Date(currentYear.value, currentMonth.value - 1, 1)
}

function nextMonth() {
  currentDate.value = new Date(currentYear.value, currentMonth.value + 1, 1)
}

function goToToday() {
  currentDate.value = new Date()
}

// Month names
const monthNames = [
  'January', 'February', 'March', 'April', 'May', 'June',
  'July', 'August', 'September', 'October', 'November', 'December'
]

const dayNames = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun']

// Load data
async function load() {
  loading.value = true
  error.value = null
  try {
    const res = await client.listItems({ tcName: props.tableId })
    items.value = Array.isArray(res.items) ? (res.items as Item[]) : []
  } catch (e) {
    error.value = String(e)
  } finally {
    loading.value = false
  }
}

// Navigation functions
function openRow(item: Item) {
  const itemId = getItemValue(item, 'id')
  if (itemId) {
    window.location.href = `/table/${props.tableId}/${itemId}`
  }
}

// Quick add functions
function showQuickAdd(date: Date) {
  quickAdd.value = {
    visible: true,
    date: date,
    name: ''
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
    name: ''
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

    // Add starts field if it exists in the table structure
    const structureRes = await client.getTableStructure({ pageId: props.tableId })
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

    await client.createItem({
      pageId: props.tableId,
      additionalFields: additionalFields
    })

    // Reload the calendar data to show the new item
    await load()

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

    // Add starts field if it exists in the table structure
    const structureRes = await client.getTableStructure({ pageId: props.tableId })
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

// Inline editing functions
function startEdit(item: Item) {
  const itemId = getItemValue(item, 'id')
  const currentName = getItemValue(item, 'name') || `Item ${itemId || 'Unknown'}`

  editingItem.value = { id: String(itemId), value: String(currentName) }
  editingValue.value = String(currentName)

  // Focus the input after the DOM updates
  setTimeout(() => {
    const input = document.querySelector('.edit-input') as HTMLInputElement
    if (input) {
      input.focus()
      input.select()
    }
  }, 0)
}

function cancelEdit() {
  editingItem.value = null
  editingValue.value = ''
}

async function saveEdit() {
  if (!editingItem.value) return

  saving.value = true
  try {
    const { id } = editingItem.value

    // Prepare the update data - all fields go into additionalFields
    const additionalFields: Record<string, string> = {}

    // Get all current values and update just the name field
    const currentItem = items.value.find(it => String(getItemValue(it, 'id')) === id)
    if (currentItem) {
      // Get all additional fields from the current item
      if (currentItem.additionalFields) {
        Object.entries(currentItem.additionalFields).forEach(([key, value]) => {
          additionalFields[key] = String(value)
        })
      }
      // Update the name field
      additionalFields['name'] = editingValue.value

      await client.editItem({
        id: id,
        additionalFields: additionalFields,
        pageId: props.tableId
      })
    }

    // Reload the data to reflect changes
    await load()
    cancelEdit()
  } catch (e) {
    console.error('Failed to save edit:', e)
    error.value = `Failed to save edit: ${e}`
  } finally {
    saving.value = false
  }
}

function isEditing(item: Item): boolean {
  const itemId = getItemValue(item, 'id')
  return editingItem.value?.id === String(itemId)
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Enter') {
    event.preventDefault()
    saveEdit()
  } else if (event.key === 'Escape') {
    event.preventDefault()
    cancelEdit()
  }
}

// Context menu functions
function showContextMenu(event: MouseEvent, item: Item) {
  event.preventDefault()
  event.stopPropagation()

  console.log('Context menu triggered for item:', item)

  contextMenu.value = {
    visible: true,
    x: event.clientX,
    y: event.clientY,
    item: item
  }
}

function hideContextMenu() {
  contextMenu.value.visible = false
  contextMenu.value.item = null
}

async function deleteItem() {
  console.log('Delete item clicked, contextMenu:', contextMenu.value)

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

    // Reload the data to reflect changes
    await load()
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
  <section class="with-header-and-content">
    <div class="section-header">
      <h2 class="calendar-title">{{ monthNames[currentMonth] }} {{ currentYear }}</h2>

      <div class = "toolbar">
        <button @click="previousMonth" class="button neutral">‹</button>
        <button @click="goToToday" class="button neutral">Today</button>
        <button @click="nextMonth" class="button neutral">›</button>
      </div>
    </div>

    <div v-if="error" class="error">{{ error }}</div>
    <div v-else-if="loading">Loading…</div>
      <div v-else class="section-content md:p-8">
      <div class="calendar-container">
        <div class="calendar-grid">
          <div v-for="day in dayNames" :key="day" class="day-header">{{ day }}</div>

          <div
            v-for="(day, index) in calendarDays"
            :key="index"
            class="calendar-day"
            :class="{
              'empty': !day,
              'today': day && day.date.toDateString() === new Date().toDateString(),
              'weekend': day && (day.date.getDay() === 0 || day.date.getDay() === 6),
              'prev-month': day && day.date.getMonth() !== currentMonth && day.date.getMonth() !== (currentMonth + 1) % 12,
              'next-month': day && day.date.getMonth() !== currentMonth && day.date.getMonth() === (currentMonth + 1) % 12
            }"
          >
            <div v-if="day" class="day-content" @click="showQuickAdd(day.date)">
              <div
                class="day-number clickable"
              >
                {{ day.date.getDate() }}
              </div>
              <div class="day-items">
                <div
                  v-for="item in day.items.sort((a, b) => {
                    const aMultiDay = isMultiDayEvent(a);
                    const bMultiDay = isMultiDayEvent(b);
                    if (aMultiDay && !bMultiDay) return -1;
                    if (!aMultiDay && bMultiDay) return 1;
                    return 0;
                  }).slice(0, 3)"
                  :key="getItemValue(item, 'id')"
                  class="calendar-item"
                  :class="{
                    'editing': isEditing(item),
                    'multi-day': isMultiDayEvent(item),
                    'multi-day-start': isMultiDayEvent(item) && getMultiDayPosition(item, day.date) === 'start',
                    'multi-day-middle': isMultiDayEvent(item) && getMultiDayPosition(item, day.date) === 'middle',
                    'multi-day-end': isMultiDayEvent(item) && getMultiDayPosition(item, day.date) === 'end'
                  }"
                  @click.stop
                >
                  <div v-if="isEditing(item)" class="edit-container" @click.stop>
                    <input
                      v-model="editingValue"
                      class="edit-input"
                      @keydown="handleKeydown"
                      @blur="saveEdit"
                      :disabled="saving"
                    />
                    <div class="edit-actions">
                      <button @click="saveEdit" class="save-btn" :disabled="saving" title="Save (Enter)">
                        ✓
                      </button>
                      <button @click="cancelEdit" class="cancel-btn" :disabled="saving" title="Cancel (Esc)">
                        ✕
                      </button>
                    </div>
                  </div>
                  <div v-else class="item-content" @click.stop="openRow(item)" @dblclick.stop="startEdit(item)" @contextmenu.stop="showContextMenu($event, item)">
                    <div class="item-title">
                      {{ getItemValue(item, 'name') || `Item ${getItemValue(item, 'id') || 'Unknown'}` }}
                      <span v-if="isMultiDayEvent(item)" class="multi-day-indicator">
                        {{ getMultiDayPosition(item, day.date) === 'start' ? '▶' :
                           getMultiDayPosition(item, day.date) === 'end' ? '◀' :
                           getMultiDayPosition(item, day.date) === 'middle' ? '▬' : '' }}
                      </span>
                    </div>
                    <div class="item-time">
                      <span v-if="getItemValue(item, 'starts') && getItemValue(item, 'finishes')">
                        {{ formatEventTime(item, day.date) }}
                      </span>
                      <span v-else-if="getItemValue(item, 'sr_created')">
                        {{ new Date(Number(getItemValue(item, 'sr_created')) * 1000).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) }}
                      </span>
                      <span v-else>No time</span>
                    </div>
                  </div>
                </div>
                <div v-if="day.items.length > 3" class="more-items">
                  +{{ day.items.length - 3 }} more
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
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

    <!-- Backdrop to close quick add -->
    <div
      v-if="quickAdd.visible"
      class="quick-add-backdrop"
      @click="hideQuickAdd"
    ></div>
  </section>
</template>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  gap: .5rem;
}

.calendar-title {
  margin: 0;
}

.error {
  color: #b00020;
  padding: 1rem;
}

.calendar-nav {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 1rem;
  margin-bottom: 1rem;
  padding: 1rem;
  background: #f8f9fa;
  border-radius: 8px;
}

.section-header .button {
  background: #fff;
}

.section-header .button:hover {
  background: #c9ccd4;
}

.month-year {
  margin: 0;
  font-size: 1.5rem;
  font-weight: 600;
  min-width: 200px;
  text-align: center;
}

.calendar-container {
  background: white;
  border: 1px solid #e0e0e0;
  overflow: hidden;
}

.calendar-header {
  display: grid;
  background: #f8f9fa;
  border-bottom: 1px solid #e0e0e0;
}

.day-header {
  padding: 1rem;
  text-align: center;
  font-weight: 600;
  color: #666;
  border-right: 1px solid #e0e0e0;
  border-bottom: 1px solid #e0e0e0;
  background: #f8f9fa;
}

.day-header:last-child {
  border-right: none;
}

.calendar-grid {
  display: grid;
  grid-template-columns: repeat(7, minmax(0, 1fr)); /* Fixed width columns */
  min-height: 400px;
}

.calendar-day {
  border-right: 1px solid #e0e0e0;
  border-bottom: 1px solid #e0e0e0;
  min-height: 120px;
  position: relative;
  transition: background-color 0.2s ease;
  cursor: pointer;
}

.calendar-day:hover {
  background-color: #f0f8ff;
}

.calendar-day:last-child {
  border-right: none;
}

.calendar-day.empty {
  background: #f8f9fa;
}

.calendar-day.empty:hover {
  background: #e9ecef;
}

.calendar-day.today {
  background: #f7f8d7;
}

.calendar-day.today:hover {
  background: #bbdefb;
}

.calendar-day.weekend {
  background: #f8f9fa;
}

.calendar-day.weekend:hover {
  background: #e9ecef;
}

.calendar-day.next-month {
  background: #f8f9fa;
  opacity: 0.6;
}

.calendar-day.next-month:hover {
  background: #e9ecef;
  opacity: 0.8;
}

.calendar-day.prev-month {
  background: #f8f9fa;
  opacity: 0.6;
}

.calendar-day.prev-month:hover {
  background: #e9ecef;
  opacity: 0.8;
}

.day-content {
  height: 100%;
  display: flex;
  flex-direction: column;
}

.day-number {
  font-size: .9rem;
  margin-bottom: 0.25rem;
  color: #333;
  text-decoration: none;
  display: inline-block;
  padding: 0.1rem;
  border-radius: 4px;
  transition: all 0.2s;
  min-width: 1.5rem;
  text-align: center;
}

.day-number.clickable {
  cursor: pointer;
  padding: 0.25rem;
  border-radius: 4px;
  transition: background-color 0.2s;
}

.day-content:hover .day-number.clickable {
  color: #007bff;
}

.day-items {
  flex: 1;
  overflow: hidden;
}

.calendar-item {
  background: #d6f1aa;
  border: 1px solid #c4db96;
  margin-bottom: 0.25rem;
  cursor: pointer;
  transition: all 0.2s;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.1);
  padding: 0.1rem 0.1rem;
}

.calendar-item:hover {
  background: #b3de6e;
  border-color: #c4db96;

  transform: translateY(-1px);
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.15);
}

.calendar-item.editing {
  background: #fff3cd;
  border-color: #ffc107;
  cursor: default;
}


.calendar-item.multi-day-start {
  border-top-left-radius: 4px;
  border-bottom-left-radius: 4px;
}

.calendar-item.multi-day-middle {
  border-radius: 0;
}

.calendar-item.multi-day-end {
  border-top-right-radius: 4px;
  border-bottom-right-radius: 4px;
}

.item-content {
  cursor: pointer;
}

.edit-container {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.edit-input {
  width: 100%;
  padding: 0.25rem;
  border: 1px solid #007bff;
  border-radius: 3px;
  font-size: 0.85rem;
  background: white;
}

.edit-input:focus {
  outline: none;
  border-color: #0056b3;
  box-shadow: 0 0 0 2px rgba(0, 123, 255, 0.25);
}

.edit-actions {
  display: flex;
  gap: 0.25rem;
  justify-content: flex-end;
}

.save-btn, .cancel-btn {
  background: none;
  border: none;
  padding: 0.125rem 0.25rem;
  border-radius: 3px;
  cursor: pointer;
  font-size: 0.75rem;
  font-weight: bold;
  transition: all 0.2s;
}

.save-btn {
  color: #28a745;
}

.save-btn:hover:not(:disabled) {
  background: #28a745;
  color: white;
}

.cancel-btn {
  color: #dc3545;
}

.cancel-btn:hover:not(:disabled) {
  background: #dc3545;
  color: white;
}

.save-btn:disabled, .cancel-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
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

.item-title {
  font-weight: bold;
  font-size: 0.85rem;
  color: #333;
  white-space: normal;
  overflow: hidden;
  text-overflow: ellipsis;
  display: flex;
  align-items: center;
  gap: 0.25rem;
}

.multi-day-indicator {
  font-size: 0.7rem;
  color: #007bff;
  font-weight: bold;
}

.item-time {
  font-size: 0.75rem;
  color: #666;
  margin-top: 0.125rem;
}

.more-items {
  font-size: 0.75rem;
  color: #666;
  text-align: center;
  padding: 0.25rem;
  background: #f8f9fa;
  border-radius: 4px;
  margin-top: 0.25rem;
}

/* Responsive design */
@media (max-width: 768px) {
  .calendar-nav {
    flex-wrap: wrap;
    gap: 0.5rem;
  }

  .month-year {
    font-size: 1.2rem;
    min-width: 150px;
  }

  .calendar-day {
    min-height: 80px;
  }

  .day-number {
    font-size: 1rem;
  }

  .item-title {
    font-size: 0.8rem;
  }

  .item-time {
    font-size: 0.7rem;
  }
}

@media (max-width: 480px) {
  .calendar-grid {
    min-height: 300px;
  }

  .calendar-day {
    min-height: 60px;
  }

  .day-header {
    padding: 0.5rem 0.25rem;
    font-size: 0.8rem;
  }
  section {
    margin-top: 0;
  }

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

@media (min-width: 768px) {
  .section-content {
    padding: 1em;
  }

  .day-content {
    padding: 0.45rem;
  }

  .calendar-item {
    border-radius: 4px;
    padding: 0.25rem 0.5rem;
  }

  .calendar-container {
    border-radius: 8px;
  }
}
</style>
