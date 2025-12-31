<script setup lang="ts">
import { ref, onMounted, computed, inject } from 'vue'
import { useAuthStore } from '../stores/auth'
import { HugeiconsIcon } from '@hugeicons/vue'
import * as Hugeicons from '@hugeicons/core-free-icons'
import { DatabaseIcon, Download01Icon } from '@hugeicons/core-free-icons'
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

// Notification state
const notificationEvents = ref<Array<{
  id: number;
  eventCode: string;
  eventName: string;
  description: string;
}>>([])
const notificationChannels = ref<Array<{
  id: number;
  userId: number;
  channelType: string;
  channelValue: string;
  channelName: string;
  isActive: boolean;
  srCreated: number;
  srUpdated: number;
}>>([])
const notificationSubscriptions = ref<Array<{
  id: number;
  userId: number;
  eventId: number;
  channelId: number;
  event?: {
    id: number;
    eventCode: string;
    eventName: string;
    description: string;
  };
  channel?: {
    id: number;
    userId: number;
    channelType: string;
    channelValue: string;
    channelName: string;
    isActive: boolean;
  };
  srCreated: number;
}>>([])

const notificationsLoading = ref(false)
const notificationError = ref<string | null>(null)

// New channel form
const newChannelType = ref('email')
const newChannelValue = ref('')
const newChannelName = ref('')
const creatingChannel = ref(false)

// New subscription form
const newSubscriptionEventCode = ref('')
const newSubscriptionChannelId = ref(0)
const creatingSubscription = ref(false)

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

function formatSWState(state: string | null): string {
  if (!state) return 'Unknown'
  const stateMap: Record<string, string> = {
    'installing': 'Installing',
    'installed': 'Installed',
    'activating': 'Activating',
    'activated': 'Active',
    'redundant': 'Redundant'
  }
  return stateMap[state] || state.charAt(0).toUpperCase() + state.slice(1)
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

async function loadNotificationEvents() {
  try {
    const response = await client.getNotificationEvents({})
    notificationEvents.value = (response.events || []).map(event => ({
      id: event.id,
      eventCode: event.eventCode,
      eventName: event.eventName,
      description: event.description
    }))
  } catch (e: any) {
    console.error('Failed to load notification events:', e)
    notificationError.value = String(e?.message || e)
  }
}

async function loadNotificationChannels() {
  try {
    const response = await client.getUserNotificationChannels({})
    notificationChannels.value = (response.channels || []).map(channel => ({
      id: channel.id,
      userId: channel.userId,
      channelType: channel.channelType,
      channelValue: channel.channelValue,
      channelName: channel.channelName || '',
      isActive: channel.isActive,
      srCreated: Number(channel.srCreated),
      srUpdated: Number(channel.srUpdated)
    }))
  } catch (e: any) {
    console.error('Failed to load notification channels:', e)
    notificationError.value = String(e?.message || e)
  }
}

async function loadNotificationSubscriptions() {
  try {
    const response = await client.getUserNotificationSubscriptions({})
    notificationSubscriptions.value = (response.subscriptions || []).map(sub => ({
      id: sub.id,
      userId: sub.userId,
      eventId: sub.eventId,
      channelId: sub.channelId,
      event: sub.event ? {
        id: sub.event.id,
        eventCode: sub.event.eventCode,
        eventName: sub.event.eventName,
        description: sub.event.description
      } : undefined,
      channel: sub.channel ? {
        id: sub.channel.id,
        userId: sub.channel.userId,
        channelType: sub.channel.channelType,
        channelValue: sub.channel.channelValue,
        channelName: sub.channel.channelName || '',
        isActive: sub.channel.isActive
      } : undefined,
      srCreated: Number(sub.srCreated)
    }))
  } catch (e: any) {
    console.error('Failed to load notification subscriptions:', e)
    notificationError.value = String(e?.message || e)
  }
}

async function createNotificationChannel() {
  if (!newChannelValue.value.trim()) {
    notificationError.value = 'Channel value is required'
    return
  }

  creatingChannel.value = true
  notificationError.value = null
  try {
    const response = await client.createUserNotificationChannel({
      channelType: newChannelType.value,
      channelValue: newChannelValue.value.trim(),
      channelName: newChannelName.value.trim() || undefined
    })

    if (response.success) {
      newChannelValue.value = ''
      newChannelName.value = ''
      await loadNotificationChannels()
    } else {
      notificationError.value = response.message || 'Failed to create channel'
    }
  } catch (e: any) {
    console.error('Failed to create notification channel:', e)
    notificationError.value = String(e?.message || e)
  } finally {
    creatingChannel.value = false
  }
}

async function deleteNotificationChannel(channelId: number) {
  if (!confirm('Are you sure you want to delete this notification channel? This will also remove all subscriptions using this channel.')) {
    return
  }

  try {
    const response = await client.deleteUserNotificationChannel({ channelId })
    if (response.success) {
      await Promise.all([
        loadNotificationChannels(),
        loadNotificationSubscriptions()
      ])
    } else {
      notificationError.value = response.message || 'Failed to delete channel'
    }
  } catch (e: any) {
    console.error('Failed to delete notification channel:', e)
    notificationError.value = String(e?.message || e)
  }
}

async function createNotificationSubscription() {
  if (!newSubscriptionEventCode.value || !newSubscriptionChannelId.value) {
    notificationError.value = 'Event and channel are required'
    return
  }

  creatingSubscription.value = true
  notificationError.value = null
  try {
    const response = await client.createUserNotificationSubscription({
      eventCode: newSubscriptionEventCode.value,
      channelId: newSubscriptionChannelId.value
    })

    if (response.success) {
      newSubscriptionEventCode.value = ''
      newSubscriptionChannelId.value = 0
      await loadNotificationSubscriptions()
    } else {
      notificationError.value = response.message || 'Failed to create subscription'
    }
  } catch (e: any) {
    console.error('Failed to create notification subscription:', e)
    notificationError.value = String(e?.message || e)
  } finally {
    creatingSubscription.value = false
  }
}

async function deleteNotificationSubscription(subscriptionId: number) {
  if (!confirm('Are you sure you want to remove this notification subscription?')) {
    return
  }

  try {
    const response = await client.deleteUserNotificationSubscription({ subscriptionId })
    if (response.success) {
      await loadNotificationSubscriptions()
    } else {
      notificationError.value = response.message || 'Failed to delete subscription'
    }
  } catch (e: any) {
    console.error('Failed to delete notification subscription:', e)
    notificationError.value = String(e?.message || e)
  }
}

onMounted(async () => {
  await Promise.all([
    loadPreferences(),
    loadBookmarks(),
    loadAPIKeys(),
    loadNotificationEvents(),
    loadNotificationChannels(),
    loadNotificationSubscriptions()
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

        <!-- PWA & Service Worker Section -->
        <div class="pwa-section">
          <h3>PWA & Service Worker</h3>
          <p>Manage Progressive Web App installation and service worker status.</p>
          <div class="pwa-actions">
            <router-link to="/admin/pwa-installation" class="pwa-link-button">
              <HugeiconsIcon :icon="Hugeicons.Download01Icon" />
              Open PWA Installation & Service Worker
            </router-link>
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

        <!-- Notifications Section -->
        <div class="notifications-section">
          <h3>Notifications</h3>
          <p>Configure notification channels and subscribe to events.</p>

          <div v-if="notificationError" class="error">{{ notificationError }}</div>

          <!-- Notification Channels -->
          <div class="notification-channels">
            <h4>Notification Channels</h4>
            <p class="section-description">Configure how you want to receive notifications (email, Telegram, or webhooks).</p>

            <!-- Create New Channel -->
            <div class="create-channel">
              <h5>Add New Channel</h5>
              <div class="create-form">
                <div class="form-group">
                  <label for="channel-type">Channel Type</label>
                  <select id="channel-type" v-model="newChannelType" :disabled="creatingChannel">
                    <option value="email">Email</option>
                    <option value="telegram">Telegram</option>
                    <option value="webhook">Webhook</option>
                  </select>
                </div>
                <div class="form-group">
                  <label for="channel-value">
                    {{ newChannelType === 'email' ? 'Email Address' : newChannelType === 'telegram' ? 'Telegram Chat ID' : 'Webhook URL' }}
                  </label>
                  <input
                    id="channel-value"
                    v-model="newChannelValue"
                    type="text"
                    :placeholder="newChannelType === 'email' ? 'user@example.com' : newChannelType === 'telegram' ? '123456789' : 'https://example.com/webhook'"
                    :disabled="creatingChannel"
                  />
                </div>
                <div v-if="newChannelType === 'webhook'" class="form-group">
                  <label for="channel-name">Channel Name (optional)</label>
                  <input
                    id="channel-name"
                    v-model="newChannelName"
                    type="text"
                    placeholder="e.g., Webhook A"
                    :disabled="creatingChannel"
                  />
                </div>
                <button
                  @click="createNotificationChannel"
                  :disabled="creatingChannel || !newChannelValue.trim()"
                  class="create-button"
                >
                  {{ creatingChannel ? 'Creating...' : 'Add Channel' }}
                </button>
              </div>
            </div>

            <!-- Current Channels -->
            <div class="current-channels">
              <h5>Your Channels</h5>
              <div v-if="notificationChannels.length === 0" class="no-items">
                <p>No channels configured. Add one above to get started.</p>
              </div>
              <div v-else class="channels-list">
                <div
                  v-for="channel in notificationChannels"
                  :key="channel.id"
                  class="channel-item"
                  :class="{ inactive: !channel.isActive }"
                >
                  <div class="channel-content">
                    <div class="channel-info">
                      <div class="channel-name">
                        <strong>{{ channel.channelType.toUpperCase() }}</strong>
                        <span v-if="channel.channelName"> - {{ channel.channelName }}</span>
                      </div>
                      <div class="channel-details">
                        <span class="detail-item">{{ channel.channelValue }}</span>
                        <span class="detail-item">
                          <span :class="channel.isActive ? 'status-active' : 'status-inactive'">
                            {{ channel.isActive ? 'Active' : 'Inactive' }}
                          </span>
                        </span>
                      </div>
                    </div>
                  </div>
                  <button
                    @click="deleteNotificationChannel(channel.id)"
                    class="delete-button"
                    title="Delete channel"
                  >
                    <HugeiconsIcon :icon="Hugeicons.Delete01Icon" />
                  </button>
                </div>
              </div>
            </div>
          </div>

          <!-- Notification Subscriptions -->
          <div class="notification-subscriptions">
            <h4>Event Subscriptions</h4>
            <p class="section-description">Choose which events you want to be notified about and via which channel.</p>

            <!-- Create New Subscription -->
            <div class="create-subscription">
              <h5>Subscribe to Event</h5>
              <div class="create-form">
                <div class="form-group">
                  <label for="subscription-event">Event</label>
                  <select id="subscription-event" v-model="newSubscriptionEventCode" :disabled="creatingSubscription">
                    <option value="">Select an event...</option>
                    <option
                      v-for="event in notificationEvents"
                      :key="event.id"
                      :value="event.eventCode"
                    >
                      {{ event.eventName }} - {{ event.description }}
                    </option>
                  </select>
                </div>
                <div class="form-group">
                  <label for="subscription-channel">Channel</label>
                  <select id="subscription-channel" v-model="newSubscriptionChannelId" :disabled="creatingSubscription">
                    <option :value="0">Select a channel...</option>
                    <option
                      v-for="channel in notificationChannels.filter(c => c.isActive)"
                      :key="channel.id"
                      :value="channel.id"
                    >
                      {{ channel.channelType.toUpperCase() }}{{ channel.channelName ? ' - ' + channel.channelName : '' }}: {{ channel.channelValue }}
                    </option>
                  </select>
                </div>
                <button
                  @click="createNotificationSubscription"
                  :disabled="creatingSubscription || !newSubscriptionEventCode || !newSubscriptionChannelId"
                  class="create-button"
                >
                  {{ creatingSubscription ? 'Creating...' : 'Subscribe' }}
                </button>
              </div>
            </div>

            <!-- Current Subscriptions -->
            <div class="current-subscriptions">
              <h5>Your Subscriptions</h5>
              <div v-if="notificationSubscriptions.length === 0" class="no-items">
                <p>No subscriptions yet. Subscribe to events above to receive notifications.</p>
              </div>
              <div v-else class="subscriptions-list">
                <div
                  v-for="subscription in notificationSubscriptions"
                  :key="subscription.id"
                  class="subscription-item"
                >
                  <div class="subscription-content">
                    <div class="subscription-info">
                      <div class="subscription-event">
                        <strong>{{ subscription.event?.eventName || 'Unknown Event' }}</strong>
                      </div>
                      <div class="subscription-details">
                        <span class="detail-item">
                          <strong>Channel:</strong> {{ subscription.channel?.channelType.toUpperCase() }}{{ subscription.channel?.channelName ? ' - ' + subscription.channel.channelName : '' }}
                        </span>
                        <span class="detail-item">
                          <strong>Value:</strong> {{ subscription.channel?.channelValue }}
                        </span>
                      </div>
                    </div>
                  </div>
                  <button
                    @click="deleteNotificationSubscription(subscription.id)"
                    class="delete-button"
                    title="Remove subscription"
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

/* PWA Section */
.pwa-section {
  margin-top: 2rem;
  padding: 1rem;
  background: #f8f9fa;
  border: 1px solid #e9ecef;
  border-radius: 6px;
}

.pwa-section h3 {
  margin: 0 0 0.5rem 0;
  color: #333;
}

.pwa-section p {
  margin: 0 0 1rem 0;
  color: #666;
  font-size: 0.9rem;
}

.pwa-actions {
  display: flex;
  gap: 1rem;
}

.pwa-link-button {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.75rem 1.5rem;
  background: #007bff;
  color: white;
  text-decoration: none;
  border-radius: 4px;
  font-weight: 500;
  transition: background-color 0.2s ease;
}

.pwa-link-button:hover {
  background: #0056b3;
  color: white;
  text-decoration: none;
}

.pwa-link-button svg {
  width: 18px;
  height: 18px;
}

/* Notifications Section */
.notifications-section {
  margin-top: 2rem;
  padding: 1rem;
  background: #f8f9fa;
  border: 1px solid #e9ecef;
  border-radius: 6px;
}

.notifications-section h3 {
  margin: 0 0 0.5rem 0;
  color: #333;
}

.notifications-section > p {
  margin: 0 0 1.5rem 0;
  color: #666;
  font-size: 0.9rem;
}

.notification-channels,
.notification-subscriptions {
  margin-bottom: 2rem;
  padding: 1rem;
  background: white;
  border: 1px solid #e9ecef;
  border-radius: 6px;
}

.notification-channels h4,
.notification-subscriptions h4 {
  margin: 0 0 0.5rem 0;
  color: #333;
}

.section-description {
  margin: 0 0 1rem 0;
  color: #666;
  font-size: 0.9rem;
}

.notification-channels h5,
.notification-subscriptions h5 {
  margin: 0 0 1rem 0;
  color: #555;
  font-size: 0.95rem;
}

.create-channel,
.create-subscription {
  margin-bottom: 2rem;
  padding: 1rem;
  background: #f8f9fa;
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

.create-form select,
.create-form input {
  width: 100%;
  max-width: 400px;
  padding: 0.5rem;
  border: 1px solid #ccc;
  border-radius: 4px;
  background: white;
}

.current-channels,
.current-subscriptions {
  margin-bottom: 2rem;
}

.no-items {
  color: #666;
  font-style: italic;
  padding: 1rem;
  text-align: center;
}

.channels-list,
.subscriptions-list {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.channel-item,
.subscription-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1rem;
  background: white;
  border: 1px solid #e9ecef;
  border-radius: 6px;
  transition: all 0.2s ease;
}

.channel-item:hover,
.subscription-item:hover {
  border-color: #dee2e6;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

.channel-item.inactive {
  opacity: 0.6;
  background: #f8f9fa;
}

.channel-content,
.subscription-content {
  flex: 1;
}

.channel-info,
.subscription-info {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.channel-name,
.subscription-event {
  font-weight: 500;
  color: #333;
}

.channel-details,
.subscription-details {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
  font-size: 0.9rem;
  color: #666;
}
</style>
