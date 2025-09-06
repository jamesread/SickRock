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
const loading = ref(false)
const error = ref<string | null>(null)
const showDeleteConfirm = ref(false)
const deleting = ref(false)

onMounted(async () => {
  loading.value = true
  try {
    const res = await client.listItems({ pageId: tableName })
    const found = (res.items as any[] | undefined)?.find((it) => String(it.id) === String(rowId))
    item.value = found ?? null
  } catch (e) {
    error.value = String(e)
  } finally {
    loading.value = false
  }
})

const entries = computed(() => {
  if (!item.value) return []

  return Object.entries(item.value).map(([key, value]) => {
    let displayValue = value
    let valueClass = ''

    // Handle different data types
    if (value === null || value === undefined) {
      displayValue = '(empty)'
      valueClass = 'empty'
    } else if (typeof value === 'bigint') {
      displayValue = Number(value)
    } else if (typeof value === 'object') {
      displayValue = JSON.stringify(value, null, 2)
      valueClass = 'json'
    } else if (typeof value === 'boolean') {
      displayValue = value ? 'Yes' : 'No'
      valueClass = value ? 'boolean-true' : 'boolean-false'
    }

    return [key, displayValue, valueClass]
  })
})

async function deleteItem() {
  deleting.value = true
  error.value = null
  try {
    await client.deleteItem({ pageId: tableName, id: rowId })
    // Navigate back to the table after successful deletion
    router.push({ name: 'table', params: { tableName } })
  } catch (e) {
    error.value = String(e)
  } finally {
    deleting.value = false
    showDeleteConfirm.value = false
  }
}

function confirmDelete() {
  showDeleteConfirm.value = true
}

function cancelDelete() {
  showDeleteConfirm.value = false
}
</script>

<template>
  <section class="with-header-and-content">
    <div class="section-header">
      <h2>Row {{ rowId }}</h2>
      <div class="header-actions">
        <router-link
          :to="`/table/${tableName}`"
          class="button back-button"
        >
          ← Back to Table
        </router-link>
        <router-link
          :to="`/table/${tableName}/${rowId}/edit`"
          class="button edit-button"
        >
          ✏️ Edit Row
        </router-link>
        <button
          @click="confirmDelete"
          class="button delete-button"
          :disabled="deleting"
        >
          🗑️ Delete Row
        </button>
      </div>
    </div>
    <div class="section-content padding">
      <div v-if="error">{{ error }}</div>
      <div v-else-if="loading">Loading…</div>
      <dl v-else-if="entries.length > 0">
        <template v-for="[k, v, valueClass] in entries" :key="k">
          <dt>{{ k }}</dt>
          <dd :class="valueClass">{{ v }}</dd>
        </template>
      </dl>
      <div v-else class="no-data">
        <p>No data available for this row.</p>
      </div>
    </div>

    <!-- Delete Confirmation Dialog -->
    <div v-if="showDeleteConfirm" class="modal-overlay" @click="cancelDelete">
      <div class="modal-content" @click.stop>
        <h3>Confirm Delete</h3>
        <p>Are you sure you want to delete this row? This action cannot be undone.</p>
        <div class="modal-actions">
          <button @click="cancelDelete" class="button cancel-button" :disabled="deleting">
            Cancel
          </button>
          <button @click="deleteItem" class="button confirm-delete-button" :disabled="deleting">
            {{ deleting ? 'Deleting...' : 'Delete' }}
          </button>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1rem;
}

.section-header h2 {
  margin: 0;
}

.header-actions {
  display: flex;
  gap: 0.5rem;
  align-items: center;
}

.edit-button {
  background: #007bff;
  color: white;
  text-decoration: none;
  padding: 0.5rem 1rem;
  border-radius: 4px;
  font-size: 0.9rem;
  font-weight: 500;
  transition: background-color 0.2s;
  border: none;
  cursor: pointer;
}

.edit-button:hover {
  background: #0056b3;
  color: white;
  text-decoration: none;
}

.edit-button:focus {
  outline: 2px solid #007bff;
  outline-offset: 2px;
}

.back-button {
  background: #6c757d;
  color: white;
  text-decoration: none;
  padding: 0.5rem 1rem;
  border-radius: 4px;
  font-size: 0.9rem;
  font-weight: 500;
  transition: background-color 0.2s;
  border: none;
  cursor: pointer;
}

.back-button:hover {
  background: #545b62;
  color: white;
  text-decoration: none;
}

.back-button:focus {
  outline: 2px solid #6c757d;
  outline-offset: 2px;
}

.delete-button {
  background: #dc3545;
  color: white;
  text-decoration: none;
  padding: 0.5rem 1rem;
  border-radius: 4px;
  font-size: 0.9rem;
  font-weight: 500;
  transition: background-color 0.2s;
  border: none;
  cursor: pointer;
}

.delete-button:hover:not(:disabled) {
  background: #c82333;
  color: white;
}

.delete-button:disabled {
  background: #6c757d;
  cursor: not-allowed;
  opacity: 0.6;
}

.delete-button:focus {
  outline: 2px solid #dc3545;
  outline-offset: 2px;
}

/* Responsive design */
@media (max-width: 768px) {
  .section-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 1rem;
  }

  .header-actions {
    width: 100%;
    justify-content: flex-end;
  }
}

/* Modal Dialog Styles */
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.modal-content {
  background: white;
  border-radius: 8px;
  padding: 2rem;
  max-width: 400px;
  width: 90%;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.3);
}

.modal-content h3 {
  margin: 0 0 1rem 0;
  color: #dc3545;
  font-size: 1.25rem;
}

.modal-content p {
  margin: 0 0 1.5rem 0;
  color: #666;
  line-height: 1.5;
}

.modal-actions {
  display: flex;
  gap: 1rem;
  justify-content: flex-end;
}

.cancel-button {
  background: #6c757d;
  color: white;
  border: none;
  padding: 0.5rem 1rem;
  border-radius: 4px;
  font-size: 0.9rem;
  font-weight: 500;
  cursor: pointer;
  transition: background-color 0.2s;
}

.cancel-button:hover:not(:disabled) {
  background: #545b62;
}

.cancel-button:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.confirm-delete-button {
  background: #dc3545;
  color: white;
  border: none;
  padding: 0.5rem 1rem;
  border-radius: 4px;
  font-size: 0.9rem;
  font-weight: 500;
  cursor: pointer;
  transition: background-color 0.2s;
}

.confirm-delete-button:hover:not(:disabled) {
  background: #c82333;
}

.confirm-delete-button:disabled {
  background: #6c757d;
  cursor: not-allowed;
  opacity: 0.6;
}

@media (max-width: 768px) {
  .modal-actions {
    flex-direction: column;
  }

  .modal-actions .button {
    width: 100%;
  }
}
</style>
