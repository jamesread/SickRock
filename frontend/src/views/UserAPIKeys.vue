<script setup lang="ts">
import RadioGroup from 'picocrank/vue/components/RadioGroup.vue'
import { YES_NO_OPTIONS } from '../utils/fieldTypes'
import { ref, onMounted, inject, nextTick } from 'vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import {
  Add01Icon,
  Copy01Icon,
  Delete01Icon,
  Edit02Icon,
  KeyIcon,
  PauseIcon,
  RefreshIcon,
} from '@hugeicons/core-free-icons'
import type { createApiClient } from '../stores/api'
import Section from 'picocrank/vue/components/Section.vue'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'
import FormLayout from 'picocrank/vue/components/FormLayout.vue'
import Dialog from 'picocrank/vue/components/Dialog.vue'
import FormField from 'picocrank/vue/components/FormField.vue'
import Table from 'picocrank/vue/components/Table.vue'
import ApiKeyScopeFields from '../components/ApiKeyScopeFields.vue'
import { formatUnixTimestamp } from '../utils/dateFormatting'
import type { DialogApi } from '../utils/dialog'
import {
  dateTimeLocalToUnix,
  emptyScope,
  scopeFromApi,
  scopeSummary,
  unixToDateTimeLocal,
  type ApiKeyResourceOption,
  type ApiKeyScope,
} from '../utils/apiKeyScope'

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
  scope: ApiKeyScope
}

const apiKeys = ref<ApiKeyRow[]>([])
const availablePermissions = ref<string[]>([])
const availableResources = ref<ApiKeyResourceOption[]>([])
const loading = ref(false)
const saving = ref(false)
const errorMessage = ref('')
const formError = ref('')

// The key dialog creates a key (editingKeyId null) or edits one. The secret never changes.
const editingKeyId = ref<number | null>(null)
const formName = ref('')
const formExpiresAt = ref('')
const formReadOnly = ref(false)
const formScope = ref<ApiKeyScope>(emptyScope())
const newApiKeyValue = ref('')

const keyDialog = ref<DialogApi | null>(null)
const newKeyDialog = ref<DialogApi | null>(null)
const nameInput = ref<HTMLInputElement | null>(null)

const tableHeaders = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'createdAt', label: 'Created', sortable: true, width: '10rem' },
  { key: 'lastUsedAt', label: 'Last used', sortable: true, width: '10rem' },
  { key: 'expiresAt', label: 'Expires', sortable: true, width: '10rem' },
  { key: 'isActive', label: 'Status', sortable: true, width: '7rem' },
  { key: 'readOnly', label: 'Access', sortable: false },
  { key: 'actions', label: 'Actions', sortable: false, width: '9rem' },
]

function formatDate(timestamp: number): string {
  if (timestamp === 0) return 'Never'
  return formatUnixTimestamp(timestamp)
}

function formatExpirationDate(timestamp: number): string {
  if (timestamp === 0) return 'Never expires'
  return formatUnixTimestamp(timestamp)
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
      scope: scopeFromApi(key.scope),
    }))
    availablePermissions.value = [...(response.availablePermissions || [])]
    availableResources.value = (response.availableResources || []).map((r) => ({ type: r.type, key: r.key, label: r.label }))
  } catch (e: unknown) {
    errorMessage.value = e instanceof Error ? e.message : 'Failed to load API keys.'
  } finally {
    loading.value = false
  }
}

function resetForm() {
  editingKeyId.value = null
  formName.value = ''
  formExpiresAt.value = ''
  formReadOnly.value = false
  formScope.value = emptyScope()
  formError.value = ''
}

function showKeyDialog() {
  keyDialog.value?.show()
  nextTick(() => nameInput.value?.focus())
}

function openCreateDialog() {
  resetForm()
  showKeyDialog()
}

function openEditDialog(row: ApiKeyRow) {
  resetForm()
  editingKeyId.value = row.id
  formName.value = row.name
  formExpiresAt.value = unixToDateTimeLocal(row.expiresAt)
  formReadOnly.value = row.readOnly
  formScope.value = scopeFromApi(row.scope)
  showKeyDialog()
}

function scopeToApi(scope: ApiKeyScope) {
  return {
    restrictPermissions: scope.restrictPermissions,
    permissions: scope.restrictPermissions ? scope.permissions : [],
    restrictResources: scope.restrictResources,
    resources: scope.restrictResources ? scope.resources : [],
  }
}

async function saveKey() {
  const name = formName.value.trim()
  if (!name) {
    formError.value = 'API key name is required.'
    return
  }

  saving.value = true
  formError.value = ''
  const expiresAt = BigInt(dateTimeLocalToUnix(formExpiresAt.value))
  const scope = scopeToApi(formScope.value)
  try {
    if (editingKeyId.value === null) {
      const response = await client.createAPIKey({ name, expiresAt, readOnly: formReadOnly.value, scope })
      newApiKeyValue.value = response.apiKey
      keyDialog.value?.close()
      newKeyDialog.value?.show()
    } else {
      await client.updateAPIKey({ apiKeyId: editingKeyId.value, name, expiresAt, readOnly: formReadOnly.value, scope })
      keyDialog.value?.close()
    }
    await loadAPIKeys()
  } catch (e: unknown) {
    formError.value = e instanceof Error ? e.message : 'Failed to save API key.'
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
  <Dialog ref="keyDialog" :title="editingKeyId === null ? 'Create API key' : 'Edit API key'" @close="resetForm">
    <p v-if="editingKeyId === null">Create a key for programmatic access to SickRock.</p>
    <p v-else>Changes apply to the existing key straight away; its secret stays the same.</p>

    <FormLayout id="key-dialog-form" @submit.prevent="saveKey">
      <FormField label="Name" for="api-key-name" :disabled="saving">
        <input
          id="api-key-name"
          ref="nameInput"
          v-model="formName"
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
          v-model="formExpiresAt"
          type="datetime-local"
          :disabled="saving"
        />
      </FormField>

      <FormField
        label="Read-only access"
        component-has-label
        description="Read-only keys cannot change anything, whatever their permissions."
        :disabled="saving"
      >
        <RadioGroup
          variant="boolean"
          :options="YES_NO_OPTIONS"
          v-model="formReadOnly"
          :disabled="saving"
          aria-label="Read-only access"
        />
      </FormField>

      <ApiKeyScopeFields
        v-model="formScope"
        :available-permissions="availablePermissions"
        :available-resources="availableResources"
        :disabled="saving"
      />

      <NotificationBlock v-if="formError" type="error" :message="formError" />
    </FormLayout>

    <template #actions>
      <button type="submit" form="key-dialog-form" class="good" :disabled="saving || !formName.trim()">
        <template v-if="editingKeyId === null">{{ saving ? 'Creating…' : 'Create API key' }}</template>
        <template v-else>{{ saving ? 'Saving…' : 'Save changes' }}</template>
      </button>
    </template>
  </Dialog>

  <Dialog ref="newKeyDialog" title="API key created" @close="newApiKeyValue = ''">
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
  </Dialog>

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

    <div v-if="errorMessage" class="list-banner-pad">
      <NotificationBlock type="error" :message="errorMessage" />
    </div>
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
            {{ value ? 'Active' : 'Expired' }}
          </span>
        </template>
        <template #cell-readOnly="{ row }">
          {{ scopeSummary(row.scope, row.readOnly) }}
        </template>
        <template #cell-actions="{ row }">
          <div class="actions-cell">
            <button
              type="button"
              class="inline-icon neutral small"
              aria-label="Edit API key"
              :disabled="saving"
              @click="openEditDialog(row)"
            >
              <HugeiconsIcon :icon="Edit02Icon" width="1em" height="1em" :strokeWidth="iconStrokeWidth" />
            </button>
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
</style>
