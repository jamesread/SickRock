<script setup lang="ts">
import { ref, onMounted, inject, nextTick, computed } from 'vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import {
  Add01Icon,
  CheckListIcon,
  Delete01Icon,
  NotificationIcon,
  RefreshIcon,
} from '@hugeicons/core-free-icons'
import type { createApiClient } from '../stores/api'
import Section from 'picocrank/vue/components/Section.vue'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'
import FormLayout from 'picocrank/vue/components/FormLayout.vue'
import FormField from 'picocrank/vue/components/FormField.vue'
import Table from 'picocrank/vue/components/Table.vue'

const iconStrokeWidth = 2.5
const client = inject<ReturnType<typeof createApiClient>>('apiClient')!

type NotificationEvent = {
  id: number
  eventCode: string
  eventName: string
  description: string
}

type NotificationChannel = {
  id: number
  userId: number
  channelType: string
  channelValue: string
  channelName: string
  isActive: boolean
  srCreated: number
  srUpdated: number
}

type NotificationSubscription = {
  id: number
  userId: number
  eventId: number
  channelId: number
  event?: NotificationEvent
  channel?: Omit<NotificationChannel, 'srCreated' | 'srUpdated'>
  srCreated: number
}

const notificationEvents = ref<NotificationEvent[]>([])
const notificationChannels = ref<NotificationChannel[]>([])
const notificationSubscriptions = ref<NotificationSubscription[]>([])

const channelsLoading = ref(false)
const subscriptionsLoading = ref(false)
const channelsError = ref('')
const subscriptionsError = ref('')
const channelCreateError = ref('')
const subscriptionCreateError = ref('')
const creatingChannel = ref(false)
const creatingSubscription = ref(false)

const newChannelType = ref('email')
const newChannelValue = ref('')
const newChannelName = ref('')
const newSubscriptionEventCode = ref('')
const newSubscriptionChannelId = ref(0)

const createChannelDialog = ref<HTMLDialogElement | null>(null)
const createSubscriptionDialog = ref<HTMLDialogElement | null>(null)
const channelValueInput = ref<HTMLInputElement | null>(null)

const activeChannels = computed(() => notificationChannels.value.filter((c) => c.isActive))

const channelValueLabel = computed(() => {
  switch (newChannelType.value) {
    case 'telegram':
      return 'Telegram chat ID'
    case 'webhook':
      return 'Webhook URL'
    default:
      return 'Email address'
  }
})

const channelValuePlaceholder = computed(() => {
  switch (newChannelType.value) {
    case 'telegram':
      return '123456789'
    case 'webhook':
      return 'https://example.com/webhook'
    default:
      return 'user@example.com'
  }
})

const subscriptionTableRows = computed(() =>
  notificationSubscriptions.value.map((sub) => ({
    id: sub.id,
    eventName: sub.event?.eventName ?? 'Unknown event',
    eventDescription: sub.event?.description ?? '',
    channelType: sub.channel?.channelType?.toUpperCase() ?? '—',
    channelName: sub.channel?.channelName ?? '',
    channelValue: sub.channel?.channelValue ?? '',
  })),
)

const channelHeaders = [
  { key: 'channelType', label: 'Type', sortable: true, width: '7rem' },
  { key: 'channelName', label: 'Name', sortable: true },
  { key: 'channelValue', label: 'Destination', sortable: true },
  { key: 'isActive', label: 'Status', sortable: true, width: '7rem' },
  { key: 'actions', label: 'Actions', sortable: false, width: '7rem' },
]

const subscriptionHeaders = [
  { key: 'eventName', label: 'Event', sortable: true },
  { key: 'channelType', label: 'Channel', sortable: true, width: '8rem' },
  { key: 'channelValue', label: 'Destination', sortable: true },
  { key: 'actions', label: 'Actions', sortable: false, width: '7rem' },
]

async function loadNotificationEvents() {
  try {
    const response = await client.getNotificationEvents({})
    notificationEvents.value = (response.events || []).map((event) => ({
      id: event.id,
      eventCode: event.eventCode,
      eventName: event.eventName,
      description: event.description,
    }))
  } catch (e: unknown) {
    subscriptionsError.value = e instanceof Error ? e.message : 'Failed to load notification events.'
  }
}

async function loadNotificationChannels() {
  channelsLoading.value = true
  channelsError.value = ''
  try {
    const response = await client.getUserNotificationChannels({})
    notificationChannels.value = (response.channels || []).map((channel) => ({
      id: channel.id,
      userId: channel.userId,
      channelType: channel.channelType,
      channelValue: channel.channelValue,
      channelName: channel.channelName || '',
      isActive: channel.isActive,
      srCreated: Number(channel.srCreated),
      srUpdated: Number(channel.srUpdated),
    }))
  } catch (e: unknown) {
    channelsError.value = e instanceof Error ? e.message : 'Failed to load notification channels.'
  } finally {
    channelsLoading.value = false
  }
}

async function loadNotificationSubscriptions() {
  subscriptionsLoading.value = true
  subscriptionsError.value = ''
  try {
    const response = await client.getUserNotificationSubscriptions({})
    notificationSubscriptions.value = (response.subscriptions || []).map((sub) => ({
      id: sub.id,
      userId: sub.userId,
      eventId: sub.eventId,
      channelId: sub.channelId,
      event: sub.event
        ? {
            id: sub.event.id,
            eventCode: sub.event.eventCode,
            eventName: sub.event.eventName,
            description: sub.event.description,
          }
        : undefined,
      channel: sub.channel
        ? {
            id: sub.channel.id,
            userId: sub.channel.userId,
            channelType: sub.channel.channelType,
            channelValue: sub.channel.channelValue,
            channelName: sub.channel.channelName || '',
            isActive: sub.channel.isActive,
          }
        : undefined,
      srCreated: Number(sub.srCreated),
    }))
  } catch (e: unknown) {
    subscriptionsError.value = e instanceof Error ? e.message : 'Failed to load subscriptions.'
  } finally {
    subscriptionsLoading.value = false
  }
}

function resetChannelForm() {
  newChannelType.value = 'email'
  newChannelValue.value = ''
  newChannelName.value = ''
  channelCreateError.value = ''
}

function resetSubscriptionForm() {
  newSubscriptionEventCode.value = ''
  newSubscriptionChannelId.value = 0
  subscriptionCreateError.value = ''
}

function openCreateChannelDialog() {
  resetChannelForm()
  createChannelDialog.value?.showModal()
  nextTick(() => channelValueInput.value?.focus())
}

function closeCreateChannelDialog() {
  createChannelDialog.value?.close()
}

function openCreateSubscriptionDialog() {
  if (!activeChannels.value.length) {
    subscriptionsError.value = 'Add an active notification channel before subscribing to events.'
    return
  }
  resetSubscriptionForm()
  createSubscriptionDialog.value?.showModal()
}

function closeCreateSubscriptionDialog() {
  createSubscriptionDialog.value?.close()
}

async function createNotificationChannel() {
  if (!newChannelValue.value.trim()) {
    channelCreateError.value = 'Destination is required.'
    return
  }

  creatingChannel.value = true
  channelCreateError.value = ''
  try {
    const response = await client.createUserNotificationChannel({
      channelType: newChannelType.value,
      channelValue: newChannelValue.value.trim(),
      channelName: newChannelName.value.trim() || undefined,
    })

    if (response.success) {
      closeCreateChannelDialog()
      await loadNotificationChannels()
    } else {
      channelCreateError.value = response.message || 'Failed to create channel.'
    }
  } catch (e: unknown) {
    channelCreateError.value = e instanceof Error ? e.message : 'Failed to create channel.'
  } finally {
    creatingChannel.value = false
  }
}

async function deleteNotificationChannel(channelId: number) {
  if (
    !confirm(
      'Delete this notification channel? Subscriptions that use it will be removed.',
    )
  ) {
    return
  }

  channelsError.value = ''
  try {
    const response = await client.deleteUserNotificationChannel({ channelId })
    if (response.success) {
      await Promise.all([loadNotificationChannels(), loadNotificationSubscriptions()])
    } else {
      channelsError.value = response.message || 'Failed to delete channel.'
    }
  } catch (e: unknown) {
    channelsError.value = e instanceof Error ? e.message : 'Failed to delete channel.'
  }
}

async function createNotificationSubscription() {
  if (!newSubscriptionEventCode.value || !newSubscriptionChannelId.value) {
    subscriptionCreateError.value = 'Event and channel are required.'
    return
  }

  creatingSubscription.value = true
  subscriptionCreateError.value = ''
  try {
    const response = await client.createUserNotificationSubscription({
      eventCode: newSubscriptionEventCode.value,
      channelId: newSubscriptionChannelId.value,
    })

    if (response.success) {
      closeCreateSubscriptionDialog()
      await loadNotificationSubscriptions()
    } else {
      subscriptionCreateError.value = response.message || 'Failed to create subscription.'
    }
  } catch (e: unknown) {
    subscriptionCreateError.value = e instanceof Error ? e.message : 'Failed to create subscription.'
  } finally {
    creatingSubscription.value = false
  }
}

async function deleteNotificationSubscription(subscriptionId: number) {
  if (!confirm('Remove this event subscription?')) {
    return
  }

  subscriptionsError.value = ''
  try {
    const response = await client.deleteUserNotificationSubscription({ subscriptionId })
    if (response.success) {
      await loadNotificationSubscriptions()
    } else {
      subscriptionsError.value = response.message || 'Failed to delete subscription.'
    }
  } catch (e: unknown) {
    subscriptionsError.value = e instanceof Error ? e.message : 'Failed to delete subscription.'
  }
}

onMounted(async () => {
  await Promise.all([
    loadNotificationEvents(),
    loadNotificationChannels(),
    loadNotificationSubscriptions(),
  ])
})
</script>

<template>
  <dialog ref="createChannelDialog" class="dialog" @close="resetChannelForm">
    <h2>Add notification channel</h2>
    <p>Choose how SickRock should deliver notifications to you.</p>

    <FormLayout @submit.prevent="createNotificationChannel">
      <FormField label="Channel type" for="channel-type" :disabled="creatingChannel">
        <select id="channel-type" v-model="newChannelType" :disabled="creatingChannel">
          <option value="email">Email</option>
          <option value="telegram">Telegram</option>
          <option value="webhook">Webhook</option>
        </select>
      </FormField>

      <FormField :label="channelValueLabel" for="channel-value" :disabled="creatingChannel">
        <input
          id="channel-value"
          ref="channelValueInput"
          v-model="newChannelValue"
          type="text"
          :placeholder="channelValuePlaceholder"
          autocomplete="off"
          :disabled="creatingChannel"
          required
        />
      </FormField>

      <FormField
        v-if="newChannelType === 'webhook'"
        label="Channel name (optional)"
        for="channel-name"
        :disabled="creatingChannel"
      >
        <input
          id="channel-name"
          v-model="newChannelName"
          type="text"
          placeholder="e.g. Team webhook"
          :disabled="creatingChannel"
        />
      </FormField>

      <NotificationBlock v-if="channelCreateError" type="error" :message="channelCreateError" />

      <template #actions>
        <button type="button" class="neutral" :disabled="creatingChannel" @click="closeCreateChannelDialog">
          Cancel
        </button>
        <button type="submit" class="good" :disabled="creatingChannel || !newChannelValue.trim()">
          {{ creatingChannel ? 'Adding…' : 'Add channel' }}
        </button>
      </template>
    </FormLayout>
  </dialog>

  <dialog ref="createSubscriptionDialog" class="dialog" @close="resetSubscriptionForm">
    <h2>Subscribe to event</h2>
    <p>Pick an event and the channel that should receive it.</p>

    <FormLayout @submit.prevent="createNotificationSubscription">
      <FormField label="Event" for="subscription-event" :disabled="creatingSubscription">
        <select
          id="subscription-event"
          v-model="newSubscriptionEventCode"
          :disabled="creatingSubscription"
          required
        >
          <option value="">Select an event…</option>
          <option v-for="event in notificationEvents" :key="event.id" :value="event.eventCode">
            {{ event.eventName }} — {{ event.description }}
          </option>
        </select>
      </FormField>

      <FormField label="Channel" for="subscription-channel" :disabled="creatingSubscription">
        <select
          id="subscription-channel"
          v-model="newSubscriptionChannelId"
          :disabled="creatingSubscription"
          required
        >
          <option :value="0">Select a channel…</option>
          <option v-for="channel in activeChannels" :key="channel.id" :value="channel.id">
            {{ channel.channelType.toUpperCase()
            }}{{ channel.channelName ? ` — ${channel.channelName}` : '' }}: {{ channel.channelValue }}
          </option>
        </select>
      </FormField>

      <NotificationBlock v-if="subscriptionCreateError" type="error" :message="subscriptionCreateError" />

      <template #actions>
        <button
          type="button"
          class="neutral"
          :disabled="creatingSubscription"
          @click="closeCreateSubscriptionDialog"
        >
          Cancel
        </button>
        <button
          type="submit"
          class="good"
          :disabled="
            creatingSubscription || !newSubscriptionEventCode || !newSubscriptionChannelId
          "
        >
          {{ creatingSubscription ? 'Saving…' : 'Subscribe' }}
        </button>
      </template>
    </FormLayout>
  </dialog>

  <Section
    title="Notification channels"
    subtitle="Configure email, Telegram, or webhook destinations for alerts."
    :icon="NotificationIcon"
    :padding="false"
  >
    <template #toolbar>
      <button
        type="button"
        class="inline-icon neutral"
        aria-label="Refresh channels"
        :disabled="channelsLoading"
        @click="loadNotificationChannels"
      >
        <HugeiconsIcon :icon="RefreshIcon" width="1em" height="1em" :strokeWidth="iconStrokeWidth" />
      </button>
      <button
        type="button"
        class="inline-icon good"
        aria-label="Add channel"
        :disabled="channelsLoading"
        @click="openCreateChannelDialog"
      >
        <HugeiconsIcon :icon="Add01Icon" width="1em" height="1em" :strokeWidth="iconStrokeWidth" />
      </button>
    </template>

    <div v-if="channelsError" class="list-banner-pad">
      <NotificationBlock type="error" :message="channelsError" />
    </div>
    <div v-if="channelsLoading && !notificationChannels.length" class="list-banner-pad muted">Loading…</div>

    <template v-else>
      <p v-if="!notificationChannels.length" class="list-banner-pad inline-notification note">
        No notification channels yet.
      </p>

      <Table
        v-else
        class="notifications-table-wrap"
        :data="notificationChannels"
        :headers="channelHeaders"
      >
        <template #cell-channelType="{ value, row }">
          <strong :class="{ muted: !row.isActive }">{{ String(value).toUpperCase() }}</strong>
        </template>
        <template #cell-channelName="{ value, row }">
          <span :class="{ muted: !row.isActive }">{{ value || '—' }}</span>
        </template>
        <template #cell-channelValue="{ value, row }">
          <span :class="{ muted: !row.isActive }">{{ value }}</span>
        </template>
        <template #cell-isActive="{ value }">
          <span :class="value ? 'tag fg-good' : 'tag bad'">{{ value ? 'Active' : 'Inactive' }}</span>
        </template>
        <template #cell-actions="{ row }">
          <div class="actions-cell">
            <button
              type="button"
              class="inline-icon bad small"
              aria-label="Delete channel"
              @click="deleteNotificationChannel(row.id)"
            >
              <HugeiconsIcon :icon="Delete01Icon" width="1em" height="1em" :strokeWidth="iconStrokeWidth" />
            </button>
          </div>
        </template>
      </Table>
    </template>
  </Section>

  <Section
    title="Event subscriptions"
    subtitle="Choose which events are sent to each channel."
    :icon="CheckListIcon"
    :padding="false"
  >
    <template #toolbar>
      <button
        type="button"
        class="inline-icon neutral"
        aria-label="Refresh subscriptions"
        :disabled="subscriptionsLoading"
        @click="loadNotificationSubscriptions"
      >
        <HugeiconsIcon :icon="RefreshIcon" width="1em" height="1em" :strokeWidth="iconStrokeWidth" />
      </button>
      <button
        type="button"
        class="inline-icon good"
        aria-label="Subscribe to event"
        :disabled="subscriptionsLoading"
        @click="openCreateSubscriptionDialog"
      >
        <HugeiconsIcon :icon="Add01Icon" width="1em" height="1em" :strokeWidth="iconStrokeWidth" />
      </button>
    </template>

    <div v-if="subscriptionsError" class="list-banner-pad">
      <NotificationBlock type="error" :message="subscriptionsError" />
    </div>
    <div
      v-if="subscriptionsLoading && !notificationSubscriptions.length"
      class="list-banner-pad muted"
    >
      Loading…
    </div>

    <template v-else>
      <p v-if="!notificationSubscriptions.length" class="list-banner-pad inline-notification note">
        No event subscriptions yet.
      </p>

      <Table
        v-else
        class="notifications-table-wrap"
        :data="subscriptionTableRows"
        :headers="subscriptionHeaders"
      >
        <template #cell-eventName="{ value, row }">
          <strong>{{ value }}</strong>
          <span v-if="row.eventDescription" class="muted text-sm block">{{ row.eventDescription }}</span>
        </template>
        <template #cell-channelType="{ value, row }">
          {{ value }}{{ row.channelName ? ` — ${row.channelName}` : '' }}
        </template>
        <template #cell-actions="{ row }">
          <div class="actions-cell">
            <button
              type="button"
              class="inline-icon bad small"
              aria-label="Remove subscription"
              @click="deleteNotificationSubscription(row.id)"
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

.notifications-table-wrap {
  margin-top: 0.5rem;
  margin-bottom: 1.5rem;
}

.actions-cell {
  display: flex;
  justify-content: flex-end;
  gap: 0.35rem;
}

.block {
  display: block;
}
</style>
