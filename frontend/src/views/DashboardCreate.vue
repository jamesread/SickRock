<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'

import { createApiClient } from '../stores/api'
import Section from 'picocrank/vue/components/Section.vue'
import { LayoutIcon } from '@hugeicons/core-free-icons'
import FormLayout from 'picocrank/vue/components/FormLayout.vue'
import FormField from 'picocrank/vue/components/FormField.vue'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'

const router = useRouter()
const name = ref('')
const createNavigationEntry = ref(true)
const loading = ref(false)
const error = ref<string | null>(null)
const success = ref<string | null>(null)

const client = createApiClient()

const submitLabel = computed(() => (loading.value ? 'Creating…' : 'Create dashboard'))

async function submit() {
  const trimmed = name.value.trim()
  if (!trimmed || loading.value) return

  loading.value = true
  error.value = null
  success.value = null

  try {
    const response = await client.createDashboard({
      name: trimmed,
      createNavigationEntry: createNavigationEntry.value,
    })

    if (!response.success) {
      throw new Error(response.message || 'Failed to create dashboard')
    }

    success.value = 'Dashboard created successfully'
    await router.push(`/dashboard/${encodeURIComponent(trimmed)}`)
  } catch (e) {
    error.value = String(e)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <Section
    title="Create Dashboard"
    subtitle="Add a new dashboard and optionally show it in the main navigation"
    :icon="LayoutIcon"
  >
    <FormLayout @submit.prevent="submit">
      <FormField
        label="Dashboard name"
        for="dashboard-name"
        label-required
        description="Used in URLs (/dashboard/…) and navigation labels"
        :disabled="loading"
      >
        <input
          id="dashboard-name"
          v-model="name"
          type="text"
          placeholder="e.g. sales_overview, home"
          autocomplete="off"
          :disabled="loading"
          required
        />
      </FormField>

      <FormField
        label="Create navigation entry"
        for="create-navigation-entry"
        description="Adds a link in the main navigation menu for this dashboard"
        :disabled="loading"
      >
        <input
          id="create-navigation-entry"
          v-model="createNavigationEntry"
          type="checkbox"
          :disabled="loading"
        />
      </FormField>

      <p v-if="success" class="inline-notification note">{{ success }}</p>
      <NotificationBlock v-if="error" type="error" :message="error" />

      <template #actions>
        <button type="submit" class="good" :disabled="loading || !name.trim()">
          {{ submitLabel }}
        </button>
      </template>
    </FormLayout>
  </Section>
</template>
