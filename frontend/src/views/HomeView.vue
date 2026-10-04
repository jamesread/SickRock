<script setup lang="ts">
import { ref, onMounted, computed, inject, watch, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import type { createApiClient } from '../stores/api'
import Section from 'picocrank/vue/components/Section.vue'
import Navigation from 'picocrank/vue/components/Navigation.vue'
import NavigationGrid from 'picocrank/vue/components/NavigationGrid.vue'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'
import { Calendar03Icon, HomeIcon, Link04Icon, ViewIcon, WorkflowIcon } from '@hugeicons/core-free-icons'
import { resolveNavigationIcon } from '../utils/navigationIcon'

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
const readOnlyExports = ref<Array<{ slug: string; title: string; tableConfiguration: string; displayMode: string }>>([])

const workflowNavigation = ref<InstanceType<typeof Navigation> | null>(null)
const exportsNavigation = ref<InstanceType<typeof Navigation> | null>(null)
const recentNavByGroup = new Map<string, InstanceType<typeof Navigation>>()

const hasRecentlyViewed = computed(() => recentlyViewed.value.length > 0)
const hasWorkflows = computed(() => workflows.value.length > 0)
const hasReadOnlyExports = computed(() => readOnlyExports.value.length > 0)

const groupedRecentlyViewed = computed(() => {
  const groups = new Map<string, { name: string; title: string; icon: string | undefined; items: typeof recentlyViewed.value }>()
  for (const item of recentlyViewed.value) {
    const key = item.name
    const displayTitle = item.table_title || item.name
    const existing = groups.get(key)
    if (existing) {
      existing.items.push(item)
      existing.items.sort((a, b) => Number(b.updated_at_unix) - Number(a.updated_at_unix))
    } else {
      groups.set(key, { name: key, title: displayTitle, icon: item.icon, items: [item] })
    }
  }
  return Array.from(groups.values()).sort((a, b) => {
    const aLatest = a.items.length ? Number(a.items[0].updated_at_unix) : 0
    const bLatest = b.items.length ? Number(b.items[0].updated_at_unix) : 0
    return bLatest - aLatest
  })
})

function formatTime(unixTimestamp: number | bigint): string {
  const timestamp = typeof unixTimestamp === 'bigint' ? Number(unixTimestamp) : unixTimestamp
  const viewedAt = new Date(timestamp * 1000)
  const now = new Date()
  const diffMs = now.getTime() - viewedAt.getTime()
  const diffMins = Math.floor(diffMs / (1000 * 60))
  const diffHours = Math.floor(diffMs / (1000 * 60 * 60))
  const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24))

  if (diffMins < 1) return 'Just now'
  if (diffMins < 60) return `${diffMins}m ago`
  if (diffHours < 24) return `${diffHours}h ago`
  if (diffDays < 7) return `${diffDays}d ago`
  return viewedAt.toLocaleDateString()
}

function startWorkflow(wf: { id: number; name: string }) {
  try {
    localStorage.setItem(PINNED_WORKFLOW_KEY, wf.name)
  } catch {
    // ignore storage errors
  }
  window.dispatchEvent(new CustomEvent('pinned-workflow-changed'))
  router.push(`/workflow/${wf.id}`)
}

function setRecentNavRef(groupName: string, el: unknown) {
  if (el) {
    recentNavByGroup.set(groupName, el as InstanceType<typeof Navigation>)
    nextTick(() => populateRecentNavigation())
  } else {
    recentNavByGroup.delete(groupName)
  }
}

function populateExportsNavigation() {
  const nav = exportsNavigation.value
  if (!nav) return
  nav.clearNavigationLinks()
  for (const exp of readOnlyExports.value) {
    const path = `/exports/${encodeURIComponent(exp.slug)}`
    const modeLabel = exp.displayMode === 'free_busy' ? 'Free/busy' : 'Full calendar'
    nav.addNavigationLink({
      id: `home-export-${exp.slug}`,
      name: `home-export-${exp.slug}`,
      title: exp.title,
      path,
      to: path,
      icon: Link04Icon,
      type: 'route',
      description: `${exp.tableConfiguration} · ${modeLabel}`,
    })
  }
}

function populateWorkflowNavigation() {
  const nav = workflowNavigation.value
  if (!nav) return
  nav.clearNavigationLinks()
  for (const wf of workflows.value) {
    nav.addCallback(wf.name, () => startWorkflow(wf), {
      name: `home-workflow-${wf.id}`,
      icon: resolveNavigationIcon({
        icon: wf.icon,
        workflowId: wf.id,
        path: `/workflow/${wf.id}`,
      }),
      description: 'Start workflow (pins to header)',
    })
  }
}

function populateRecentNavigation() {
  for (const group of groupedRecentlyViewed.value) {
    const nav = recentNavByGroup.get(group.name)
    if (!nav) continue
    nav.clearNavigationLinks()
    const tableIcon = resolveNavigationIcon({
      icon: group.icon,
      path: `/table/${group.name}`,
    })
    nav.addNavigationLink({
      id: `home-table-${group.name}`,
      name: `home-table-${group.name}`,
      title: 'View table',
      path: `/table/${group.name}`,
      to: `/table/${group.name}`,
      icon: tableIcon,
      type: 'route',
      description: group.title,
    })
    for (const item of group.items) {
      const linkName = `home-recent-${group.name}-${item.table_id}`
      nav.addNavigationLink({
        id: linkName,
        name: linkName,
        title: item.item_name || `Row ${item.table_id}`,
        path: `/table/${item.name}/${item.table_id}`,
        to: `/table/${item.name}/${item.table_id}`,
        icon: tableIcon,
        type: 'route',
        description: `${formatTime(item.updated_at_unix)} · ID ${item.table_id}`,
      })
    }
  }
}

function refreshNavigationGrids() {
  nextTick(() => {
    populateExportsNavigation()
    populateWorkflowNavigation()
    populateRecentNavigation()
  })
}

watch([workflows, readOnlyExports, groupedRecentlyViewed], refreshNavigationGrids)

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

    try {
      const recentRes = await client!.getMostRecentlyViewed({ limit: 5 })
      recentlyViewed.value = (recentRes.items || []).map((item) => ({
        name: item.name,
        table_id: item.tableId,
        icon: item.icon,
        updated_at_unix: item.updatedAtUnix,
        item_name: item.itemName,
        table_title: item.tableTitle,
      }))
    } catch (e) {
      console.warn('Failed to load recently viewed items:', e)
    }

    try {
      const exportsRes = await client!.listAccessibleReadOnlyExports({})
      readOnlyExports.value = (exportsRes.exports ?? [])
        .map((e) => ({
          slug: e.slug,
          title: e.title || e.slug,
          tableConfiguration: e.tableConfiguration,
          displayMode: e.displayMode || 'full',
        }))
        .sort((a, b) => a.title.localeCompare(b.title))
    } catch (e) {
      console.warn('Failed to load read-only exports:', e)
      readOnlyExports.value = []
    }

    try {
      const navRes = await client!.getNavigation({})
      const wfList = (navRes as { workflows?: Array<{ id: number; name?: string; icon?: string }> }).workflows || []
      workflows.value = wfList
        .map((w) => ({
          id: w.id,
          name: w.name || `Workflow ${w.id}`,
          icon: w.icon || '',
        }))
        .sort((a, b) => a.name.localeCompare(b.name))
    } catch (e) {
      console.warn('Failed to load workflows:', e)
      workflows.value = []
    }
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
    refreshNavigationGrids()
  }
}

onMounted(load)
</script>

<template>
  <div>
    <Section title="Welcome" :icon="HomeIcon">
      <div v-if="loading">Loading…</div>
      <div v-else>
        <div v-if="!isAuthenticated" class="subtle">Please log in to view your recently viewed items.</div>
        <NotificationBlock v-else-if="error" type="error" :message="error" />
        <div v-else class="meta">
          <p class="version">
            Version: {{ version }}
            <span v-if="commit">({{ commit }})</span>
            <span v-if="date"> on {{ date }}</span>
          </p>
        </div>
      </div>
    </Section>

    <Section
      v-if="!loading && isAuthenticated && !error && hasReadOnlyExports"
      title="Shared calendars"
      subtitle="Read-only calendar exports you can open."
      :icon="Calendar03Icon"
    >
      <Navigation ref="exportsNavigation">
        <NavigationGrid />
      </Navigation>
    </Section>

    <Section
      v-if="!loading && isAuthenticated && !error && hasWorkflows"
      title="Workflows"
      :icon="WorkflowIcon"
    >
      <Navigation ref="workflowNavigation">
        <NavigationGrid />
      </Navigation>
    </Section>

    <Section
      v-if="!loading && isAuthenticated && !error"
      title="Recently Viewed"
      :icon="ViewIcon"
      :padding="false"
    >
      <template v-if="hasRecentlyViewed">
        <template v-for="group in groupedRecentlyViewed" :key="group.name">
          <div class="section-subheader">
            <h3>{{ group.title }}</h3>
          </div>
          <div class="section-content padding">
            <Navigation :ref="(el) => setRecentNavRef(group.name, el)">
              <NavigationGrid compact />
            </Navigation>
          </div>
        </template>
      </template>
      <div v-else class="section-content padding">
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
  color: var(--muted-text-color, #666);
}

.subtle {
  color: var(--muted-text-color, #777);
}

</style>
