<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'

import { createApiClient } from '../stores/api'
import Section from 'picocrank/vue/components/Section.vue'

const router = useRouter()
const route = useRoute()
const name = ref('')
const database = ref('main')
const table = ref('')
const createTableInDatabase = ref(true)
const createConfiguration = ref(true)
const loading = ref(false)
const error = ref<string | null>(null)
const success = ref<string | null>(null)

// Track if user has manually edited the name field
const nameManuallyEdited = ref(false)

// Available databases from table configurations
const availableDatabases = ref<string[]>([])

// Transport handled by authenticated client
const client = createApiClient()

// Load available databases from table configurations
async function loadAvailableDatabases() {
  try {
    const response = await client.getTableConfigurations({})
    const databases = new Set<string>()

    // Extract unique database names from table configurations
    for (const page of response.pages || []) {
      // Handle both null/undefined and empty strings, and trim whitespace
      const dbName = page.database?.trim() || ''
      if (dbName !== '') {
        databases.add(dbName)
      }
    }

    // Always include 'main' as it's the default
    if (!databases.has('main')) {
      databases.add('main')
    }

    availableDatabases.value = Array.from(databases).sort()

    // Debug logging to help troubleshoot
    console.log('Available databases loaded:', availableDatabases.value)
    console.log('Table configurations:', response.pages?.map(p => ({ title: p.title, database: p.database })))
  } catch (e) {
    console.warn('Failed to load available databases:', e)
    // Fallback to just 'main' if loading fails
    availableDatabases.value = ['main']
  }
}

// Pre-fill from URL query parameters if present (e.g. from Database Browser "Configure")
onMounted(async () => {
  await loadAvailableDatabases()

  if (route.query.table) {
    table.value = String(route.query.table)
  }
  if (route.query.database) {
    database.value = String(route.query.database)
  }
  // From database browser: table already exists, only add configuration
  if (route.query.table != null && route.query.table !== '') {
    createTableInDatabase.value = false
    createConfiguration.value = true
  }
})

// Watch table and automatically update name unless manually edited
watch(table, (newTable) => {
  if (!nameManuallyEdited.value) {
    name.value = newTable
  }
})

// Track manual edits to the name field
function onNameInput() {
  nameManuallyEdited.value = true
}

// Auto-fill table name if not specified
function ensureTableName() {
  if (!table.value && name.value) {
    table.value = name.value
  }
}

async function submit() {
  if (!table.value || loading.value) return
  if (!createTableInDatabase.value && (!name.value || !createConfiguration.value)) {
    error.value = 'Configuration name is required when adding a configuration for an existing table'
    return
  }

  ensureTableName()

  loading.value = true
  error.value = null
  success.value = null

  try {
    if (createTableInDatabase.value) {
      // Create the physical table in the database
      const createTableResponse = await client.createTable({
        database: database.value,
        table: table.value
      })

      if (!createTableResponse.success) {
        throw new Error(createTableResponse.message || 'Failed to create table')
      }
    }

    if (createConfiguration.value) {
      if (!name.value) {
        throw new Error('Configuration name is required when creating a table configuration')
      }

      // Create the table configuration entry
      const response = await client.createTableConfiguration({
        name: name.value,
        database: database.value,
        table: table.value
      })

      if (!response.success) {
        throw new Error(response.message || 'Failed to create table configuration')
      }

      success.value = createTableInDatabase.value
        ? 'Table and configuration created successfully'
        : 'Table configuration created successfully'

      // Navigate to the table view
      await router.push(`/table/${encodeURIComponent(name.value)}`)
    } else {
      // Only reachable when createTableInDatabase is true: table created, no config
      await router.push(`/table/${encodeURIComponent(table.value)}`)
    }
  } catch (e) {
    error.value = String(e)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <Section
    title="Create Table"
    subtitle="Create a new table in the database or add a configuration for an existing table"
  >
    <form @submit.prevent="submit">
        <label for="table">Physical Table Name *</label>
        <input
          id="table"
          v-model="table"
          type="text"
          placeholder="Start typing here to set both fields"
          @keyup.enter="submit"
          required
        />
        <small>The actual table name in the database</small>

        <label for="database">Database *</label>
        <input
          id="database"
          v-model="database"
          type="text"
          list="database-list"
          placeholder="Database name (default: main)"
          @keyup.enter="submit"
        />
        <datalist id="database-list">
          <option v-for="db in availableDatabases" :key="db" :value="db">
            {{ db }}
          </option>
        </datalist>
        <small>{{ createTableInDatabase ? 'The database where the table will be created' : 'The database containing the existing table' }}</small>

        <div>&nbsp;</div>
        <label>
          <input
            type="checkbox"
            v-model="createTableInDatabase"
          />
          Create table in database
        </label>
        <small>Untick when adding a configuration for a table that already exists (e.g. from Database Browser)</small>

        <div>&nbsp;</div>
        <label>
          <input
            type="checkbox"
            v-model="createConfiguration"
            :disabled="!createTableInDatabase"
          />
          Create table configuration entry
        </label>
        <small v-if="createTableInDatabase">Recommended: Adds the table to SickRock's navigation and configuration system</small>
        <small v-else>Required when configuring an existing table</small>

        <label for="name">Configuration Name *</label>
        <input
          id="name"
          v-model="name"
          type="text"
          placeholder="e.g., employees, tasks, projects"
          @input="onNameInput"
          @keyup.enter="submit"
          :required="createConfiguration"
        />
        <small>This is the name used in URLs and references</small>

        <button
          type="submit"
          :disabled="loading || !table || (createConfiguration && !name)"
        >
          {{
            loading
              ? 'Creating...'
              : !createTableInDatabase
                ? 'Add configuration'
                : 'Create'
          }}
        </button>

      <div v-if="success" class="form-result success">✓ {{ success }}</div>
      <div v-if="error" class="form-result error">✗ {{ error }}</div>
    </form>
  </Section>
</template>

<style scoped>
form {
  grid-template-columns: 200px max-content 1fr;
}
</style>
