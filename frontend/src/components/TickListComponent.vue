<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import Section from 'picocrank/vue/components/Section.vue'
import { createApiClient } from '../stores/api'
import ViewsButton from './ViewsButton.vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { CheckmarkSquare03Icon } from '@hugeicons/core-free-icons'

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

// Client-side only completion state (stored in JavaScript, not persisted to server)
const completionState = ref<Map<string, boolean>>(new Map())

// Table structure state
const tableStructure = ref<any>(null)
const tableTitle = ref<string>('')

// Computed property for the section title
const sectionTitle = computed(() => {
  return tableTitle.value || `Table: ${props.tableId}`
})

// Transport handled by authenticated client
const client = createApiClient()

// Helper function to get item value for a column
function getItemValue(item: any, column: string): any {
  // Check standard fields first
  if (column === 'id' || column === 'sr_created' || column === 'sr_updated') {
    return item[column]
  }
  // Check additional fields from protobuf
  if (item.additionalFields && item.additionalFields[column] !== undefined) {
    return item.additionalFields[column]
  }
  // Fallback to direct property access
  return item[column]
}

// Helper function to check if a column is a tinyint (boolean) column
function isTinyintColumn(column: string): boolean {
  const field = tableStructure.value?.fields?.find((f: any) => f.name === column)
  return field?.type?.startsWith('tinyint') || false
}

// Helper function to get boolean value from tinyint column
function getBooleanValue(item: any, column: string): boolean {
  const value = getItemValue(item, column)
  if (value === null || value === undefined) return false
  const numValue = Number(value)
  return numValue === 1
}

// Find the completion field - look for common names like "completed", "done", "checked", or first tinyint field
// This is optional - if no completion field exists, items will just be displayed without checkmarks
const completionField = computed(() => {
  if (!tableStructure.value?.fields) return null

  // First, try to find a field with a common completion name
  const commonNames = ['completed', 'done', 'checked', 'finished', 'status']
  for (const name of commonNames) {
    const field = tableStructure.value.fields.find((f: any) =>
      f.name.toLowerCase() === name.toLowerCase()
    )
    if (field) return field.name
  }

  // If no common name found, use the first tinyint field
  const tinyintField = tableStructure.value.fields.find((f: any) =>
    f.type?.startsWith('tinyint')
  )
  return tinyintField?.name || null
})

// Get display field - prefer "name" or "title", otherwise first text field
const displayField = computed(() => {
  if (!tableStructure.value?.fields) return null

  // Prefer "name" or "title"
  const preferred = tableStructure.value.fields.find((f: any) =>
    ['name', 'title'].includes(f.name.toLowerCase())
  )
  if (preferred) return preferred.name

  // Otherwise use first string/varchar field
  const textField = tableStructure.value.fields.find((f: any) =>
    f.type?.includes('varchar') || f.type === 'string' || f.type?.includes('text')
  )
  return textField?.name || null
})

async function load() {
  loading.value = true
  error.value = null
  try {
    const res = await client.listItems({ tcName: props.tableId })
    items.value = Array.isArray(res.items) ? (res.items as Item[]) : []

    // Fetch table configuration to get the title
    const configs = await client.getTableConfigurations({})
    const config = configs.pages?.find(p => p.id === props.tableId)
    if (config && config.title) {
      tableTitle.value = config.title
    } else {
      tableTitle.value = `Table: ${props.tableId}`
    }

    // Load table structure
    const structureRes = await client.getTableStructure({ pageId: props.tableId })
    tableStructure.value = structureRes
  } catch (e) {
    error.value = String(e)
  } finally {
    loading.value = false
  }
}

// Toggle completion state (client-side only, no server updates)
function toggleCompleted(item: any) {
  const itemId = String(item.id)
  const currentState = isCompleted(item)
  completionState.value.set(itemId, !currentState)
}

function getItemDisplayText(item: any): string {
  if (displayField.value) {
    const value = getItemValue(item, displayField.value)
    if (value !== null && value !== undefined) {
      return String(value)
    }
  }
  // Fallback to ID or first available field
  return `Item ${item.id || 'Unknown'}`
}

function isCompleted(item: any): boolean {
  const itemId = String(item.id)

  // First check client-side state (takes precedence)
  if (completionState.value.has(itemId)) {
    return completionState.value.get(itemId) || false
  }

  // Fall back to database value if no client-side state exists
  if (!completionField.value) return false
  return getBooleanValue(item, completionField.value)
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
            if (viewType === 'ticklist') {
              load()
            }
          }"
        />
        <router-link :to="`/table/${props.tableId}/column-types`" class="button neutral">Structure</router-link>
        <router-link :to="`/table/${props.tableId}/insert-row`" class="button primary">
          <HugeiconsIcon :icon="CheckmarkSquare03Icon" />
          Add Item
        </router-link>
      </div>
    </template>

    <div v-if="loading" class="loading">Loading...</div>
    <div v-else-if="error" class="error">{{ error }}</div>
    <div v-else>
      <div class="ticklist-container">
        <div
          v-for="item in items"
          :key="String(item.id)"
          class="ticklist-item"
          :class="{ completed: isCompleted(item) }"
          @click="toggleCompleted(item)"
        >
          <div class="ticklist-checkbox">
            <HugeiconsIcon
              v-if="isCompleted(item)"
              :icon="CheckmarkSquare03Icon"
              class="checkmark-icon"
            />
            <div v-else class="checkbox-empty"></div>
          </div>
          <div class="ticklist-content">
            <div class="ticklist-title">{{ getItemDisplayText(item) }}</div>
          </div>
        </div>
        <div v-if="items.length === 0" class="empty-state">
          <p>No items yet. <router-link :to="`/table/${props.tableId}/insert-row`">Add your first item</router-link></p>
        </div>
      </div>
    </div>
  </Section>
</template>

<style scoped>
.toolbar {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
}

.loading, .error, .warning {
  padding: 20px;
  text-align: center;
}

.error {
  color: #dc3545;
}


.ticklist-container {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
  padding: 20px;
}

.ticklist-item {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 20px;
  background: white;
  border: 2px solid #e9ecef;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s;
  min-height: 80px;
}

.ticklist-item:hover {
  border-color: #007bff;
  box-shadow: 0 2px 8px rgba(0, 123, 255, 0.15);
}

.ticklist-item.completed {
  background: #f8f9fa;
  border-color: #28a745;
  opacity: 0.7;
}

.ticklist-item.completed .ticklist-title {
  text-decoration: line-through;
  color: #6c757d;
}

.ticklist-checkbox {
  flex-shrink: 0;
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.checkmark-icon {
  width: 32px;
  height: 32px;
  color: #28a745;
}

.checkbox-empty {
  width: 24px;
  height: 24px;
  border: 2px solid #dee2e6;
  border-radius: 4px;
  background: white;
}

.ticklist-item:hover .checkbox-empty {
  border-color: #007bff;
}

.ticklist-content {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  text-align: center;
}

.ticklist-title {
  font-size: 16px;
  font-weight: 500;
  color: #212529;
  word-wrap: break-word;
  width: 100%;
}

.empty-state {
  grid-column: 1 / -1;
  text-align: center;
  padding: 40px;
  color: #6c757d;
}

.empty-state a {
  color: #007bff;
  text-decoration: none;
}

.empty-state a:hover {
  text-decoration: underline;
}
</style>
