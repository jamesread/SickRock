<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { createConnectTransport } from '@connectrpc/connect-web'
import { createClient } from '@connectrpc/connect'
import { SickRock } from '../gen/sickrock_pb'

const props = defineProps<{
  tableId: string,
  fieldDefs: Array<{ name: string; type: string; required: boolean }>,
  selectedDate?: string | null
}>()
const emit = defineEmits<{ created: [] }>()

const form = ref<Record<string, any>>({})
const loading = ref(false)
const error = ref<string | null>(null)

// Watch for changes to selectedDate prop and update form
watch(() => props.selectedDate, (newDate) => {
  if (newDate) {
    try {
      const date = new Date(newDate)
      if (!isNaN(date.getTime())) {
        // Convert to datetime-local format (YYYY-MM-DDTHH:MM)
        const year = date.getFullYear()
        const month = String(date.getMonth() + 1).padStart(2, '0')
        const day = String(date.getDate()).padStart(2, '0')
        const hours = String(date.getHours()).padStart(2, '0')
        const minutes = String(date.getMinutes()).padStart(2, '0')
        form.value.datetime_local = `${year}-${month}-${day}T${hours}:${minutes}`
        form.value.created_at_unix = Math.floor(date.getTime() / 1000)
      }
    } catch {
      // Ignore invalid dates
    }
  }
}, { immediate: true })


// Watch for changes to any datetime field and update unix timestamp
watch(() => {
  const datetimeFields: Record<string, any> = {}
  for (const field of props.fieldDefs) {
    if (field.type === 'datetime' && form.value[field.name]) {
      datetimeFields[field.name] = form.value[field.name]
    }
  }
  return datetimeFields
}, (newDateTimeFields) => {
  for (const [fieldName, value] of Object.entries(newDateTimeFields)) {
    if (value) {
      try {
        const date = new Date(value)
        if (!isNaN(date.getTime())) {
          // Store the unix timestamp for this datetime field
          form.value[`${fieldName}_unix`] = Math.floor(date.getTime() / 1000)
        }
      } catch {
        // Ignore invalid dates
      }
    }
  }
}, { deep: true })

// Format the selected date for display
const formattedDate = computed(() => {
  if (!props.selectedDate) return null
  try {
    const date = new Date(props.selectedDate)
    return date.toLocaleDateString('en-US', {
      weekday: 'long',
      year: 'numeric',
      month: 'long',
      day: 'numeric'
    })
  } catch {
    return null
  }
})

const transport = createConnectTransport({ baseUrl: '/api' })
const client = createClient(SickRock, transport)

async function submit() {
  const payload: Record<string, any> = {}
  for (const f of props.fieldDefs) {
    if (f.name === 'id') continue
    const v = form.value[f.name]
    if (v == null || v === '') continue

    if (f.type === 'datetime') {
      // For datetime fields, convert to unix timestamp
      try {
        const date = new Date(v)
        if (!isNaN(date.getTime())) {
          payload[f.name] = String(Math.floor(date.getTime() / 1000))
        }
      } catch {
        // Skip invalid dates
        continue
      }
    } else if (f.type === 'int64') {
      payload[f.name] = String(Number(v))
    } else {
      payload[f.name] = String(v)
    }
  }
  // No specific validation needed since all fields are dynamic
  loading.value = true
  error.value = null
  try {
    // Use selected date if available, otherwise use current timestamp
    let unixTimestamp: number | undefined
    if (props.selectedDate) {
      try {
        const date = new Date(props.selectedDate)
        if (!isNaN(date.getTime())) {
          unixTimestamp = Math.floor(date.getTime() / 1000)
        }
      } catch {
        // Fall back to current timestamp if selected date is invalid
        unixTimestamp = Math.floor(Date.now() / 1000)
      }
    } else {
      // Use current timestamp if no selected date
      unixTimestamp = Math.floor(Date.now() / 1000)
    }

      const createRequest: any = {
    pageId: props.tableId,
    createdAtUnix: BigInt(unixTimestamp),
    additionalFields: payload
  }

    await client.createItem(createRequest)
    form.value = {}
    emit('created')
  } catch (e) {
    error.value = String(e)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div>
    <div v-if="formattedDate" class="selected-date-info">
      <h3>Adding item for {{ formattedDate }}</h3>
      <p class="date-note">This item will be created with the selected date's timestamp.</p>
    </div>
    <form @submit.prevent="submit">
      <template v-for="f in fieldDefs" :key="f.name">
        <template v-if="f.name !== 'id'">
            <label :for="'field-' + f.name">{{ f.name }}<span v-if="f.required" aria-label="required" title="required">*</span></label>
            <input
              v-if="f.type === 'string'"
              v-model="form[f.name]"
              :placeholder="f.name"
              :id="'field-' + f.name"
              type="text"
              @keyup.enter="submit"
            />
            <input
              v-else-if="f.type === 'int64'"
              v-model.number="form[f.name]"
              :placeholder="f.name"
              :id="'field-' + f.name"
              type="number"
              @keyup.enter="submit"
            />
            <input
              v-else-if="f.type === 'datetime'"
              v-model="form[f.name]"
              :placeholder="f.name"
              :id="'field-' + f.name"
              type="datetime-local"
              @keyup.enter="submit"
            />
        </template>
      </template>

      <button type="submit" :disabled="loading">Add</button>
    </form>
    <div v-if="error">{{ error }}</div>
  </div>
</template>



<style scoped>
.selected-date-info {
  background: #e3f2fd;
  border: 1px solid #2196f3;
  border-radius: 4px;
  padding: 1rem;
  margin-bottom: 1rem;
}

.selected-date-info h3 {
  margin: 0 0 0.5rem 0;
  color: #1976d2;
  font-size: 1.1rem;
}

.date-note {
  margin: 0;
  font-size: 0.9rem;
  color: #666;
  font-style: italic;
}

form {
  grid-template-columns: max-content 1fr;
  gap: 1em;
}

input[type="datetime-local"] {
  padding: 0.5rem;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-size: 1rem;
  background-color: white;
  width: 100%;
  box-sizing: border-box;
}

input[type="datetime-local"]:focus {
  outline: none;
  border-color: #007bff;
  box-shadow: 0 0 0 2px rgba(0, 123, 255, 0.25);
}

input[type="datetime-local"]:hover {
  border-color: #999;
}

/* Ensure datetime inputs have consistent styling with other inputs */
input[type="datetime-local"]::-webkit-calendar-picker-indicator {
  cursor: pointer;
  border-radius: 4px;
  margin-right: 2px;
  opacity: 0.6;
  transition: opacity 0.2s;
}

input[type="datetime-local"]::-webkit-calendar-picker-indicator:hover {
  opacity: 1;
}

.readonly-field {
  background-color: #f5f5f5;
  color: #666;
  cursor: not-allowed;
}

.field-help {
  display: block;
  margin-top: 0.25rem;
  color: #666;
  font-size: 0.85rem;
  font-style: italic;
}

</style>
