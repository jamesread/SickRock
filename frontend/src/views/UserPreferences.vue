<script setup lang="ts">
import { ref, onMounted, computed, inject } from 'vue'
import { useAuthStore } from '../stores/auth'
import { HugeiconsIcon } from '@hugeicons/vue'
import * as Hugeicons from '@hugeicons/core-free-icons'
import { DatabaseIcon } from '@hugeicons/core-free-icons'
import type { createApiClient } from '../stores/api'
import Section from 'picocrank/vue/components/Section.vue'
import { formatUnixTimestamp } from '../utils/dateFormatting'

// Use global API client
const client = inject<ReturnType<typeof createApiClient>>('apiClient')
const authStore = useAuthStore()
const user = computed(() => authStore.user)

const loading = ref(true)
const error = ref<string | null>(null)

// User preferences state
const preferences = ref({
  theme: 'light',
  language: 'en',
  notifications: true,
  emailUpdates: false
})

// Bookmarks state
const bookmarks = ref<Array<{
  id: number;
  userId: number;
  navigationItemId: number;
  navigationItem?: {
    id: number;
    ordinal: number;
    tableConfiguration: number;
    tableName: string;
    tableTitle: string;
    icon: string;
    tableView: string;
    dashboardId: number;
    dashboardName: string;
  };
}>>([])

const bookmarksLoading = ref(false)

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

async function loadPreferences() {
  loading.value = true
  error.value = null
  try {
    // TODO: Implement actual preferences loading from API
    // For now, using default values
    console.log('Loading user preferences...')
  } catch (e: any) {
    error.value = String(e?.message || e)
  } finally {
    loading.value = false
  }
}

async function loadBookmarks() {
  bookmarksLoading.value = true
  try {
    const response = await client.getUserBookmarks({})
    bookmarks.value = (response.bookmarks || []).map(bookmark => ({
      id: bookmark.id,
      userId: bookmark.userId,
      navigationItemId: bookmark.navigationItemId,
      navigationItem: bookmark.navigationItem ? {
        id: bookmark.navigationItem.id,
        ordinal: bookmark.navigationItem.ordinal,
        tableConfiguration: bookmark.navigationItem.tableConfiguration,
        tableName: bookmark.navigationItem.tableName,
        tableTitle: bookmark.navigationItem.tableTitle,
        icon: bookmark.navigationItem.icon,
        tableView: bookmark.navigationItem.tableView,
        dashboardId: bookmark.navigationItem.dashboardId,
        dashboardName: bookmark.navigationItem.dashboardName
      } : undefined
    }))
  } catch (e: any) {
    console.error('Failed to load bookmarks:', e)
  } finally {
    bookmarksLoading.value = false
  }
}

async function loadAPIKeys() {
  apiKeysLoading.value = true
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

async function savePreferences() {
  try {
    // TODO: Implement actual preferences saving to API
    console.log('Saving preferences:', preferences.value)
  } catch (e: any) {
    error.value = String(e?.message || e)
  }
}

async function removeBookmark(bookmarkId: number) {
  try {
    await client.deleteUserBookmark({ bookmarkId })
    await loadBookmarks() // Reload bookmarks
  } catch (e: any) {
    console.error('Failed to remove bookmark:', e)
    error.value = String(e?.message || e)
  }
}

onMounted(async () => {
  await Promise.all([
    loadPreferences(),
    loadBookmarks(),
    loadAPIKeys()
  ])
})
</script>

<template>
  <Section title="User Preferences">
    <div v-if="loading">Loading preferences…</div>
    <div v-else>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-else>
        <div class="preferences-form">
          <div class="form-group">
            <label for="theme">Theme</label>
            <select id="theme" v-model="preferences.theme">
              <option value="light">Light</option>
              <option value="dark">Dark</option>
              <option value="auto">Auto</option>
            </select>
          </div>

          <div class="form-group">
            <label for="language">Language</label>
            <select id="language" v-model="preferences.language">
              <option value="en">English</option>
              <option value="es">Spanish</option>
              <option value="fr">French</option>
            </select>
          </div>

          <div class="form-group">
            <label class="checkbox-label">
              <input type="checkbox" v-model="preferences.notifications" />
              Enable notifications
            </label>
          </div>

          <div class="form-group">
            <label class="checkbox-label">
              <input type="checkbox" v-model="preferences.emailUpdates" />
              Email updates
            </label>
          </div>

          <div class="form-actions">
            <button @click="savePreferences" class="save-button">
              Save Preferences
            </button>
          </div>
        </div>

        <div class="user-info">
          <h3>Account Information</h3>
          <p><strong>Username:</strong> {{ user?.username }}</p>
          <p><strong>Status:</strong> Active</p>
        </div>

        <!-- Bookmarks Section -->
        <div class="bookmarks-section">
          <h3>Bookmarks</h3>

          <!-- Current Bookmarks -->
          <div class="current-bookmarks">
            <h4>Current Bookmarks</h4>
            <div v-if="bookmarksLoading" class="loading">Loading bookmarks...</div>
            <div v-else-if="bookmarks.length === 0" class="no-bookmarks">
              <p>No bookmarks yet. Use the star button in the header to add bookmarks.</p>
            </div>
            <div v-else class="bookmarks-list">
              <div
                v-for="bookmark in bookmarks"
                :key="bookmark.id"
                class="bookmark-item"
              >
                <div class="bookmark-content">
                  <HugeiconsIcon
                    :icon="(bookmark.navigationItem?.icon && (Hugeicons as any)[bookmark.navigationItem.icon])
                        ? (Hugeicons as any)[bookmark.navigationItem.icon]
                        : DatabaseIcon"
                    class="bookmark-icon"
                  />
                  <div class="bookmark-info">
                    <span class="bookmark-title">{{ bookmark.navigationItem?.tableName || 'Unknown' }}</span>
                  </div>
                </div>
                <button
                  @click="removeBookmark(bookmark.id)"
                  class="remove-bookmark-btn"
                  title="Remove bookmark"
                >
                  <HugeiconsIcon :icon="Hugeicons.Delete01Icon" />
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- API Keys Section -->
        <div class="api-keys-section">
          <h3>API Keys</h3>

          <div v-if="apiKeyError" class="error">{{ apiKeyError }}</div>

          <!-- Create New API Key -->
          <div class="create-api-key">
            <h4>Create New API Key</h4>
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
            <h4>New API Key Created</h4>
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
            <h4>Current API Keys</h4>
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
        </div>
      </div>
    </div>
  </Section>
</template>

<style scoped>
.preferences-form {
  margin-bottom: 2rem;
}

.form-group {
  margin-bottom: 1rem;
}

.form-group label {
  display: block;
  margin-bottom: 0.5rem;
  font-weight: 500;
}

.form-group select {
  width: 100%;
  max-width: 300px;
  padding: 0.5rem;
  border: 1px solid #ccc;
  border-radius: 4px;
  background: white;
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  cursor: pointer;
}

.checkbox-label input[type="checkbox"] {
  margin: 0;
}

.form-actions {
  margin-top: 1.5rem;
}

.save-button {
  padding: 0.75rem 1.5rem;
  background: #007bff;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-weight: 500;
  transition: background-color 0.2s ease;
}

.save-button:hover {
  background: #0056b3;
}

.user-info {
  padding: 1rem;
  background: #f8f9fa;
  border: 1px solid #e9ecef;
  border-radius: 6px;
}

.user-info h3 {
  margin: 0 0 1rem 0;
  color: #333;
}

.user-info p {
  margin: 0.5rem 0;
  color: #666;
}

.error {
  background: #f8d7da;
  color: #721c24;
  padding: 0.75rem;
  border: 1px solid #f5c6cb;
  border-radius: 4px;
  margin-bottom: 1rem;
}

/* Bookmarks Section */
.bookmarks-section {
  margin-top: 2rem;
  padding: 1rem;
  background: #f8f9fa;
  border: 1px solid #e9ecef;
  border-radius: 6px;
}

.bookmarks-section h3 {
  margin: 0 0 1.5rem 0;
  color: #333;
}

.bookmarks-section h4 {
  margin: 0 0 1rem 0;
  color: #555;
  font-size: 1rem;
}

.current-bookmarks {
  margin-bottom: 2rem;
}

.loading {
  color: #666;
  font-style: italic;
}

.no-bookmarks {
  color: #666;
  font-style: italic;
  padding: 1rem;
  text-align: center;
}

/* Bookmarks List */
.bookmarks-list {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.bookmark-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.75rem;
  background: white;
  border: 1px solid #e9ecef;
  border-radius: 6px;
  transition: all 0.2s ease;
}

.bookmark-item:hover {
  border-color: #dee2e6;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

.bookmark-content {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  flex: 1;
}

.bookmark-icon {
  width: 20px;
  height: 20px;
  color: #666;
}

.bookmark-info {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.bookmark-title {
  font-weight: 500;
  color: #333;
}

.remove-bookmark-btn {
  padding: 0.5rem;
  background: #dc3545;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  transition: background-color 0.2s ease;
}

.remove-bookmark-btn:hover {
  background: #c82333;
}

/* API Keys Section */
.api-keys-section {
  margin-top: 2rem;
  padding: 1rem;
  background: #f8f9fa;
  border: 1px solid #e9ecef;
  border-radius: 6px;
}

.api-keys-section h3 {
  margin: 0 0 1.5rem 0;
  color: #333;
}

.api-keys-section h4 {
  margin: 0 0 1rem 0;
  color: #555;
  font-size: 1rem;
}

.create-api-key {
  margin-bottom: 2rem;
  padding: 1rem;
  background: white;
  border: 1px solid #e9ecef;
  border-radius: 6px;
}

.create-form {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.create-form .form-group {
  margin-bottom: 0;
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
  padding: 1rem;
  background: #d4edda;
  border: 1px solid #c3e6cb;
  border-radius: 6px;
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

.no-api-keys {
  color: #666;
  font-style: italic;
  padding: 1rem;
  text-align: center;
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
</style>
