<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { createConnectTransport } from '@connectrpc/connect-web'
import { createClient } from '@connectrpc/connect'
import { SickRock } from '../gen/sickrock_pb'

const route = useRoute()
const router = useRouter()
const tableName = route.params.tableName as string
const rowId = route.params.rowId as string

const transport = createConnectTransport({ baseUrl: '/api' })
const client = createClient(SickRock, transport)

const item = ref<Record<string, unknown> | null>(null)
const tableStructure = ref<Array<{ name: string; type: string; required: boolean }>>([])
const loading = ref(false)
const saving = ref(false)
const error = ref<string | null>(null)

// Form data - reactive object to hold all field values
const formData = ref<Record<string, string>>({})

onMounted(async () => {
  loading.value = true
  try {
    // Load the item data
    const res = await client.listItems({ pageId: tableName })
    const found = (res.items as any[] | undefined)?.find((it) => String(it.id) === String(rowId))
    item.value = found ?? null

    // Load table structure
    const structureRes = await client.getTableStructure({ pageId: tableName })
    tableStructure.value = (structureRes.fields ?? []).map(f => ({
      name: f.name,
      type: f.type,
      required: !!f.required
    }))

    // Initialize form data with current values
    if (item.value) {
      const initialData: Record<string, string> = {}
      
      // Handle standard fields
      if (item.value.name !== undefined) {
        initialData.name = String(item.value.name)
      }
      
      // Handle additional fields
      if (item.value.additionalFields) {
        Object.entries(item.value.additionalFields).forEach(([key, value]) => {
          initialData[key] = String(value)
        })
      }
      
      formData.value = initialData
    }
  } catch (e) {
    error.value = String(e)
  } finally {
    loading.value = false
  }
})

const editableFields = computed(() => {
  // Filter out non-editable fields like id and created_at_unix
  return tableStructure.value.filter(field => 
    field.name !== 'id' && field.name !== 'created_at_unix'
  )
})

async function saveChanges() {
  saving.value = true
  error.value = null
  
  try {
    // Prepare additional fields (exclude name since it's handled separately)
    const additionalFields: Record<string, string> = {}
    Object.entries(formData.value).forEach(([key, value]) => {
      if (key !== 'name' && value !== undefined && value !== null) {
        additionalFields[key] = String(value)
      }
    })
    
    await client.editItem({
      id: rowId,
      name: formData.value.name || '',
      additionalFields: additionalFields,
      pageId: tableName
    })
    
    // Navigate back to the row view
    router.push(`/table/${tableName}/${rowId}`)
  } catch (e) {
    error.value = String(e)
  } finally {
    saving.value = false
  }
}

function cancelEdit() {
  router.push(`/table/${tableName}/${rowId}`)
}

// Helper function to get input type based on field type
function getInputType(fieldType: string): string {
  switch (fieldType) {
    case 'int64':
      return 'number'
    case 'string':
    default:
      return 'text'
  }
}
</script>

<template>
  <section class="with-header-and-content">
    <div class="section-header">
      <h2>Edit Row {{ rowId }}</h2>
      <div>
        <button @click="cancelEdit" :disabled="saving">Cancel</button>
        <button @click="saveChanges" :disabled="saving" class="primary">
          {{ saving ? 'Saving...' : 'Save Changes' }}
        </button>
      </div>
    </div>
    
    <div class="section-content">
      <div v-if="error" class="error">{{ error }}</div>
      <div v-else-if="loading">Loading…</div>
      <form v-else @submit.prevent="saveChanges">
        <div class="form-group" v-for="field in editableFields" :key="field.name">
          <label :for="field.name">
            {{ field.name }}
            <span v-if="field.required" class="required">*</span>
          </label>
          <input
            :id="field.name"
            v-model="formData[field.name]"
            :type="getInputType(field.type)"
            :required="field.required"
            :placeholder="field.required ? 'Required' : 'Optional'"
          />
        </div>
        
        <div class="form-actions">
          <button type="button" @click="cancelEdit" :disabled="saving">Cancel</button>
          <button type="submit" :disabled="saving" class="primary">
            {{ saving ? 'Saving...' : 'Save Changes' }}
          </button>
        </div>
      </form>
    </div>
  </section>
</template>

<style scoped>
.form-group {
  margin-bottom: 1rem;
}

.form-group label {
  display: block;
  margin-bottom: 0.5rem;
  font-weight: bold;
}

.form-group input {
  width: 100%;
  padding: 0.5rem;
  border: 1px solid #ccc;
  border-radius: 4px;
  font-size: 1rem;
}

.form-group input:focus {
  outline: none;
  border-color: #007bff;
  box-shadow: 0 0 0 2px rgba(0, 123, 255, 0.25);
}

.required {
  color: #dc3545;
}

.form-actions {
  margin-top: 2rem;
  display: flex;
  gap: 1rem;
}

.form-actions button {
  padding: 0.75rem 1.5rem;
  border: 1px solid #ccc;
  border-radius: 4px;
  background: white;
  cursor: pointer;
  font-size: 1rem;
}

.form-actions button.primary {
  background: #007bff;
  color: white;
  border-color: #007bff;
}

.form-actions button:hover:not(:disabled) {
  background: #f8f9fa;
}

.form-actions button.primary:hover:not(:disabled) {
  background: #0056b3;
}

.form-actions button:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.section-header h2 {
  margin: 0;
}

.section-header > div {
  display: flex;
  gap: 1rem;
}
</style>
