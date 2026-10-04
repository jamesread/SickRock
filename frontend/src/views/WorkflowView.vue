<script setup lang="ts">
import { ref, onMounted, computed, watch, nextTick } from 'vue'
import { useRoute } from 'vue-router'
import { inject } from 'vue'
import type { createApiClient } from '../stores/api'
import Section from 'picocrank/vue/components/Section.vue'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'
import Navigation from 'picocrank/vue/components/Navigation.vue'
import NavigationGrid from 'picocrank/vue/components/NavigationGrid.vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { DatabaseIcon, Edit03Icon, PlayIcon, StopIcon } from '@hugeicons/core-free-icons'
import * as Hugeicons from '@hugeicons/core-free-icons'
import { resolveNavigationIcon } from '../utils/navigationIcon'

type WorkflowItem = {
  id: number
  title: string
  tableName: string
  dashboardName: string
  dashboardId: number
  icon: string
  path: string
  ordinal: number
}

type AvailableNavItem = {
  id: number
  title: string
  tableName: string
  dashboardName: string
  dashboardId: number
  icon: string
}

const client = inject<ReturnType<typeof createApiClient>>('apiClient')
const route = useRoute()

const loading = ref(true)
const error = ref<string | null>(null)
const workflowName = ref<string>('')
const workflowId = ref<number | null>(null)
const workflowIcon = ref<string>('')
const PINNED_WORKFLOW_KEY = 'sickrock_pinned_workflow'
const isPinned = ref(false)
const items = ref<WorkflowItem[]>([])
const workflowNavigation = ref<InstanceType<typeof Navigation> | null>(null)

const sectionIcon = computed(() =>
  resolveNavigationIcon({
    icon: workflowIcon.value,
    workflowId: workflowId.value ?? undefined,
    path: workflowId.value ? `/workflow/${workflowId.value}` : undefined,
  }),
)

const showAddModal = ref(false)
const availableItems = ref<AvailableNavItem[]>([])
const selectedItemIds = ref<Set<number>>(new Set())
const orderedSelectedIds = ref<number[]>([])
const addingItems = ref(false)

function itemSubtitle(item: { dashboardId: number; tableName?: string }) {
  if (item.dashboardId > 0) return 'Dashboard'
  return item.tableName ? `Table · ${item.tableName}` : 'Table'
}

function resolveItemIcon(item: { icon: string; dashboardId: number; path?: string }) {
  const iconName = item.icon?.trim()
  if (iconName && (Hugeicons as Record<string, unknown>)[iconName]) {
    return (Hugeicons as Record<string, typeof DatabaseIcon>)[iconName]
  }
  return resolveNavigationIcon({
    icon: item.icon,
    dashboardId: item.dashboardId,
    path: item.path,
  })
}

function populateWorkflowNavigation() {
  const nav = workflowNavigation.value
  if (!nav) return
  nav.clearNavigationLinks()
  for (const item of items.value) {
    nav.addNavigationLink({
      id: `workflow-item-${item.id}`,
      name: `workflow-item-${item.id}`,
      title: item.title,
      path: item.path,
      to: item.path,
      icon: resolveItemIcon(item),
      type: 'route',
      description: itemSubtitle(item),
    })
  }
}

function refreshWorkflowNavigation() {
  nextTick(() => populateWorkflowNavigation())
}

watch(items, refreshWorkflowNavigation, { deep: true })

async function load() {
  loading.value = true
  error.value = null
  try {
    const idParam = Number(route.params.workflowId || route.params.id || 0)
    if (!idParam || Number.isNaN(idParam)) {
      throw new Error('Invalid workflow id')
    }

    const navResponse = await client!.getNavigation({})
    const workflow = (navResponse as { workflows?: Array<{ id: number; name?: string; icon?: string; items?: unknown[] }> })
      .workflows?.find((w) => Number(w.id) === idParam)

    if (!workflow) {
      throw new Error(`Workflow not found: ${idParam}`)
    }

    workflowId.value = workflow.id
    workflowIcon.value = workflow.icon || ''
    workflowName.value = workflow.name || String(workflow.id)

    items.value = (workflow.items || []).map((raw: unknown) => {
      const item = raw as {
        id: number
        title?: string
        tableTitle?: string
        tableName?: string
        dashboardName?: string
        dashboardId?: number
        icon?: string
        ordinal?: number
      }
      const title = item.title || item.tableTitle || item.tableName || String(item.id)
      const dashboardId = item.dashboardId ?? 0
      const path = dashboardId > 0 ? `/dashboard/${item.dashboardName}` : `/table/${item.tableName}`
      return {
        id: item.id,
        title,
        tableName: item.tableName ?? '',
        dashboardName: item.dashboardName ?? '',
        dashboardId,
        icon: item.icon || '',
        path,
        ordinal: item.ordinal || 0,
      }
    })

    items.value.sort((a, b) => a.ordinal - b.ordinal)

    try {
      const stored = localStorage.getItem(PINNED_WORKFLOW_KEY)
      isPinned.value = !!stored && stored === workflowName.value
    } catch {
      isPinned.value = false
    }
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
    refreshWorkflowNavigation()
  }
}

async function saveWorkflowMembers(orderedNavIds: number[]) {
  if (!workflowId.value) return
  await client!.setWorkflowNavigationMembers({
    workflowId: workflowId.value,
    members: orderedNavIds.map((navigationItemId, idx) => ({
      navigationItemId,
      ordinal: (idx + 1) * 10,
    })),
  })
}

async function openAddModal() {
  showAddModal.value = true
  selectedItemIds.value = new Set()
  orderedSelectedIds.value = []

  try {
    const navResponse = await client!.getNavigation({})
    const allItems = (navResponse.items || []).filter(
      (item: { tableName?: string; dashboardId?: number }) => item.tableName || (item.dashboardId && item.dashboardId > 0),
    )

    availableItems.value = allItems.map((item: {
      id: number
      title?: string
      tableTitle?: string
      tableName?: string
      dashboardName?: string
      dashboardId?: number
      icon?: string
    }) => {
      const title = item.title || item.tableTitle || item.tableName || String(item.id)
      return {
        id: item.id,
        title,
        tableName: item.tableName ?? '',
        dashboardName: item.dashboardName ?? '',
        dashboardId: item.dashboardId ?? 0,
        icon: item.icon || '',
      }
    })

    orderedSelectedIds.value = items.value.map((it) => it.id)
    selectedItemIds.value = new Set(orderedSelectedIds.value)
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
    showAddModal.value = false
  }
}

function availableItemById(id: number): AvailableNavItem | undefined {
  return availableItems.value.find((it) => it.id === id)
}

function toggleItemSelection(itemId: number) {
  if (selectedItemIds.value.has(itemId)) {
    selectedItemIds.value.delete(itemId)
    orderedSelectedIds.value = orderedSelectedIds.value.filter((id) => id !== itemId)
  } else {
    selectedItemIds.value.add(itemId)
    orderedSelectedIds.value.push(itemId)
  }
}

function moveModalMemberUp(index: number) {
  if (index <= 0 || addingItems.value) return
  const next = [...orderedSelectedIds.value]
  ;[next[index - 1], next[index]] = [next[index], next[index - 1]]
  orderedSelectedIds.value = next
}

function moveModalMemberDown(index: number) {
  if (index >= orderedSelectedIds.value.length - 1 || addingItems.value) return
  const next = [...orderedSelectedIds.value]
  ;[next[index], next[index + 1]] = [next[index + 1], next[index]]
  orderedSelectedIds.value = next
}

async function addSelectedItems() {
  addingItems.value = true
  error.value = null

  try {
    const orderedIds = orderedSelectedIds.value.filter((id) => selectedItemIds.value.has(id))
    await saveWorkflowMembers(orderedIds)
    showAddModal.value = false
    await load()
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    addingItems.value = false
  }
}

function closeModal() {
  showAddModal.value = false
  selectedItemIds.value = new Set()
  orderedSelectedIds.value = []
}

function togglePin() {
  try {
    if (isPinned.value) {
      localStorage.removeItem(PINNED_WORKFLOW_KEY)
      isPinned.value = false
    } else {
      localStorage.setItem(PINNED_WORKFLOW_KEY, workflowName.value)
      isPinned.value = true
    }
    window.dispatchEvent(new CustomEvent('pinned-workflow-changed'))
  } catch {
    // ignore storage errors
  }
}

const unselectedAvailableItems = computed(() =>
  availableItems.value.filter((item) => !selectedItemIds.value.has(item.id)),
)

onMounted(load)
</script>

<template>
  <Section :title="workflowName" :icon="sectionIcon">
    <template #toolbar>
      <button
        v-if="workflowId"
        type="button"
        class="button inline-icon neutral"
        @click="openAddModal"
      >
        <HugeiconsIcon :icon="Edit03Icon" width="1em" height="1em" aria-hidden="true" />
        <span>Change Navigation Links</span>
      </button>
      <button
        v-if="workflowId"
        type="button"
        class="button inline-icon"
        :class="isPinned ? 'bad' : 'good'"
        @click="togglePin"
      >
        <HugeiconsIcon
          :icon="isPinned ? StopIcon : PlayIcon"
          width="1em"
          height="1em"
          aria-hidden="true"
        />
        <span>{{ isPinned ? 'Stop Workflow' : 'Start Workflow' }}</span>
      </button>
    </template>

    <div v-if="loading" class="muted">Loading…</div>
    <NotificationBlock v-else-if="error" type="error" :message="error" />
    <template v-else>
      <p v-if="items.length === 0" class="inline-notification note">
        No navigation links yet. Use <strong>Change Navigation Links</strong> in the toolbar to add steps to this workflow.
      </p>
      <Navigation v-else ref="workflowNavigation">
        <NavigationGrid />
      </Navigation>
    </template>
  </Section>

  <div v-if="showAddModal" class="modal-overlay" @click.self="closeModal">
    <div class="modal-content">
      <div class="modal-header">
        <h2>Workflow members for {{ workflowName }}</h2>
        <button type="button" class="modal-close neutral" @click="closeModal" aria-label="Close">×</button>
      </div>
      <div class="modal-body">
        <p class="subtle modal-hint">
          Choose sidebar links for this workflow and set their order. To add a sidebar entry that opens a workflow
          directly, edit Navigation — not this dialog.
        </p>

        <h3 class="modal-subtitle">Selected members (order)</h3>
        <p v-if="!orderedSelectedIds.length" class="subtle">No members selected. Add links below.</p>
        <ul v-else class="member-order-list">
          <li v-for="(memberId, index) in orderedSelectedIds" :key="memberId" class="member-order-row">
            <div class="member-order-main">
              <HugeiconsIcon
                :icon="resolveItemIcon(availableItemById(memberId) ?? { icon: '', dashboardId: 0 })"
                width="1.25em"
                height="1.25em"
                aria-hidden="true"
              />
              <span class="member-order-title">{{ availableItemById(memberId)?.title ?? `Link #${memberId}` }}</span>
            </div>
            <div class="member-order-actions">
              <button
                type="button"
                class="inline-icon neutral"
                title="Move up"
                aria-label="Move up"
                :disabled="index === 0 || addingItems"
                @click="moveModalMemberUp(index)"
              >
                ↑
              </button>
              <button
                type="button"
                class="inline-icon neutral"
                title="Move down"
                aria-label="Move down"
                :disabled="index === orderedSelectedIds.length - 1 || addingItems"
                @click="moveModalMemberDown(index)"
              >
                ↓
              </button>
              <button
                type="button"
                class="inline-icon bad"
                title="Remove from workflow"
                aria-label="Remove from workflow"
                :disabled="addingItems"
                @click="toggleItemSelection(memberId)"
              >
                ✕
              </button>
            </div>
          </li>
        </ul>

        <h3 class="modal-subtitle">Add navigation links</h3>
        <p v-if="!unselectedAvailableItems.length" class="subtle">All navigation links are already members.</p>
        <ul v-else class="items-list">
          <li
            v-for="item in unselectedAvailableItems"
            :key="item.id"
            class="item-option"
            @click="toggleItemSelection(item.id)"
          >
            <HugeiconsIcon :icon="resolveItemIcon(item)" width="1.25em" height="1.25em" aria-hidden="true" />
            <div class="item-info">
              <div class="item-title">{{ item.title }}</div>
              <div class="subtle item-subtitle">{{ itemSubtitle(item) }}</div>
            </div>
            <span class="tag note">Add</span>
          </li>
        </ul>
      </div>
      <div class="modal-footer">
        <button type="button" class="button neutral" @click="closeModal" :disabled="addingItems">
          Cancel
        </button>
        <button type="button" class="button good" @click="addSelectedItems" :disabled="addingItems">
          {{ addingItems ? 'Saving…' : 'Save Changes' }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.modal-overlay {
  position: fixed;
  inset: 0;
  background: color-mix(in srgb, var(--body-bg-color, #000) 45%, transparent);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  padding: 20px;
}

.modal-content {
  background: var(--section-bg-color, #fff);
  color: var(--text-color, inherit);
  border-radius: 8px;
  box-shadow: var(--menu-dropdown-shadow, 0 4px 20px rgba(0, 0, 0, 0.15));
  max-width: 640px;
  width: 100%;
  max-height: 80vh;
  display: flex;
  flex-direction: column;
}

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1rem 1.25rem;
  border-bottom: 1px solid var(--border-color, #e9ecef);
}

.modal-header h2 {
  margin: 0;
  font-size: 1.15rem;
}

.modal-close {
  font-size: 1.5rem;
  line-height: 1;
  padding: 0.25rem 0.5rem;
}

.modal-body {
  padding: 1rem 1.25rem;
  overflow-y: auto;
  flex: 1;
}

.modal-hint {
  margin-top: 0;
}

.modal-subtitle {
  margin: 1.25rem 0 0.5rem;
  font-size: 1rem;
}

.member-order-list,
.items-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.member-order-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.5rem 0.65rem;
  border: 1px solid var(--border-color, #e9ecef);
  border-radius: 6px;
  background: var(--standout-bg-color, #f8f9fa);
}

.member-order-main {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  min-width: 0;
}

.member-order-title {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.member-order-actions {
  display: flex;
  align-items: center;
  gap: 0.25rem;
  flex-shrink: 0;
}

.item-option {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.65rem 0.75rem;
  border: 1px solid var(--border-color, #e9ecef);
  border-radius: 6px;
  cursor: pointer;
}

.item-option:hover {
  background: var(--hover-background-color, #f8f9fa);
}

.item-info {
  flex: 1;
  min-width: 0;
}

.item-title {
  font-weight: 600;
}

.item-subtitle {
  font-size: 0.9rem;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 0.75rem;
  padding: 1rem 1.25rem;
  border-top: 1px solid var(--border-color, #e9ecef);
}

.muted {
  color: var(--muted-text-color, #6c757d);
}
</style>
