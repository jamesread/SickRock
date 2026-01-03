<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Section from 'picocrank/vue/components/Section.vue'
import { createApiClient } from '../stores/api'
import { HugeiconsIcon } from '@hugeicons/vue'
import { ArrowLeft01Icon } from '@hugeicons/core-free-icons'
import ViewsButton from '../components/ViewsButton.vue'

const route = useRoute()
const router = useRouter()
const tableName = route.params.tableName as string
const dateParam = route.params.date as string

// Parse date from YYYY-MM-DD format
const selectedDate = computed(() => {
  if (!dateParam) return null
  const [year, month, day] = dateParam.split('-').map(Number)
  return new Date(year, month - 1, day)
})

const formattedDate = computed(() => {
  if (!selectedDate.value) return ''
  return selectedDate.value.toLocaleDateString('en-US', {
    weekday: 'long',
    year: 'numeric',
    month: 'long',
    day: 'numeric'
  })
})

type Item = Record<string, unknown>

const items = ref<Item[]>([])
const loading = ref(false)
const error = ref<string | null>(null)
const tableTitle = ref<string>('')

// Mobile menu state
const mobileMenuVisible = ref(false)

// Transport handled by authenticated client
const client = createApiClient()

// Helper function to get item value for a column
function getItemValue(item: any, column: string): any {
  if (column === 'id' || column === 'sr_created' || column === 'sr_updated') {
    return item[column]
  }
  if (item.additionalFields && item.additionalFields[column] !== undefined) {
    return item.additionalFields[column]
  }
  return item[column]
}

// Helper function to parse MySQL datetime/date strings as local time
function parseServerDateTime(dateStr: any): Date | null {
  if (!dateStr) return null

  const str = String(dateStr).trim()
  const datetimeMatch = str.match(/^(\d{4})-(\d{2})-(\d{2})(?: (\d{2}):(\d{2}):(\d{2}))?$/)

  if (!datetimeMatch) {
    const date = new Date(str)
    return isNaN(date.getTime()) ? null : date
  }

  const [, year, month, day, hour = '0', minute = '0', second = '0'] = datetimeMatch

  const date = new Date(
    parseInt(year, 10),
    parseInt(month, 10) - 1,
    parseInt(day, 10),
    parseInt(hour, 10),
    parseInt(minute, 10),
    parseInt(second, 10)
  )

  return isNaN(date.getTime()) ? null : date
}

// Filter items for the selected day
const dayEvents = computed(() => {
  if (!selectedDate.value) return []

  const targetDate = new Date(selectedDate.value)
  targetDate.setHours(0, 0, 0, 0)
  const targetDateEnd = new Date(targetDate)
  targetDateEnd.setHours(23, 59, 59, 999)

  return items.value.filter(item => {
    const calendarDate = getItemValue(item, 'calendar_date')
    const starts = getItemValue(item, 'starts')
    const finishes = getItemValue(item, 'finishes')
    const srCreated = getItemValue(item, 'sr_created')

    let eventDate: Date | null = null
    let startDate: Date | null = null
    let endDate: Date | null = null

    if (calendarDate) {
      eventDate = parseServerDateTime(calendarDate)
      if (eventDate) {
        eventDate.setHours(0, 0, 0, 0)
        return eventDate.getTime() === targetDate.getTime()
      }
    } else if (starts) {
      startDate = parseServerDateTime(starts)
      if (finishes) {
        endDate = parseServerDateTime(finishes)
      } else {
        endDate = startDate
      }

      if (startDate && endDate) {
        // Check if the target date falls within the event range
        const startDateOnly = new Date(startDate)
        startDateOnly.setHours(0, 0, 0, 0)
        const endDateOnly = new Date(endDate)
        endDateOnly.setHours(0, 0, 0, 0)

        return targetDate.getTime() >= startDateOnly.getTime() &&
               targetDate.getTime() <= endDateOnly.getTime()
      }
    } else if (srCreated) {
      eventDate = new Date(Number(srCreated) * 1000)
      eventDate.setHours(0, 0, 0, 0)
      return eventDate.getTime() === targetDate.getTime()
    }

    return false
  })
})

// Format event time
function formatEventTime(item: Item): string {
  const starts = getItemValue(item, 'starts')
  const finishes = getItemValue(item, 'finishes')
  const calendarDate = getItemValue(item, 'calendar_date')

  if (calendarDate) {
    // For calendar_date, no time is shown
    return 'All day'
  }

  if (starts) {
    const startDate = parseServerDateTime(starts)
    if (startDate) {
      // Check if start time is midnight
      if (startDate.getHours() === 0 && startDate.getMinutes() === 0) {
        if (finishes) {
          const endDate = parseServerDateTime(finishes)
          if (endDate && (endDate.getHours() !== 0 || endDate.getMinutes() !== 0)) {
            return `Until ${endDate.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}`
          }
        }
        return 'All day'
      }

      if (finishes) {
        const endDate = parseServerDateTime(finishes)
        if (endDate) {
          return `${startDate.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })} - ${endDate.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}`
        }
      }

      return startDate.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    }
  }

  return 'No time'
}

function handleEventClick(item: Item) {
  const itemId = getItemValue(item, 'id')
  if (itemId) {
    router.push(`/table/${tableName}/${itemId}`)
  }
}

function goBack() {
  router.push(`/table/${tableName}`)
}

function toggleMobileMenu() {
  mobileMenuVisible.value = !mobileMenuVisible.value
}

function hideMobileMenu() {
  mobileMenuVisible.value = false
}

function handleViewChanged(viewType: string) {
  // Navigate to the table view with the selected view type
  hideMobileMenu()
  router.push(`/table/${tableName}`)
}

function handleSectionClick(event: MouseEvent) {
  // Check if the click is on the section header/title area
  const target = event.target as HTMLElement

  // Stop propagation if clicking on buttons or links
  if (target.closest('button') || target.closest('a') || target.closest('.toolbar')) {
    return
  }

  // Check if clicked element is within the section header area
  // The Section component from picocrank likely renders the title as h1 or h2
  const isHeaderClick = target.closest('h1') ||
                        target.closest('h2') ||
                        target.closest('.section-header') ||
                        target.tagName === 'H1' ||
                        target.tagName === 'H2' ||
                        (target.parentElement && (target.parentElement.tagName === 'H1' || target.parentElement.tagName === 'H2'))

  if (isHeaderClick) {
    event.stopPropagation()
    toggleMobileMenu()
  }
}

async function loadData() {
  loading.value = true
  error.value = null
  try {
    // Load items
    const res = await client.listItems({ tcName: tableName })
    items.value = Array.isArray(res.items) ? (res.items as Item[]) : []

    // Load table configuration title
    if (!tableTitle.value) {
      try {
        const configs = await client.getTableConfigurations({})
        const config = configs.pages?.find(p => p.id === tableName)
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

onMounted(loadData)
</script>

<template>
  <Section title="Day view" :padding="true" class="day-view-section" @click="handleSectionClick">
    <template #toolbar>
      <button @click="goBack" class="button neutral">
        <HugeiconsIcon :icon="ArrowLeft01Icon" />
        Back to Calendar
      </button>
    </template>

    <div v-if="loading" class="loading">Loading...</div>
    <div v-else-if="error" class="error">{{ error }}</div>
    <div v-else-if="!selectedDate" class="error">Invalid date</div>
    <div v-else>
      <h2 class="day-title">{{ formattedDate }}</h2>

      <div v-if="dayEvents.length === 0" class="no-events">
        <p>No events scheduled for this day.</p>
      </div>

      <div v-else class="events-list">
        <div
          v-for="item in dayEvents"
          :key="getItemValue(item, 'id')"
          class="event-item"
          @click="handleEventClick(item)"
        >
          <div class="event-content">
            <div class="event-title">
              {{ getItemValue(item, 'name') || `Item ${getItemValue(item, 'id')}` }}
            </div>
            <div class="event-time">
              {{ formatEventTime(item) }}
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Mobile Menu Modal -->
    <div
      v-if="mobileMenuVisible"
      class="mobile-menu-modal"
      @click.stop
    >
      <div class="mobile-menu-content">
        <h3>Menu</h3>
        <div class="mobile-menu-actions">
          <ViewsButton
            :table-id="tableName"
            :show-view-create="true"
            :show-view-edit="true"
            @view-changed="handleViewChanged"
          />
          <router-link :to="`/table/${tableName}/column-types`" class="button neutral" @click="hideMobileMenu">
            Structure
          </router-link>
          <button @click="hideMobileMenu" class="button neutral">
            Cancel
          </button>
        </div>
      </div>
    </div>

    <!-- Backdrop to close mobile menu -->
    <div
      v-if="mobileMenuVisible"
      class="mobile-menu-backdrop"
      @click="hideMobileMenu"
    ></div>
  </Section>
</template>

<style scoped>
/* Make section header clickable */
.day-view-section :deep(.section-header),
.day-view-section :deep(header),
.day-view-section :deep(h1),
.day-view-section :deep(h2) {
  cursor: pointer;
  user-select: none;
  position: relative;
}

.day-view-section :deep(.section-header):hover,
.day-view-section :deep(header):hover,
.day-view-section :deep(h1):hover,
.day-view-section :deep(h2):hover {
  opacity: 0.8;
}

.day-view-section :deep(.section-header):active,
.day-view-section :deep(header):active,
.day-view-section :deep(h1):active,
.day-view-section :deep(h2):active {
  opacity: 0.6;
}

.day-title {
  margin: 0 0 1.5rem 0;
  font-size: 1.5rem;
  font-weight: 600;
  color: #333;
}

.loading,
.error {
  padding: 2rem;
  text-align: center;
}

.error {
  color: #d32f2f;
}

.no-events {
  padding: 3rem;
  text-align: center;
  color: #666;
}

.events-list {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.event-item {
  cursor: pointer;
  transition: all 0.2s ease;
  padding: 0.5rem 0.75rem;
  background: #e3f2fd;
  border-left: 3px solid #2196f3;
  border-radius: 4px;
}

.event-item:hover {
  background: #bbdefb;
  transform: translateX(2px);
}

.event-content {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.event-title {
  font-weight: bold;
  margin: 0;
  color: #1976d2;
}

.event-time {
  font-size: 0.8em;
  color: #666;
}

@media (max-width: 768px) {
  .event-content,
  .event-title,
  .event-time {
    font-size: 0.7rem;
  }
}

/* Menu Modal */
.mobile-menu-modal {
  position: fixed;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  background: white;
  border-radius: 8px;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.15);
  z-index: 1001;
  min-width: 300px;
  max-width: 400px;
}

.mobile-menu-content {
  padding: 1.5rem;
}

.mobile-menu-content h3 {
  margin: 0 0 1rem 0;
  color: #333;
  font-size: 1.2rem;
}

.mobile-menu-actions {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.mobile-menu-backdrop {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  z-index: 1000;
}
</style>
