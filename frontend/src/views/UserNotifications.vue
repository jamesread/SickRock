<script setup lang="ts">
import { ref, onMounted, inject } from 'vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import * as Hugeicons from '@hugeicons/core-free-icons'
import { Delete01Icon } from '@hugeicons/core-free-icons'
import type { createApiClient } from '../stores/api'
import Section from 'picocrank/vue/components/Section.vue'

// Use global API client
const client = inject<ReturnType<typeof createApiClient>>('apiClient')

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
  notificationsLoading.value = true
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
  } finally {
    notificationsLoading.value = false
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
    loadNotificationEvents(),
    loadNotificationChannels(),
    loadNotificationSubscriptions()
  ])
})
</script>

<template>
  <div v-if="notificationError" class="error">{{ notificationError }}</div>

  <!-- Notification Channels -->
  <Section title="Notification Channels">
    <p class="section-description">Configure how you want to receive notifications (email, Telegram, or webhooks).</p>

    <div class="notification-channels-content">

      <!-- Create New Channel -->
      <div class="create-channel">
        <h4>Add New Channel</h4>
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
        <h4>Your Channels</h4>
        <div v-if="notificationsLoading" class="loading">Loading channels...</div>
        <div v-else-if="notificationChannels.length === 0" class="no-items">
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
  </Section>

  <!-- Notification Subscriptions -->
  <Section title="Event Subscriptions">
    <p class="section-description">Choose which events you want to be notified about and via which channel.</p>

    <div class="notification-subscriptions-content">

      <!-- Create New Subscription -->
      <div class="create-subscription">
        <h4>Subscribe to Event</h4>
        <div v-if="notificationChannels.filter(c => c.isActive).length === 0" class="no-channels-placeholder">
          <p>No active notification channels configured. Please add a channel above before creating event subscriptions.</p>
        </div>
        <div v-else class="create-form">
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
        <h4>Your Subscriptions</h4>
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
  </Section>
</template>

<style scoped>
.section-description {
  margin: 0 0 1.5rem 0;
  color: #6c757d;
  font-size: 0.95rem;
}

.error {
  background: #f8d7da;
  color: #721c24;
  padding: 0.75rem;
  border: 1px solid #f5c6cb;
  border-radius: 4px;
  margin-bottom: 1rem;
}

.notification-channels-content,
.notification-subscriptions-content {
  margin-top: 1rem;
}

.create-channel,
.create-subscription {
  margin-bottom: 2rem;
  padding: 1.5rem;
  background: #f8f9fa;
  border: 1px solid #e9ecef;
  border-radius: 6px;
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

.create-form select,
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

.current-channels,
.current-subscriptions {
  margin-bottom: 2rem;
}

.loading {
  color: #666;
  font-style: italic;
  padding: 1rem;
}

.no-items {
  color: #666;
  font-style: italic;
  padding: 2rem;
  text-align: center;
  background: white;
  border: 1px solid #e9ecef;
  border-radius: 6px;
}

.no-channels-placeholder {
  padding: 1.5rem;
  background: #fff3cd;
  border: 1px solid #ffeaa7;
  border-radius: 6px;
  color: #856404;
}

.no-channels-placeholder p {
  margin: 0;
  font-size: 0.95rem;
  line-height: 1.5;
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

.delete-button {
  padding: 0.5rem;
  background: #dc3545;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  transition: background-color 0.2s ease;
  flex-shrink: 0;
}

.delete-button:hover {
  background: #c82333;
}

.delete-button :deep(svg) {
  width: 16px;
  height: 16px;
}
</style>
