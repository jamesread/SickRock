<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { createApiClient } from '../stores/api'
import Section from 'picocrank/vue/components/Section.vue'
import { RouterLink } from 'vue-router'

const client = createApiClient()

const database = ref('main')
const tables = ref<Array<{
  tableName: string
  hasConfiguration: boolean
  configurationName: string
  view: string
}>>([])
const loading = ref(false)
const error = ref<string | null>(null)

async function loadTables() {
  loading.value = true
  error.value = null

  try {
    const response = await client.getDatabaseTables({ database: database.value })
    tables.value = response.tables
  } catch (e) {
    error.value = String(e)
    console.error('Failed to load database tables:', e)
  } finally {
    loading.value = false
  }
}

function onDatabaseChange() {
  loadTables()
}

onMounted(() => {
  loadTables()
})
</script>

<template>
  <Section title="Database Browser">
    <div class="database-browser">
      <div class="controls">
        <div class="form-group">
          <label for="database">Database:</label>
          <input
            id="database"
            v-model="database"
            type="text"
            @change="onDatabaseChange"
            @keyup.enter="onDatabaseChange"
          />
        </div>
        <button @click="loadTables" class="button primary" :disabled="loading">
          {{ loading ? 'Loading...' : 'Refresh' }}
        </button>
      </div>

      <div v-if="error" class="error-message">
        ✗ {{ error }}
      </div>

      <div v-if="!loading && tables.length === 0" class="no-tables">
        No tables found in database "{{ database }}"
      </div>

      <table v-if="tables.length > 0" class="tables-list">
        <thead>
          <tr>
            <th>Table Name</th>
            <th>Has Configuration</th>
            <th>Configuration Name</th>
            <th>View</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="table in tables" :key="table.tableName">
            <td class="table-name">{{ table.tableName }}</td>
            <td class="status">
              <span v-if="table.hasConfiguration" class="badge configured">✓ Yes</span>
              <span v-else class="badge not-configured">✗ No</span>
            </td>
            <td>{{ table.configurationName || '-' }}</td>
            <td>{{ table.view || '-' }}</td>
            <td class="actions">
              <RouterLink
                v-if="table.hasConfiguration && table.configurationName"
                :to="`/table/${table.configurationName}`"
                class="button small"
              >
                Open
              </RouterLink>
              <RouterLink
                v-else
                :to="`/admin/table/create?table=${table.tableName}&database=${database}`"
                class="button small secondary"
              >
                Configure
              </RouterLink>
            </td>
          </tr>
        </tbody>
      </table>

      <div class="summary" v-if="tables.length > 0">
        <p>
          Total: {{ tables.length }} tables |
          Configured: {{ tables.filter(t => t.hasConfiguration).length }} |
          Unconfigured: {{ tables.filter(t => !t.hasConfiguration).length }}
        </p>
      </div>
    </div>
  </Section>
</template>

<style scoped>
.database-browser {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.controls {
  display: flex;
  gap: 1rem;
  align-items: flex-end;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  flex: 1;
  max-width: 300px;
}

.form-group label {
  font-weight: 600;
  color: #333;
}

.form-group input {
  padding: 0.75rem;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-size: 1rem;
}

.form-group input:focus {
  outline: none;
  border-color: #007bff;
  box-shadow: 0 0 0 2px rgba(0, 123, 255, 0.25);
}

.tables-list {
  width: 100%;
  border-collapse: collapse;
  background: white;
  border-radius: 8px;
  overflow: hidden;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
}

.tables-list thead {
  background: #f8f9fa;
}

.tables-list th {
  padding: 1rem;
  text-align: left;
  font-weight: 600;
  color: #495057;
  border-bottom: 2px solid #dee2e6;
}

.tables-list td {
  padding: 1rem;
  border-bottom: 1px solid #dee2e6;
}

.tables-list tbody tr:hover {
  background: #f8f9fa;
}

.table-name {
  font-family: 'Monaco', 'Courier New', monospace;
  font-weight: 500;
}

.status {
  text-align: center;
}

.badge {
  display: inline-block;
  padding: 0.25rem 0.75rem;
  border-radius: 12px;
  font-size: 0.875rem;
  font-weight: 600;
}

.badge.configured {
  background: #d4edda;
  color: #155724;
}

.badge.not-configured {
  background: #f8d7da;
  color: #721c24;
}

.actions {
  text-align: center;
}

.button.small {
  padding: 0.5rem 1rem;
  font-size: 0.875rem;
}

.button.secondary {
  background: #6c757d;
  color: white;
}

.button.secondary:hover {
  background: #5a6268;
}

.error-message {
  padding: 1rem;
  background: #f8d7da;
  border: 1px solid #f5c6cb;
  border-radius: 4px;
  color: #721c24;
}

.no-tables {
  padding: 2rem;
  text-align: center;
  color: #6c757d;
  background: #f8f9fa;
  border-radius: 4px;
}

.summary {
  padding: 1rem;
  background: #f8f9fa;
  border-radius: 4px;
  text-align: center;
  font-weight: 500;
  color: #495057;
}

.summary p {
  margin: 0;
}
</style>
