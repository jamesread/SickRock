<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'

import Section from 'picocrank/vue/components/Section.vue'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'
import { Calendar03Icon } from '@hugeicons/core-free-icons'
import type { RssCalendarFeed } from '../gen/sickrock_pb'
import { createApiClient } from '../stores/api'

const client = createApiClient()

const feeds = ref<RssCalendarFeed[]>([])
const loading = ref(true)
const error = ref<string | null>(null)
const refreshingId = ref<number | null>(null)
const refreshMessage = ref<string | null>(null)

async function load() {
  loading.value = true
  error.value = null
  try {
    const res = await client.listRssCalendarFeeds({})
    feeds.value = res.feeds ?? []
  } catch (e) {
    error.value = String(e)
  } finally {
    loading.value = false
  }
}

async function refreshFeed(feed: RssCalendarFeed) {
  if (!feed.id) return
  refreshingId.value = feed.id
  refreshMessage.value = null
  error.value = null
  try {
    const res = await client.refreshRssCalendarFeed({ id: feed.id })
    refreshMessage.value = res.message || 'Feed refreshed.'
    if (res.feed) {
      const idx = feeds.value.findIndex(f => f.id === res.feed?.id)
      if (idx >= 0) feeds.value[idx] = res.feed
    }
  } catch (e) {
    error.value = String(e)
  } finally {
    refreshingId.value = null
  }
}

function formatLastRefresh(feed: RssCalendarFeed): string {
  if (!feed.lastRefreshAt) return 'Never'
  const date = new Date(Number(feed.lastRefreshAt) * 1000)
  return Number.isNaN(date.getTime()) ? '—' : date.toLocaleString()
}

onMounted(() => void load())
</script>

<template>
  <Section title="Calendar feeds" :icon="Calendar03Icon">
    <template #toolbar>
      <RouterLink to="/admin/control-panel" class="button neutral">Control Panel</RouterLink>
      <RouterLink to="/admin/rss-calendar-feeds/new" class="button">Add feed</RouterLink>
    </template>

    <NotificationBlock v-if="error" variant="bad">{{ error }}</NotificationBlock>
    <NotificationBlock v-if="refreshMessage" variant="good">{{ refreshMessage }}</NotificationBlock>

    <p class="text-sm">
      Import calendar events from feeds into a table. Feeds refresh automatically every hour by default
      (configurable per feed). Use <strong>Refresh now</strong> for a manual sync.
    </p>

    <div v-if="loading">Loading…</div>
    <table v-else-if="feeds.length > 0" class="table">
      <thead>
        <tr>
          <th>Name</th>
          <th>Table</th>
          <th>Interval</th>
          <th>Last refresh</th>
          <th>Status</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="feed in feeds" :key="feed.id">
          <td>
            <RouterLink :to="`/admin/rss-calendar-feeds/${feed.id}`">{{ feed.name }}</RouterLink>
            <div class="text-sm subtle">{{ feed.feedUrl }}</div>
          </td>
          <td>
            <RouterLink :to="`/table/${feed.tableConfiguration}`">{{ feed.tableConfiguration }}</RouterLink>
          </td>
          <td>{{ feed.refreshIntervalMinutes || 60 }} min</td>
          <td>{{ formatLastRefresh(feed) }}</td>
          <td>
            <span v-if="feed.lastRefreshStatus">{{ feed.lastRefreshStatus }}</span>
            <span v-if="feed.lastItemsCreated || feed.lastItemsUpdated" class="text-sm">
              (+{{ feed.lastItemsCreated }} new, {{ feed.lastItemsUpdated }} updated)
            </span>
          </td>
          <td class="actions">
            <button
              type="button"
              class="button neutral inline"
              :disabled="refreshingId === feed.id"
              @click="refreshFeed(feed)"
            >
              {{ refreshingId === feed.id ? 'Refreshing…' : 'Refresh now' }}
            </button>
            <RouterLink :to="`/admin/rss-calendar-feeds/${feed.id}`" class="button neutral inline">Edit</RouterLink>
          </td>
        </tr>
      </tbody>
    </table>
    <p v-else>No feeds configured yet.</p>
  </Section>
</template>
