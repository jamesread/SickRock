<script setup lang="ts">
import { ref, onMounted, computed, inject, watch, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import type { createApiClient } from '../stores/api'
import Section from 'picocrank/vue/components/Section.vue'
import Navigation from 'picocrank/vue/components/Navigation.vue'
import NavigationGrid from 'picocrank/vue/components/NavigationGrid.vue'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'
import { Bookmark01Icon, Calendar03Icon, DashboardSquare01Icon, Edit03Icon, FlashIcon, HomeIcon, Link04Icon, ViewIcon, WorkflowIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/vue'
import HugeIcon from '../components/HugeIcon.vue'
import HomeLayoutEditor from '../components/HomeLayoutEditor.vue'
import { useDesignMode } from '../composables/useDesignMode'
import {
  defaultHomeLayout,
  navigationItemPath,
  parseHomeLayout,
  quickAccessLinks,
  serializeHomeLayout,
  visibleHomeSections,
  type HomeLayout,
  type HomeLink,
} from '../utils/homeLayout'
import { resolveNavigationIcon } from '../utils/navigationIcon'

const client = inject<ReturnType<typeof createApiClient>>('apiClient')
const authStore = useAuthStore()
const isAuthenticated = computed(() => authStore.isAuthenticated)
const router = useRouter()
const { designModeEnabled } = useDesignMode()
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

// The user's home layout (Edit home, in design mode): section order and visibility, quick access.
const layout = ref<HomeLayout>(defaultHomeLayout())
const bookmarkLinks = ref<HomeLink[]>([])
const pageLinks = ref<HomeLink[]>([])
const layoutEditor = ref<InstanceType<typeof HomeLayoutEditor> | null>(null)
const savingLayout = ref(false)
const layoutError = ref<string | null>(null)
const shownSections = computed(() => visibleHomeSections(layout.value))
const quickLinks = computed(() => quickAccessLinks(layout.value, bookmarkLinks.value))
const bookmarksNavigation = ref<InstanceType<typeof Navigation> | null>(null)
const pagesNavigation = ref<InstanceType<typeof Navigation> | null>(null)

type NavigationApi = InstanceType<typeof Navigation>
/** Template refs inside the sections' v-for are arrays; each holds at most one grid. */
function navOf(r: { value: unknown }): NavigationApi | null {
  const value = r.value as NavigationApi | NavigationApi[] | null
  return (Array.isArray(value) ? value[0] : value) ?? null
}

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
  const nav = navOf(exportsNavigation)
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

// Bumped on every grid rebuild so an icon that loads late never re-adds a stale link.
let navigationGeneration = 0

function populateWorkflowNavigation() {
  const nav = navOf(workflowNavigation)
  if (!nav) return
  nav.clearNavigationLinks()
  const generation = navigationGeneration
  for (const wf of workflows.value) {
    const options = {
      name: `home-workflow-${wf.id}`,
      description: 'Start workflow (pins to header)',
    }
    const icon = resolveNavigationIcon(
      { icon: wf.icon, workflowId: wf.id, path: `/workflow/${wf.id}` },
      (loaded) => {
        if (generation === navigationGeneration) nav.addCallback(wf.name, () => startWorkflow(wf), { ...options, icon: loaded })
      },
    )
    nav.addCallback(wf.name, () => startWorkflow(wf), { ...options, icon })
  }
}

function populateRecentNavigation() {
  for (const group of groupedRecentlyViewed.value) {
    const nav = recentNavByGroup.get(group.name)
    if (!nav) continue
    nav.clearNavigationLinks()
    const generation = navigationGeneration
    const links = group.items.map((item) => {
      const linkName = `home-recent-${group.name}-${item.table_id}`
      return {
        id: linkName,
        name: linkName,
        title: item.item_name || `Row ${item.table_id}`,
        path: `/table/${item.name}/${item.table_id}`,
        to: `/table/${item.name}/${item.table_id}`,
        type: 'route',
        description: `${formatTime(item.updated_at_unix)} · ID ${item.table_id}`,
      }
    })
    const tableIcon = resolveNavigationIcon(
      { icon: group.icon, path: `/table/${group.name}` },
      (loaded) => {
        if (generation !== navigationGeneration) return
        for (const link of links) nav.addNavigationLink({ ...link, icon: loaded })
      },
    )
    for (const link of links) {
      nav.addNavigationLink({ ...link, icon: tableIcon })
    }
  }
}

/** Fills a grid of links (all bookmarks, or all pages) with their icons. */
function populateLinkNavigation(nav: InstanceType<typeof Navigation> | null, links: HomeLink[], prefix: string) {
  if (!nav) return
  nav.clearNavigationLinks()
  const generation = navigationGeneration
  for (const link of links) {
    const options = { id: `${prefix}-${link.id}`, name: `${prefix}-${link.id}`, title: link.title, path: link.path, to: link.path, type: 'route' }
    const icon = resolveNavigationIcon({ icon: link.icon, path: link.path }, (loaded) => {
      if (generation === navigationGeneration) nav.addNavigationLink({ ...options, icon: loaded })
    })
    nav.addNavigationLink({ ...options, icon })
  }
}

function refreshNavigationGrids() {
  nextTick(() => {
    navigationGeneration++
    populateExportsNavigation()
    populateWorkflowNavigation()
    populateRecentNavigation()
    populateLinkNavigation(navOf(bookmarksNavigation), bookmarkLinks.value, 'home-bookmark')
    populateLinkNavigation(navOf(pagesNavigation), pageLinks.value, 'home-page')
  })
}

watch([workflows, readOnlyExports, groupedRecentlyViewed, bookmarkLinks, pageLinks, shownSections], refreshNavigationGrids)

function openLayoutEditor() {
  layoutError.value = null
  layoutEditor.value?.show()
}

async function saveLayout(next: HomeLayout) {
  savingLayout.value = true
  layoutError.value = null
  try {
    await client!.setHomeLayout({ layoutJson: serializeHomeLayout(next) })
    layout.value = parseHomeLayout(serializeHomeLayout(next))
    layoutEditor.value?.close()
  } catch (e) {
    layoutError.value = e instanceof Error ? e.message : String(e)
  } finally {
    savingLayout.value = false
  }
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
      const layoutRes = await client!.getHomeLayout({})
      layout.value = parseHomeLayout(layoutRes.layoutJson)
    } catch (e) {
      console.warn('Failed to load home layout:', e)
    }

    try {
      const navRes = await client!.getNavigation({})
      bookmarkLinks.value = (navRes.bookmarks ?? [])
        .filter(b => b.navigationItem)
        .map(b => ({
          id: b.navigationItemId,
          title: b.title || b.navigationItem!.title || b.navigationItem!.tableTitle || b.navigationItem!.tableName,
          path: navigationItemPath(b.navigationItem!),
          icon: b.navigationItem!.icon,
        }))
        .filter(b => b.path)
      pageLinks.value = (navRes.items ?? [])
        .map(item => ({ id: item.id, title: item.title || item.tableTitle || item.tableName, path: navigationItemPath(item), icon: item.icon }))
        .filter(item => item.path)
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
    <template v-for="sectionId in shownSections" :key="sectionId">
      <Section v-if="sectionId === 'welcome'" title="Welcome" :icon="HomeIcon">
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
        v-else-if="sectionId === 'quick-access' && !loading && quickLinks.length"
        title="Quick access"
        :icon="FlashIcon"
      >
        <div class="action-row quick-access">
          <router-link v-for="link in quickLinks" :key="link.id" :to="link.path" class="button inline-icon">
            <HugeIcon :name="link.icon" :fallback="Bookmark01Icon" width="1em" height="1em" aria-hidden="true" />
            <span>{{ link.title }}</span>
          </router-link>
        </div>
      </Section>

      <Section
        v-else-if="sectionId === 'shared-calendars' && !loading && isAuthenticated && !error && hasReadOnlyExports"
        title="Shared calendars"
        subtitle="Read-only calendar exports you can open."
        :icon="Calendar03Icon"
      >
        <Navigation ref="exportsNavigation">
          <NavigationGrid />
        </Navigation>
      </Section>

      <Section
        v-else-if="sectionId === 'workflows' && !loading && isAuthenticated && !error && hasWorkflows"
        title="Workflows"
        :icon="WorkflowIcon"
      >
        <Navigation ref="workflowNavigation">
          <NavigationGrid />
        </Navigation>
      </Section>

      <Section
        v-else-if="sectionId === 'recently-viewed' && !loading && isAuthenticated && !error"
        title="Recently Viewed"
        :icon="ViewIcon"
        :padding="false"
      >
        <template v-if="hasRecentlyViewed">
          <template v-for="group in groupedRecentlyViewed" :key="group.name">
            <div class="section-subheader">
              <h3>
                <router-link :to="`/table/${group.name}`" :title="`View table: ${group.title}`">
                  {{ group.title }}
                </router-link>
              </h3>
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

      <Section
        v-else-if="sectionId === 'bookmarks' && !loading && isAuthenticated && !error"
        title="Bookmarks"
        :icon="Bookmark01Icon"
      >
        <Navigation v-if="bookmarkLinks.length" ref="bookmarksNavigation">
          <NavigationGrid />
        </Navigation>
        <p v-else class="subtle">No bookmarks yet. Bookmark a page with the bookmark button in the header.</p>
      </Section>

      <Section
        v-else-if="sectionId === 'pages' && !loading && isAuthenticated && !error && pageLinks.length"
        title="Pages"
        :icon="DashboardSquare01Icon"
      >
        <Navigation ref="pagesNavigation">
          <NavigationGrid />
        </Navigation>
      </Section>
    </template>

    <div v-if="designModeEnabled && isAuthenticated && !loading" class="action-row home-edit">
      <button type="button" class="inline-icon design-only" @click="openLayoutEditor">
        <HugeiconsIcon :icon="Edit03Icon" width="1em" height="1em" aria-hidden="true" />
        <span>Edit home</span>
      </button>
    </div>
    <HomeLayoutEditor
      ref="layoutEditor"
      :layout="layout"
      :bookmarks="bookmarkLinks"
      :saving="savingLayout"
      :error="layoutError"
      @save="saveLayout"
    />
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

.home-edit {
  justify-content: center;
  margin: 1rem;
}

/* Links styled as buttons, unlike <button>s, default to content-box and overflow a full-width row. */
.quick-access {
  justify-content: flex-start;
}

.quick-access a.button {
  box-sizing: border-box;
}

</style>
