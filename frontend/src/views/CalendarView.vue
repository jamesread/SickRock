<script setup lang="ts">
import { onMounted, ref, computed, watch } from 'vue'
import { useRoute } from 'vue-router'
import { createConnectTransport } from '@connectrpc/connect-web'
import { createClient } from '@connectrpc/connect'
import { SickRock } from '../gen/sickrock_pb'
import RowActionsDropdown from '../components/RowActionsDropdown.vue'

const route = useRoute()
const tableId = route.params.tableName as string

type Item = Record<string, unknown>

const items = ref<Item[]>([])
const loading = ref(false)
const error = ref<string | null>(null)

// Calendar state
const currentDate = ref(new Date())
const currentMonth = computed(() => currentDate.value.getMonth())
const currentYear = computed(() => currentDate.value.getFullYear())

const transport = createConnectTransport({ baseUrl: '/api' })
const client = createClient(SickRock, transport)

// Helper function to get item value for a column
function getItemValue(item: any, column: string): any {
  if (column === 'id' || column === 'name' || column === 'created_at_unix') {
    return item[column]
  }
  if (item.additionalFields && item.additionalFields[column] !== undefined) {
    return item.additionalFields[column]
  }
  return item[column]
}

// Get items for a specific date
function getItemsForDate(date: Date): Item[] {
  const targetDate = new Date(date)
  targetDate.setHours(0, 0, 0, 0)

  return items.value.filter(item => {
    const createdAt = getItemValue(item, 'created_at_unix')
    if (!createdAt) {
      // If no created_at_unix, use current date as fallback
      const itemDate = new Date()
      itemDate.setHours(0, 0, 0, 0)
      return itemDate.getTime() === targetDate.getTime()
    }

    const itemDate = new Date(Number(createdAt) * 1000)
    itemDate.setHours(0, 0, 0, 0)

    return itemDate.getTime() === targetDate.getTime()
  })
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
  const startDay = firstDay.getDay()

  const days = []

  // Add empty cells for days before the first day of the month
  for (let i = 0; i < startDay; i++) {
    days.push(null)
  }

  // Add days of the month
  for (let day = 1; day <= daysInMonth; day++) {
    const date = new Date(year, month, day)
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

const dayNames = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']

// Load data
async function load() {
  loading.value = true
  error.value = null
  try {
    const res = await client.listItems({ pageId: tableId })
    items.value = Array.isArray(res.items) ? (res.items as Item[]) : []
  } catch (e) {
    error.value = String(e)
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <section class="with-header-and-content">
    <div class="section-header">
      <h2 class="calendar-title">{{ tableId }} Calendar</h2>
      <button @click="load" :disabled="loading">Reload</button>
    </div>

    <div v-if="error" class="error">{{ error }}</div>
    <div v-else-if="loading">Loading…</div>
    <div v-else class="section-content padding">
      <!-- Calendar Navigation -->
      <div class="calendar-nav">
        <button @click="previousMonth" class="nav-button">‹</button>
        <h3 class="month-year">{{ monthNames[currentMonth] }} {{ currentYear }}</h3>
        <button @click="nextMonth" class="nav-button">›</button>
        <button @click="goToToday" class="today-button">Today</button>
      </div>

      <!-- Calendar Grid -->
      <div class="calendar-container">
        <!-- Day headers -->
        <div class="calendar-header">
          <div v-for="day in dayNames" :key="day" class="day-header">{{ day }}</div>
        </div>

        <!-- Calendar days -->
        <div class="calendar-grid">
          <div
            v-for="(day, index) in calendarDays"
            :key="index"
            class="calendar-day"
            :class="{
              'empty': !day,
              'today': day && day.date.toDateString() === new Date().toDateString(),
              'has-items': day && day.items.length > 0
            }"
          >
            <div v-if="day" class="day-content">
              <router-link
                :to="`/table/${tableId}/insert-row?date=${day.date.toISOString().split('T')[0]}`"
                class="day-number"
              >
                {{ day.date.getDate() }}
              </router-link>
              <div class="day-items">
                <div
                  v-for="item in day.items.slice(0, 3)"
                  :key="getItemValue(item, 'id')"
                  class="calendar-item"
                  @click="$router.push(`/table/${tableId}/${getItemValue(item, 'id')}`)"
                >
                  <div class="item-title">{{ getItemValue(item, 'name') || `Item ${getItemValue(item, 'id') || 'Unknown'}` }}</div>
                  <div class="item-time">
                    {{ getItemValue(item, 'created_at_unix')
                        ? new Date(Number(getItemValue(item, 'created_at_unix')) * 1000).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
                        : 'No time' }}
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
  </section>
</template>

<style scoped>
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

.nav-button {
  background: #007bff;
  color: white;
  border: none;
  border-radius: 4px;
  padding: 0.5rem 1rem;
  cursor: pointer;
  font-size: 1.2rem;
  transition: background-color 0.2s;
}

.nav-button:hover {
  background: #0056b3;
}

.today-button {
  background: #6c757d;
  color: white;
  border: none;
  border-radius: 4px;
  padding: 0.5rem 1rem;
  cursor: pointer;
  transition: background-color 0.2s;
}

.today-button:hover {
  background: #545b62;
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
  border-radius: 8px;
  overflow: hidden;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

.calendar-header {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  background: #f8f9fa;
  border-bottom: 1px solid #e0e0e0;
}

.day-header {
  padding: 1rem;
  text-align: center;
  font-weight: 600;
  color: #666;
  border-right: 1px solid #e0e0e0;
}

.day-header:last-child {
  border-right: none;
}

.calendar-grid {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  grid-template-rows: repeat(6, 1fr);
  min-height: 400px;
}

.calendar-day {
  border-right: 1px solid #e0e0e0;
  border-bottom: 1px solid #e0e0e0;
  min-height: 120px;
  position: relative;
}

.calendar-day:last-child {
  border-right: none;
}

.calendar-day.empty {
  background: #f8f9fa;
}

.calendar-day.today {
  background: #e3f2fd;
}

.calendar-day.has-items {
  background: #f0f8ff;
}

.day-content {
  padding: 0.5rem;
  height: 100%;
  display: flex;
  flex-direction: column;
}

.day-number {
  font-weight: 600;
  font-size: 1.1rem;
  margin-bottom: 0.5rem;
  color: #333;
  text-decoration: none;
  display: inline-block;
  padding: 0.25rem;
  border-radius: 4px;
  transition: all 0.2s;
  min-width: 1.5rem;
  text-align: center;
}

.day-number:hover {
  background-color: #007bff;
  color: white;
  transform: scale(1.1);
}

.day-items {
  flex: 1;
  overflow: hidden;
}

.calendar-item {
  background: white;
  border: 1px solid #ddd;
  border-radius: 4px;
  padding: 0.25rem 0.5rem;
  margin-bottom: 0.25rem;
  cursor: pointer;
  transition: all 0.2s;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.1);
}

.calendar-item:hover {
  background: #f8f9fa;
  border-color: #007bff;
  transform: translateY(-1px);
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.15);
}

.item-title {
  font-weight: 500;
  font-size: 0.85rem;
  color: #333;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
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

  .day-content {
    padding: 0.25rem;
  }

  .day-number {
    font-size: 1rem;
  }

  .calendar-item {
    padding: 0.125rem 0.25rem;
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
}
</style>
