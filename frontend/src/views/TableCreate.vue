<script setup lang="ts">
import { ref, watch, onMounted, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'

import { createApiClient } from '../stores/api'
import Section from 'picocrank/vue/components/Section.vue'
import { AddIcon } from '@hugeicons/core-free-icons'
import FormLayout from 'picocrank/vue/components/FormLayout.vue'
import FormField from 'picocrank/vue/components/FormField.vue'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'

const router = useRouter()
const route = useRoute()
const name = ref('')
const database = ref('main')
const table = ref('')
const createTableInDatabase = ref(true)
const createConfiguration = ref(true)
const createNavigationEntry = ref(true)
const loading = ref(false)
const error = ref<string | null>(null)
const success = ref<string | null>(null)

const nameManuallyEdited = ref(false)
const availableDatabases = ref<string[]>([])

const client = createApiClient()

const databaseDescription = computed(() =>
  createTableInDatabase.value
    ? 'The database where the table will be created'
    : 'The database containing the existing table',
)

const createConfigurationDescription = computed(() =>
  createTableInDatabase.value
    ? 'Recommended: adds the table to SickRock navigation and configuration'
    : 'Required when configuring an existing table',
)

const submitLabel = computed(() => {
  if (loading.value) return 'Creating…'
  if (!createTableInDatabase.value) return 'Add configuration'
  return 'Create'
})

async function loadAvailableDatabases() {
  try {
    const response = await client.getTableConfigurations({})
    const databases = new Set<string>()

    for (const page of response.pages || []) {
      const dbName = page.database?.trim() || ''
      if (dbName !== '') {
        databases.add(dbName)
      }
    }

    if (!databases.has('main')) {
      databases.add('main')
    }

    availableDatabases.value = Array.from(databases).sort()
  } catch (e) {
    console.warn('Failed to load available databases:', e)
    availableDatabases.value = ['main']
  }
}

onMounted(async () => {
  await loadAvailableDatabases()

  if (route.query.table) {
    table.value = String(route.query.table)
  }
  if (route.query.database) {
    database.value = String(route.query.database)
  }
  if (route.query.table != null && route.query.table !== '') {
    createTableInDatabase.value = false
    createConfiguration.value = true
  }
})

watch(table, (newTable) => {
  if (!nameManuallyEdited.value) {
    name.value = newTable
  }
})

function onNameInput() {
  nameManuallyEdited.value = true
}

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
      const createTableResponse = await client.createTable({
        database: database.value,
        table: table.value,
      })

      if (!createTableResponse.success) {
        throw new Error(createTableResponse.message || 'Failed to create table')
      }
    }

    if (createConfiguration.value) {
      if (!name.value) {
        throw new Error('Configuration name is required when creating a table configuration')
      }

      const response = await client.createTableConfiguration({
        name: name.value,
        database: database.value,
        table: table.value,
        createNavigationEntry: createNavigationEntry.value,
      })

      if (!response.success) {
        throw new Error(response.message || 'Failed to create table configuration')
      }

      success.value = createTableInDatabase.value
        ? 'Table and configuration created successfully'
        : 'Table configuration created successfully'

      await router.push(`/table/${encodeURIComponent(name.value)}`)
    } else {
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
    :icon="AddIcon"
  >
    <FormLayout @submit.prevent="submit">
      <FormField
        label="Physical table name"
        for="table"
        label-required
        description="The actual table name in the database"
        :disabled="loading"
      >
        <input
          id="table"
          v-model="table"
          type="text"
          placeholder="Start typing here to set both fields"
          autocomplete="off"
          :disabled="loading"
          required
        />
      </FormField>

      <FormField
        label="Database"
        for="database"
        label-required
        :description="databaseDescription"
        :disabled="loading"
      >
        <input
          id="database"
          v-model="database"
          type="text"
          list="database-list"
          placeholder="Database name (default: main)"
          autocomplete="off"
          :disabled="loading"
        />
        <datalist id="database-list">
          <option v-for="db in availableDatabases" :key="db" :value="db">
            {{ db }}
          </option>
        </datalist>
      </FormField>

      <FormField
        label="Create table in database"
        for="create-table-in-database"
        description="Untick when adding a configuration for a table that already exists (e.g. from Database Browser)"
        :disabled="loading"
      >
        <input
          id="create-table-in-database"
          v-model="createTableInDatabase"
          type="checkbox"
          :disabled="loading"
        />
      </FormField>

      <FormField
        label="Create table configuration entry"
        for="create-configuration"
        :description="createConfigurationDescription"
        :disabled="loading || !createTableInDatabase"
      >
        <input
          id="create-configuration"
          v-model="createConfiguration"
          type="checkbox"
          :disabled="loading || !createTableInDatabase"
        />
      </FormField>

      <FormField
        v-if="createConfiguration"
        label="Create navigation entry"
        for="create-navigation-entry"
        description="Adds a link in the main navigation menu for this table configuration"
        :disabled="loading"
      >
        <input
          id="create-navigation-entry"
          v-model="createNavigationEntry"
          type="checkbox"
          :disabled="loading"
        />
      </FormField>

      <FormField
        label="Configuration name"
        for="name"
        :label-required="createConfiguration"
        description="The name used in URLs and references"
        :disabled="loading"
      >
        <input
          id="name"
          v-model="name"
          type="text"
          placeholder="e.g., employees, tasks, projects"
          autocomplete="off"
          :disabled="loading"
          :required="createConfiguration"
          @input="onNameInput"
        />
      </FormField>

      <p v-if="success" class="inline-notification note">{{ success }}</p>
      <NotificationBlock v-if="error" type="error" :message="error" />

      <template #actions>
        <button
          type="submit"
          class="good"
          :disabled="loading || !table || (createConfiguration && !name)"
        >
          {{ submitLabel }}
        </button>
      </template>
    </FormLayout>
  </Section>
</template>
