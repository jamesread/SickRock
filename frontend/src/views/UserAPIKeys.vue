<script setup lang="ts">
import { ref, onMounted, inject } from 'vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import * as Hugeicons from '@hugeicons/core-free-icons'
import { Delete01Icon, Copy01Icon, PauseIcon } from '@hugeicons/core-free-icons'
import type { createApiClient } from '../stores/api'
import Section from 'picocrank/vue/components/Section.vue'
import { formatUnixTimestamp } from '../utils/dateFormatting'

// Use global API client
const client = inject<ReturnType<typeof createApiClient>>('apiClient')

// API Keys state
const apiKeys = ref<Array<{
  id: number;
  userId: number;
  name: string;
  createdAt: number;
  lastUsedAt: number;
  expiresAt: number;
  isActive: boolean;
}>>([])

const apiKeysLoading = ref(false)
const creatingApiKey = ref(false)
const newApiKeyName = ref('')
const newApiKeyExpiresAt = ref(0)
const showNewApiKey = ref(false)
const newApiKeyValue = ref('')
const apiKeyError = ref<string | null>(null)

async function loadAPIKeys() {
  apiKeysLoading.value = true
  apiKeyError.value = null
  try {
    const response = await client.getAPIKeys({})
    apiKeys.value = (response.apiKeys || []).map(key => ({
      id: key.id,
      userId: key.userId,
      name: key.name,
      createdAt: Number(key.createdAt),
      lastUsedAt: Number(key.lastUsedAt),
      expiresAt: Number(key.expiresAt),
      isActive: key.isActive
    }))
  } catch (e: any) {
    console.error('Failed to load API keys:', e)
    apiKeyError.value = String(e?.message || e)
  } finally {
    apiKeysLoading.value = false
  }
}

async function createAPIKey() {
  if (!newApiKeyName.value.trim()) {
    apiKeyError.value = 'API key name is required'
    return
  }

  creatingApiKey.value = true
  apiKeyError.value = null
  try {
    const response = await client.createAPIKey({
      name: newApiKeyName.value.trim(),
      expiresAt: BigInt(newApiKeyExpiresAt.value)
    })

    if (response.success) {
      newApiKeyValue.value = response.apiKey
      showNewApiKey.value = true
      newApiKeyName.value = ''
      newApiKeyExpiresAt.value = 0
      await loadAPIKeys() // Reload API keys
    } else {
      apiKeyError.value = response.message || 'Failed to create API key'
    }
  } catch (e: any) {
    console.error('Failed to create API key:', e)
    apiKeyError.value = String(e?.message || e)
  } finally {
    creatingApiKey.value = false
  }
}

async function deleteAPIKey(apiKeyId: number) {
  if (!confirm('Are you sure you want to delete this API key? This action cannot be undone.')) {
    return
  }

  try {
    const response = await client.deleteAPIKey({ apiKeyId })
    if (response.success) {
      await loadAPIKeys() // Reload API keys
    } else {
      apiKeyError.value = response.message || 'Failed to delete API key'
    }
  } catch (e: any) {
    console.error('Failed to delete API key:', e)
    apiKeyError.value = String(e?.message || e)
  }
}

async function deactivateAPIKey(apiKeyId: number) {
  try {
    const response = await client.deactivateAPIKey({ apiKeyId })
    if (response.success) {
      await loadAPIKeys() // Reload API keys
    } else {
      apiKeyError.value = response.message || 'Failed to deactivate API key'
    }
  } catch (e: any) {
    console.error('Failed to deactivate API key:', e)
    apiKeyError.value = String(e?.message || e)
  }
}

function copyToClipboard(text: string) {
  navigator.clipboard.writeText(text).then(() => {
    // Could add a toast notification here
    console.log('API key copied to clipboard')
  }).catch(err => {
    console.error('Failed to copy to clipboard:', err)
  })
}

function formatDate(timestamp: number): string {
  if (timestamp === 0) return 'Never'
  return formatUnixTimestamp(timestamp)
}

function formatExpirationDate(timestamp: number): string {
  if (timestamp === 0) return 'Never expires'
  return formatUnixTimestamp(timestamp)
}

onMounted(async () => {
  await loadAPIKeys()
})
</script>

<template>
  <Section title="API Keys">
    <div v-if="apiKeyError" class="error">{{ apiKeyError }}</div>

    <!-- Create New API Key -->
    <div class="create-api-key">
      <h3>Create New API Key</h3>
      <div class="create-form">
        <div class="form-group">
          <label for="api-key-name">Name</label>
          <input
            id="api-key-name"
            v-model="newApiKeyName"
            type="text"
            placeholder="e.g., My App API Key"
            :disabled="creatingApiKey"
          />
        </div>
        <div class="form-group">
          <label for="api-key-expires">Expires At (optional)</label>
          <input
            id="api-key-expires"
            v-model="newApiKeyExpiresAt"
            type="datetime-local"
            :disabled="creatingApiKey"
          />
        </div>
        <button
          @click="createAPIKey"
          :disabled="creatingApiKey || !newApiKeyName.trim()"
          class="create-button"
        >
          {{ creatingApiKey ? 'Creating...' : 'Create API Key' }}
        </button>
      </div>
    </div>

    <!-- New API Key Display -->
    <div v-if="showNewApiKey" class="new-api-key-display">
      <h3>New API Key Created</h3>
      <div class="api-key-warning">
        <strong>Important:</strong> This is the only time you'll see this API key. Copy it now and store it securely.
      </div>
      <div class="api-key-value">
        <code>{{ newApiKeyValue }}</code>
        <button @click="copyToClipboard(newApiKeyValue)" class="copy-button">
          <HugeiconsIcon :icon="Hugeicons.Copy01Icon" />
          Copy
        </button>
      </div>
      <button @click="showNewApiKey = false" class="close-button">Close</button>
    </div>

    <!-- Current API Keys -->
    <div class="current-api-keys">
      <h3>Current API Keys</h3>
      <div v-if="apiKeysLoading" class="loading">Loading API keys...</div>
      <div v-else-if="apiKeys.length === 0" class="no-api-keys">
        <p>No API keys yet. Create one above to get started.</p>
      </div>
      <div v-else class="api-keys-list">
        <div
          v-for="apiKey in apiKeys"
          :key="apiKey.id"
          class="api-key-item"
          :class="{ inactive: !apiKey.isActive }"
        >
          <div class="api-key-content">
            <div class="api-key-info">
              <div class="api-key-name">{{ apiKey.name }}</div>
              <div class="api-key-details">
                <span class="detail-item">
                  <strong>Created:</strong> {{ formatDate(apiKey.createdAt) }}
                </span>
                <span class="detail-item">
                  <strong>Last Used:</strong> {{ formatDate(apiKey.lastUsedAt) }}
                </span>
                <span class="detail-item">
                  <strong>Expires:</strong> {{ formatExpirationDate(apiKey.expiresAt) }}
                </span>
                <span class="detail-item">
                  <strong>Status:</strong>
                  <span :class="apiKey.isActive ? 'status-active' : 'status-inactive'">
                    {{ apiKey.isActive ? 'Active' : 'Inactive' }}
                  </span>
                </span>
              </div>
            </div>
          </div>
          <div class="api-key-actions">
            <button
              v-if="apiKey.isActive"
              @click="deactivateAPIKey(apiKey.id)"
              class="deactivate-button"
              title="Deactivate API key"
            >
              <HugeiconsIcon :icon="Hugeicons.PauseIcon" />
            </button>
            <button
              @click="deleteAPIKey(apiKey.id)"
              class="delete-button"
              title="Delete API key"
            >
              <HugeiconsIcon :icon="Hugeicons.Delete01Icon" />
            </button>
          </div>
        </div>
      </div>
    </div>
  </Section>
</template>

<style scoped>
.error {
  background: #f8d7da;
  color: #721c24;
  padding: 0.75rem;
  border: 1px solid #f5c6cb;
  border-radius: 4px;
  margin-bottom: 1rem;
}

.create-api-key {
  margin-bottom: 2rem;
  padding: 1.5rem;
  background: #f8f9fa;
  border: 1px solid #e9ecef;
  border-radius: 6px;
}

.create-api-key h3 {
  margin: 0 0 1rem 0;
  color: #333;
}

.create-form {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.form-group {
  margin-bottom: 0;
}

.form-group label {
  display: block;
  margin-bottom: 0.5rem;
  font-weight: 500;
  color: #333;
}

.create-form input {
  width: 100%;
  max-width: 400px;
  padding: 0.5rem;
  border: 1px solid #ccc;
  border-radius: 4px;
  background: white;
}

.create-button {
  padding: 0.75rem 1.5rem;
  background: #28a745;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-weight: 500;
  transition: background-color 0.2s ease;
  align-self: flex-start;
}

.create-button:hover:not(:disabled) {
  background: #218838;
}

.create-button:disabled {
  background: #6c757d;
  cursor: not-allowed;
}

.new-api-key-display {
  margin-bottom: 2rem;
  padding: 1.5rem;
  background: #d4edda;
  border: 1px solid #c3e6cb;
  border-radius: 6px;
}

.new-api-key-display h3 {
  margin: 0 0 1rem 0;
  color: #155724;
}

.api-key-warning {
  background: #fff3cd;
  color: #856404;
  padding: 0.75rem;
  border: 1px solid #ffeaa7;
  border-radius: 4px;
  margin-bottom: 1rem;
}

.api-key-value {
  display: flex;
  align-items: center;
  gap: 1rem;
  margin-bottom: 1rem;
}

.api-key-value code {
  flex: 1;
  padding: 0.75rem;
  background: #f8f9fa;
  border: 1px solid #e9ecef;
  border-radius: 4px;
  font-family: 'Courier New', monospace;
  font-size: 0.9rem;
  word-break: break-all;
}

.copy-button {
  padding: 0.5rem 1rem;
  background: #007bff;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 0.5rem;
  transition: background-color 0.2s ease;
}

.copy-button:hover {
  background: #0056b3;
}

.close-button {
  padding: 0.5rem 1rem;
  background: #6c757d;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  transition: background-color 0.2s ease;
}

.close-button:hover {
  background: #545b62;
}

.current-api-keys {
  margin-bottom: 2rem;
}

.current-api-keys h3 {
  margin: 0 0 1rem 0;
  color: #333;
}

.loading {
  color: #666;
  font-style: italic;
  padding: 1rem;
}

.no-api-keys {
  color: #666;
  font-style: italic;
  padding: 2rem;
  text-align: center;
  background: #f8f9fa;
  border: 1px solid #e9ecef;
  border-radius: 6px;
}

.api-keys-list {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.api-key-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1rem;
  background: white;
  border: 1px solid #e9ecef;
  border-radius: 6px;
  transition: all 0.2s ease;
}

.api-key-item:hover {
  border-color: #dee2e6;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

.api-key-item.inactive {
  opacity: 0.6;
  background: #f8f9fa;
}

.api-key-content {
  flex: 1;
}

.api-key-name {
  font-weight: 500;
  color: #333;
  margin-bottom: 0.5rem;
}

.api-key-details {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
  font-size: 0.9rem;
  color: #666;
}

.detail-item {
  display: flex;
  align-items: center;
  gap: 0.25rem;
}

.status-active {
  color: #28a745;
  font-weight: 500;
}

.status-inactive {
  color: #dc3545;
  font-weight: 500;
}

.api-key-actions {
  display: flex;
  gap: 0.5rem;
}

.deactivate-button {
  padding: 0.5rem;
  background: #ffc107;
  color: #212529;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  transition: background-color 0.2s ease;
}

.deactivate-button:hover {
  background: #e0a800;
}

.delete-button {
  padding: 0.5rem;
  background: #dc3545;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  transition: background-color 0.2s ease;
}

.delete-button:hover {
  background: #c82333;
}

.deactivate-button :deep(svg),
.delete-button :deep(svg) {
  width: 16px;
  height: 16px;
}
</style>
