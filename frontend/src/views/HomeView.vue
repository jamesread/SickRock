<script setup lang="ts">
import { ref, onMounted, computed, inject } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { HugeiconsIcon } from '@hugeicons/vue'
import * as Hugeicons from '@hugeicons/core-free-icons'
import type { createApiClient } from '../stores/api'
import Section from 'picocrank/vue/components/Section.vue'

// Use global API client
const client = inject<ReturnType<typeof createApiClient>>('apiClient')
const authStore = useAuthStore()
const isAuthenticated = computed(() => authStore.isAuthenticated)
const router = useRouter()
const PINNED_WORKFLOW_KEY = 'sickrock_pinned_workflow'

const loading = ref(true)
const error = ref<string | null>(null)
const version = ref<string>('')
const commit = ref<string>('')
const date = ref<string>('')
const recentlyViewed = ref<Array<{
  name: string
  table_id: string
  icon: string
  updated_at_unix: number | bigint
  item_name: string
  table_title?: string
}>>([])
const workflows = ref<Array<{ id: number; name: string; icon?: string }>>([])

const hasRecentlyViewed = computed(() => recentlyViewed.value.length > 0)
const hasWorkflows = computed(() => workflows.value.length > 0)

// Group recently viewed items by table name
const groupedRecentlyViewed = computed(() => {
  const groups = new Map<string, { name: string; title: string; icon: string | undefined; items: typeof recentlyViewed.value }>()
  for (const item of recentlyViewed.value) {
    const key = item.name
    const displayTitle = item.table_title || item.name
    const existing = groups.get(key)
    if (existing) {
      existing.items.push(item)
      // keep most recent first within group
      existing.items.sort((a, b) => Number(b.updated_at_unix) - Number(a.updated_at_unix))
    } else {
      groups.set(key, { name: key, title: displayTitle, icon: item.icon, items: [item] })
    }
  }
  // order groups by most recent item in each group
  return Array.from(groups.values()).sort((a, b) => {
    const aLatest = a.items.length ? Number(a.items[0].updated_at_unix) : 0
    const bLatest = b.items.length ? Number(b.items[0].updated_at_unix) : 0
    return bLatest - aLatest
  })
})

function formatTime(unixTimestamp: number | bigint): string {
  const timestamp = typeof unixTimestamp === 'bigint' ? Number(unixTimestamp) : unixTimestamp
  const date = new Date(timestamp * 1000)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffMins = Math.floor(diffMs / (1000 * 60))
  const diffHours = Math.floor((diffMs) / (1000 * 60 * 60))
  const diffDays = Math.floor((diffMs) / (1000 * 60 * 60 * 24))

  if (diffMins < 1) return 'Just now'
  if (diffMins < 60) return `${diffMins}m ago`
  if (diffHours < 24) return `${diffHours}h ago`
  if (diffDays < 7) return `${diffDays}d ago`
  return date.toLocaleDateString()
}

async function load() {
  loading.value = true
  error.value = null
  try {
    if (!isAuthenticated.value) {
      loading.value = false
      return
    }
    const init = await client!.init({})
    version.value = init.version
    commit.value = init.commit
    date.value = init.date

    // Load recently viewed items
    try {
      const recentRes = await client!.getMostRecentlyViewed({ limit: 5 })
      recentlyViewed.value = (recentRes.items || []).map(item => ({
        name: item.name,
        table_id: item.tableId,
        icon: item.icon,
        updated_at_unix: item.updatedAtUnix,
        item_name: item.itemName,
        table_title: item.tableTitle
      }))
    } catch (e) {
      // Don't fail the entire load if recently viewed fails
      console.warn('Failed to load recently viewed items:', e)
    }

    // Load workflows
    try {
      const navRes = await client!.getNavigation({})
      const wfList = (navRes as any).workflows || []
      workflows.value = wfList
        .map((w: any) => ({
          id: w.id,
          name: w.name || `Workflow ${w.id}`,
          icon: w.icon || 'DatabaseIcon'
        }))
        .sort((a: any, b: any) => a.name.localeCompare(b.name))
    } catch (e) {
      console.warn('Failed to load workflows:', e)
      workflows.value = []
    }
  } catch (e: any) {
    error.value = String(e?.message || e)
  } finally {
    loading.value = false
  }
}

function startWorkflow(wf: { id: number; name: string }) {
  try {
    localStorage.setItem(PINNED_WORKFLOW_KEY, wf.name)
  } catch {
    // ignore storage errors
  }
  // Notify App.vue to refresh pinned workflow header
  window.dispatchEvent(new CustomEvent('pinned-workflow-changed'))
  router.push(`/workflow/${wf.id}`)
}

onMounted(load)
</script>

<template>
  <div>
    <Section title="Welcome">
      <div v-if="loading">Loading…</div>
      <div v-else>
        <div v-if="!isAuthenticated" class="subtle">Please log in to view your recently viewed items.</div>
        <div v-else-if="error" class="error">{{ error }}</div>
        <div class="meta" v-else>
          <p class="version">Version: {{ version }} <span v-if="commit">({{ commit }})</span> <span v-if="date">on {{ date }}</span></p>
        </div>
      </div>
    </Section>

    <Section v-if="!loading && isAuthenticated && !error && hasWorkflows" title="Workflows">
      <div class="workflows-grid">
        <div
          v-for="wf in workflows"
          :key="wf.id"
          class="workflow-card"
        >
          <router-link
            :to="`/workflow/${wf.id}`"
            class="workflow-card-main"
            :title="wf.name"
          >
            <HugeiconsIcon
              :icon="(Hugeicons as any)[wf.icon || 'DatabaseIcon'] || Hugeicons.DatabaseIcon"
              class="workflow-card-icon"
            />
            <div class="workflow-card-info">
              <h4 class="workflow-card-title">{{ wf.name }}</h4>
              <p class="workflow-card-subtitle">Workflow</p>
            </div>
          </router-link>
          <div class="workflow-card-actions">
            <button
              type="button"
              class="button primary small good"
              @click="startWorkflow(wf)"
            >
              Start Workflow
            </button>
          </div>
        </div>
      </div>
    </Section>

    <Section v-if="!loading && isAuthenticated && !error" title="Recently Viewed">
      <div v-if="hasRecentlyViewed">
        <div class="recently-viewed-groups">
          <div
            v-for="group in groupedRecentlyViewed"
            :key="group.name"
            class="recent-group"
          >
            <div class="group-header">
              <router-link
                :to="`/table/${group.name}`"
                class="table-heading-link"
                title="View table"
              >
                <HugeiconsIcon
                  v-if="group.icon && (Hugeicons as any)[group.icon]"
                  :icon="(Hugeicons as any)[group.icon]"
                />
                <h4>{{ group.title }}</h4>
              </router-link>
            </div>

            <div class="recently-viewed">
              <router-link
                v-for="item in group.items"
                :key="`${item.name}-${item.table_id}`"
                :to="`/table/${item.name}/${item.table_id}`"
                class="recent-item"
              >
                <div class="item-header">
                  <div class="item-info">
                    <span class="item-name">{{ item.item_name }}</span>
                    <span class="table-name">ID: {{ item.table_id }}</span>
                  </div>
                </div>
                <div class="item-details">
                  <span class="item-time">{{ formatTime(item.updated_at_unix) }}</span>
                </div>
              </router-link>
            </div>
          </div>
        </div>
      </div>
      <div v-else>
        <p class="subtle">No recently viewed items yet. Browse your tables to see them here.</p>
      </div>
    </Section>
  </div>
</template>

<style scoped>
.meta {
  margin-top: 0.5rem;
}

.version {
  color: #666;
}

/* Workflows list styled similarly to recently viewed cards */
.workflows-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 0.75rem;
}

.workflow-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.75rem;
  background: #f8f9fa;
  border: 1px solid #e9ecef;
  border-radius: 6px;
  transition: all 0.2s ease;
}

.workflow-card:hover {
  background: #e9ecef;
  border-color: #dee2e6;
  transform: translateY(-1px);
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

.workflow-card-main {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  text-decoration: none;
  color: inherit;
  flex: 1;
}

.workflow-card-icon {
  width: 20px;
  height: 20px;
}

.workflow-card-info {
  display: flex;
  flex-direction: column;
  gap: 0.125rem;
}

.workflow-card-title {
  margin: 0;
  font-weight: 500;
  font-size: 0.95rem;
}

.workflow-card-subtitle {
  margin: 0;
  font-size: 0.8rem;
  color: #6c757d;
}

.workflow-card-actions {
  display: flex;
  align-items: center;
  margin-left: 0.75rem;
}

/* Groups */
.recently-viewed-groups {
  display: grid;
  grid-auto-flow: dense;
  grid-template-columns: repeat(auto-fit, minmax(500px, 1fr));
  gap: 1rem;
}

.recent-group {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.group-header {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.table-heading-link {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  text-decoration: none;
  color: inherit;
  transition: color 0.2s ease;
}

.table-heading-link:hover {
  color: #007bff;
}

.table-heading-link h4 {
  margin: 0;
}

/* Recently Viewed Styles */
.recently-viewed {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.recent-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.75rem;
  background: #f8f9fa;
  border: 1px solid #e9ecef;
  border-radius: 6px;
  text-decoration: none;
  color: inherit;
  transition: all 0.2s ease;
  cursor: pointer;
}

.recent-item:hover {
  background: #e9ecef;
  border-color: #dee2e6;
  transform: translateY(-1px);
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

.item-header {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex: 1;
}

.item-info {
  display: flex;
  flex-direction: column;
  gap: 0.125rem;
}

.item-name {
  font-weight: 500;
  font-size: 0.95rem;
}

.table-name {
  font-size: 0.8rem;
  color: #6c757d;
  font-weight: 400;
}

.item-details {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  margin: 0 1rem;
  font-size: 0.875rem;
  color: #6c757d;
}

.item-time {
  font-size: 0.75rem;
}

.subtle {
  color: #777;
}

.error {
  background: #f8d7da;
  color: #721c24;
  padding: 0.75rem;
  border: 1px solid #f5c6cb;
  border-radius: 4px;
}
</style>
