<script setup lang="ts">
import { ref, onMounted, inject, nextTick } from 'vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import {
  Add01Icon,
  Copy01Icon,
  Delete01Icon,
  KeyIcon,
  PauseIcon,
  RefreshIcon,
} from '@hugeicons/core-free-icons'
import type { createApiClient } from '../stores/api'
import Section from 'picocrank/vue/components/Section.vue'
import FormLayout from 'picocrank/vue/components/FormLayout.vue'
import FormField from 'picocrank/vue/components/FormField.vue'
import Table from 'picocrank/vue/components/Table.vue'
import { formatUnixTimestamp } from '../utils/dateFormatting'

const iconStrokeWidth = 2.5
const client = inject<ReturnType<typeof createApiClient>>('apiClient')!

type ApiKeyRow = {
  id: number
  userId: number
  name: string
  createdAt: number
  lastUsedAt: number
  expiresAt: number
  isActive: boolean
  readOnly: boolean
}

const apiKeys = ref<ApiKeyRow[]>([])
const loading = ref(false)
const saving = ref(false)
const errorMessage = ref('')
const createError = ref('')

const newApiKeyName = ref('')
const newApiKeyExpiresAt = ref('')
const newApiKeyReadOnly = ref(false)
const newApiKeyValue = ref('')

const createDialog = ref<HTMLDialogElement | null>(null)
const newKeyDialog = ref<HTMLDialogElement | null>(null)
const createNameInput = ref<HTMLInputElement | null>(null)

const tableHeaders = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'createdAt', label: 'Created', sortable: true, width: '10rem' },
  { key: 'lastUsedAt', label: 'Last used', sortable: true, width: '10rem' },
  { key: 'expiresAt', label: 'Expires', sortable: true, width: '10rem' },
  { key: 'isActive', label: 'Status', sortable: true, width: '7rem' },
  { key: 'readOnly', label: 'Access', sortable: true, width: '8rem' },
  { key: 'actions', label: 'Actions', sortable: false, width: '7rem' },
]

function formatDate(timestamp: number): string {
  if (timestamp === 0) return 'Never'
  return formatUnixTimestamp(timestamp)
}

function formatExpirationDate(timestamp: number): string {
  if (timestamp === 0) return 'Never expires'
  return formatUnixTimestamp(timestamp)
}

function expiresAtToUnix(value: string): bigint {
  if (!value.trim()) return 0n
  return BigInt(Math.floor(new Date(value).getTime() / 1000))
}

async function loadAPIKeys() {
  loading.value = true
  errorMessage.value = ''
  try {
    const response = await client.getAPIKeys({})
    apiKeys.value = (response.apiKeys || []).map((key) => ({
      id: key.id,
      userId: key.userId,
      name: key.name,
      createdAt: Number(key.createdAt),
      lastUsedAt: Number(key.lastUsedAt),
      expiresAt: Number(key.expiresAt),
      isActive: key.isActive,
      readOnly: key.readOnly ?? false,
    }))
  } catch (e: unknown) {
    errorMessage.value = e instanceof Error ? e.message : 'Failed to load API keys.'
  } finally {
    loading.value = false
  }
}

function resetCreateForm() {
  newApiKeyName.value = ''
  newApiKeyExpiresAt.value = ''
  newApiKeyReadOnly.value = false
  createError.value = ''
}

function openCreateDialog() {
  resetCreateForm()
  createDialog.value?.showModal()
  nextTick(() => createNameInput.value?.focus())
}

function closeCreateDialog() {
  createDialog.value?.close()
}

function openNewKeyDialog() {
  newKeyDialog.value?.showModal()
}

function closeNewKeyDialog() {
  newKeyDialog.value?.close()
  newApiKeyValue.value = ''
}

async function createAPIKey() {
  if (!newApiKeyName.value.trim()) {
    createError.value = 'API key name is required.'
    return
  }

  saving.value = true
  createError.value = ''
  try {
    const response = await client.createAPIKey({
      name: newApiKeyName.value.trim(),
      expiresAt: expiresAtToUnix(newApiKeyExpiresAt.value),
      readOnly: newApiKeyReadOnly.value,
    })

    if (response.success) {
      newApiKeyValue.value = response.apiKey
      closeCreateDialog()
      openNewKeyDialog()
      await loadAPIKeys()
    } else {
      createError.value = response.message || 'Failed to create API key.'
    }
  } catch (e: unknown) {
    createError.value = e instanceof Error ? e.message : 'Failed to create API key.'
  } finally {
    saving.value = false
  }
}

async function deleteAPIKey(apiKeyId: number) {
  if (!confirm('Are you sure you want to delete this API key? This action cannot be undone.')) {
    return
  }

  saving.value = true
  errorMessage.value = ''
  try {
    const response = await client.deleteAPIKey({ apiKeyId })
    if (response.success) {
      await loadAPIKeys()
    } else {
      errorMessage.value = response.message || 'Failed to delete API key.'
    }
  } catch (e: unknown) {
    errorMessage.value = e instanceof Error ? e.message : 'Failed to delete API key.'
  } finally {
    saving.value = false
  }
}

async function deactivateAPIKey(apiKeyId: number) {
  saving.value = true
  errorMessage.value = ''
  try {
    const response = await client.deactivateAPIKey({ apiKeyId })
    if (response.success) {
      await loadAPIKeys()
    } else {
      errorMessage.value = response.message || 'Failed to deactivate API key.'
    }
  } catch (e: unknown) {
    errorMessage.value = e instanceof Error ? e.message : 'Failed to deactivate API key.'
  } finally {
    saving.value = false
  }
}

function copyToClipboard(text: string) {
  navigator.clipboard.writeText(text).catch((err) => {
    console.error('Failed to copy to clipboard:', err)
  })
}

onMounted(loadAPIKeys)
</script>

<template>
  <dialog ref="createDialog" class="dialog" @close="resetCreateForm">
    <h2>Create API key</h2>
    <p>Create a key for programmatic access to SickRock.</p>

    <FormLayout @submit.prevent="createAPIKey">
      <FormField label="Name" for="api-key-name" :disabled="saving">
        <input
          id="api-key-name"
          ref="createNameInput"
          v-model="newApiKeyName"
          type="text"
          placeholder="e.g., My App API Key"
          autocomplete="off"
          :disabled="saving"
          required
        />
      </FormField>

      <FormField label="Expires at (optional)" for="api-key-expires" :disabled="saving">
        <input
          id="api-key-expires"
          v-model="newApiKeyExpiresAt"
          type="datetime-local"
          :disabled="saving"
        />
      </FormField>

      <FormField label="Read-only access" for="api-key-readonly" :disabled="saving">
        <input
          id="api-key-readonly"
          v-model="newApiKeyReadOnly"
          type="checkbox"
          :disabled="saving"
        />
      </FormField>

      <p v-if="createError" class="inline-notification error">{{ createError }}</p>

      <template #actions>
        <button type="button" class="neutral" :disabled="saving" @click="closeCreateDialog">Cancel</button>
        <button type="submit" class="good" :disabled="saving || !newApiKeyName.trim()">
          {{ saving ? 'Creating…' : 'Create API key' }}
        </button>
      </template>
    </FormLayout>
  </dialog>

  <dialog ref="newKeyDialog" class="dialog" @close="newApiKeyValue = ''">
    <h2>API key created</h2>
    <p class="inline-notification note">
      This is the only time you will see this API key. Copy it now and store it securely.
    </p>
    <div class="new-key-value">
      <code>{{ newApiKeyValue }}</code>
      <button type="button" class="inline-icon good" @click="copyToClipboard(newApiKeyValue)">
        <HugeiconsIcon :icon="Copy01Icon" width="1em" height="1em" :strokeWidth="iconStrokeWidth" />
        <span>Copy</span>
      </button>
    </div>
    <div class="dialog-actions">
      <button type="button" class="neutral" @click="closeNewKeyDialog">Close</button>
    </div>
  </dialog>

  <Section
    title="API Keys"
    subtitle="Create and manage API keys for programmatic access to SickRock."
    :icon="KeyIcon"
    :padding="false"
  >

    <template #toolbar>
      <button type="button" class="inline-icon neutral" aria-label="Refresh" :disabled="loading" @click="loadAPIKeys">
        <HugeiconsIcon :icon="RefreshIcon" width="1em" height="1em" :strokeWidth="iconStrokeWidth" />
      </button>
      <button
        type="button"
        class="inline-icon good"
        aria-label="Create API key"
        :disabled="loading"
        @click="openCreateDialog"
      >
        <HugeiconsIcon :icon="Add01Icon" width="1em" height="1em" :strokeWidth="iconStrokeWidth" />
      </button>
    </template>

    <div v-if="errorMessage" class="list-banner-pad inline-notification error">{{ errorMessage }}</div>
    <div v-if="loading && !apiKeys.length" class="list-banner-pad muted">Loading…</div>

    <template v-else>
      <p v-if="!apiKeys.length" class="list-banner-pad inline-notification note">No API keys yet.</p>

      <Table
        v-else
        class="api-keys-table-wrap"
        :data="apiKeys"
        :headers="tableHeaders"
      >
        <template #cell-name="{ value, row }">
          <strong :class="{ muted: !row.isActive }">{{ value }}</strong>
        </template>
        <template #cell-createdAt="{ value }">
          {{ formatDate(value) }}
        </template>
        <template #cell-lastUsedAt="{ value }">
          {{ formatDate(value) }}
        </template>
        <template #cell-expiresAt="{ value }">
          {{ formatExpirationDate(value) }}
        </template>
        <template #cell-isActive="{ value }">
          <span :class="value ? 'tag fg-good' : 'tag bad'">
            {{ value ? 'Active' : 'Inactive' }}
          </span>
        </template>
        <template #cell-readOnly="{ value }">
          {{ value ? 'Read-only' : 'Read-write' }}
        </template>
        <template #cell-actions="{ row }">
          <div class="actions-cell">
            <button
              v-if="row.isActive"
              type="button"
              class="inline-icon neutral small"
              aria-label="Deactivate API key"
              :disabled="saving"
              @click="deactivateAPIKey(row.id)"
            >
              <HugeiconsIcon :icon="PauseIcon" width="1em" height="1em" :strokeWidth="iconStrokeWidth" />
            </button>
            <button
              type="button"
              class="inline-icon bad small"
              aria-label="Delete API key"
              :disabled="saving"
              @click="deleteAPIKey(row.id)"
            >
              <HugeiconsIcon :icon="Delete01Icon" width="1em" height="1em" :strokeWidth="iconStrokeWidth" />
            </button>
          </div>
        </template>
      </Table>
    </template>
  </Section>
</template>

<style scoped>
.list-banner-pad {
  padding-left: 1em;
  padding-right: 1em;
}

.api-keys-table-wrap {
  margin-top: 0.5rem;
  margin-bottom: 1.5rem;
}

.actions-cell {
  display: flex;
  justify-content: flex-end;
  gap: 0.35rem;
}

.new-key-value {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.75rem;
  margin: 1rem 0;
}

.new-key-value code {
  flex: 1 1 16rem;
  word-break: break-all;
}

.dialog-actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
}
</style>
