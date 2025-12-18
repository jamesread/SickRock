<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { inject } from 'vue'
import type { createApiClient } from '../stores/api'
import Section from 'picocrank/vue/components/Section.vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { DatabaseIcon } from '@hugeicons/core-free-icons'
import * as Hugeicons from '@hugeicons/core-free-icons'

const client = inject<ReturnType<typeof createApiClient>>('apiClient')
const route = useRoute()
const router = useRouter()

const loading = ref(true)
const error = ref<string | null>(null)
const workflowName = ref<string>('')
const workflowId = ref<number | null>(null)
const workflowIcon = ref<string>('')
const PINNED_WORKFLOW_KEY = 'sickrock_pinned_workflow'
const isPinned = ref(false)
const items = ref<Array<{
  id: number;
  title: string;
  tableName: string;
  dashboardName: string;
  dashboardId: number;
  icon: string;
  path: string;
}>>([])

// Modal state
const showAddModal = ref(false)
const availableItems = ref<Array<{
  id: number;
  title: string;
  tableName: string;
  dashboardName: string;
  dashboardId: number;
  icon: string;
  workflowId: number;
}>>([])
const selectedItemIds = ref<Set<number>>(new Set())
const addingItems = ref(false)

async function load() {
  loading.value = true
  error.value = null
  try {
    const idParam = Number(route.params.workflowId || route.params.id || 0)
    if (!idParam || Number.isNaN(idParam)) {
      throw new Error('Invalid workflow id')
    }

    const navResponse = await client.getNavigation({})
    const workflow = (navResponse as any).workflows?.find((w: any) => Number(w.id) === idParam)

    if (!workflow) {
      throw new Error(`Workflow not found: ${idParam}`)
    }

    workflowId.value = workflow.id
    workflowIcon.value = workflow.icon || 'DatabaseIcon'
    workflowName.value = workflow.name || String(workflow.id)

    items.value = (workflow.items || []).map((item: any) => {
      const title = item.title || item.tableTitle || item.tableName || String(item.id)
      const path = item.dashboardId > 0
        ? `/dashboard/${item.dashboardName}`
        : `/table/${item.tableName}`
      const icon = item.icon || 'DatabaseIcon'

      return {
        id: item.id,
        title,
        tableName: item.tableName,
        dashboardName: item.dashboardName,
        dashboardId: item.dashboardId,
        icon,
        path
      }
    })

    // Sync pin state for this workflow
    try {
      const stored = localStorage.getItem(PINNED_WORKFLOW_KEY)
      isPinned.value = !!stored && stored === workflowName.value
    } catch {
      isPinned.value = false
    }
  } catch (e: any) {
    error.value = String(e?.message || e)
  } finally {
    loading.value = false
  }
}

async function openAddModal() {
  showAddModal.value = true
  selectedItemIds.value = new Set()

  try {
    const navResponse = await client.getNavigation({})
    // Get ALL navigation items (not just those not in workflow)
    const allItems = navResponse.items || []

    availableItems.value = allItems.map((item: any) => {
      const title = item.title || item.tableTitle || item.tableName || String(item.id)
      const isInThisWorkflow = item.workflowId === workflowId.value

      // Pre-check items that are already in this workflow
      if (isInThisWorkflow) {
        selectedItemIds.value.add(item.id)
      }

      return {
        id: item.id,
        title,
        tableName: item.tableName,
        dashboardName: item.dashboardName,
        dashboardId: item.dashboardId,
        icon: item.icon || 'DatabaseIcon',
        workflowId: item.workflowId
      }
    })
  } catch (e: any) {
    error.value = String(e?.message || e)
    showAddModal.value = false
  }
}

function toggleItemSelection(itemId: number) {
  if (selectedItemIds.value.has(itemId)) {
    selectedItemIds.value.delete(itemId)
  } else {
    selectedItemIds.value.add(itemId)
  }
}

async function addSelectedItems() {
  addingItems.value = true
  error.value = null

  try {
    // Get current items in this workflow
    const currentItemIds = new Set(items.value.map(item => item.id))

    // Items to add: selected but not currently in workflow
    const itemsToAdd = Array.from(selectedItemIds.value).filter(id => !currentItemIds.has(id))

    // Items to remove: currently in workflow but not selected
    const itemsToRemove = Array.from(currentItemIds).filter(id => !selectedItemIds.value.has(id))

    // Update items to add (set workflow_id)
    const addPromises = itemsToAdd.map(itemId => {
      return client.editItem({
        id: String(itemId),
        pageId: 'table_navigation',
        additionalFields: {
          workflow_id: String(workflowId.value)
        }
      })
    })

    // Update items to remove (clear workflow_id to NULL)
    const removePromises = itemsToRemove.map(itemId => {
      return client.editItem({
        id: String(itemId),
        pageId: 'table_navigation',
        additionalFields: {
          workflow_id: ''
        }
      })
    })

    await Promise.all([...addPromises, ...removePromises])

    // Close modal and reload workflow
    showAddModal.value = false
    await load()
  } catch (e: any) {
    error.value = String(e?.message || e)
  } finally {
    addingItems.value = false
  }
}

function closeModal() {
  showAddModal.value = false
  selectedItemIds.value = new Set()
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
    // Notify App.vue to refresh pinned workflow header
    window.dispatchEvent(new CustomEvent('pinned-workflow-changed'))
  } catch {
    // ignore storage errors
  }
}

onMounted(load)
</script>

<template>
  <Section :title="workflowName">
    <template #toolbar>
      <div class="workflow-actions">
        <button
          v-if="workflowId"
          @click="openAddModal"
          class="btn btn-primary add-navigation-link-btn"
        >
          Change Navigation Links
        </button>
        <button
          v-if="workflowId"
          @click="togglePin"
          :class="['btn', 'btn-outline-primary', 'pin-workflow-btn', { 'bad': isPinned }]"
        >
          {{ isPinned ? 'Stop Workflow' : 'Start Workflow' }}
        </button>
      </div>
    </template>
    <div v-if="loading">Loading…</div>
    <div v-else>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-else>
        <div v-if="items.length === 0" class="workflow-empty-state">
          <div class="empty-state-icon">
            <HugeiconsIcon :icon="Hugeicons[workflowIcon] || DatabaseIcon" />
          </div>
          <div class="empty-state-title">No navigation links yet</div>
          <div class="empty-state-description">
            Get started by adding navigation links to this workflow. Click "Add Navigation Link" above to begin.
          </div>
        </div>
        <div v-else class="workflow-items">
          <div
            v-for="item in items"
            :key="item.id"
            class="workflow-item"
            @click="router.push(item.path)"
          >
            <div class="workflow-item-icon">
              <HugeiconsIcon :icon="Hugeicons[item.icon] || DatabaseIcon" />
            </div>
            <div class="workflow-item-content">
              <div class="workflow-item-title">{{ item.title }}</div>
              <div class="workflow-item-subtitle">
                {{ item.dashboardId > 0 ? 'Dashboard' : 'Table' }}
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </Section>

  <!-- Change Navigation Links Modal -->
  <div v-if="showAddModal" class="modal-overlay" @click.self="closeModal">
    <div class="modal-content">
      <div class="modal-header">
        <h2>Change Navigation Links for {{ workflowName }}</h2>
        <button class="modal-close" @click="closeModal" aria-label="Close">×</button>
      </div>
      <div class="modal-body">
        <div v-if="availableItems.length === 0" class="no-items-message">
          No navigation links available.
        </div>
        <div v-else class="items-list">
          <div
            v-for="item in availableItems"
            :key="item.id"
            class="item-option"
            :class="{ selected: selectedItemIds.has(item.id) }"
            @click="toggleItemSelection(item.id)"
          >
            <div class="item-checkbox">
              <input
                type="checkbox"
                :checked="selectedItemIds.has(item.id)"
                @change="toggleItemSelection(item.id)"
                @click.stop
              />
            </div>
            <div class="item-icon">
              <HugeiconsIcon :icon="Hugeicons[item.icon] || DatabaseIcon" />
            </div>
            <div class="item-info">
              <div class="item-title">{{ item.title }}</div>
              <div class="item-subtitle">
                {{ item.dashboardId > 0 ? 'Dashboard' : 'Table' }}
                <span v-if="item.workflowId > 0 && item.workflowId !== workflowId" class="workflow-badge">In another workflow</span>
              </div>
            </div>
          </div>
        </div>
      </div>
      <div class="modal-footer">
        <button class="btn btn-secondary" @click="closeModal" :disabled="addingItems">
          Cancel
        </button>
        <button
          class="btn btn-primary"
          @click="addSelectedItems"
          :disabled="addingItems"
        >
          {{ addingItems ? 'Saving...' : 'Save Changes' }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.workflow-items {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
  gap: 16px;
  margin-top: 20px;
}

.workflow-item {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 16px;
  background: #f8f9fa;
  border: 1px solid #e9ecef;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s ease;
}

.workflow-item:hover {
  background: #e9ecef;
  border-color: #007bff;
  transform: translateY(-2px);
  box-shadow: 0 4px 8px rgba(0, 123, 255, 0.1);
}

.workflow-item-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 48px;
  height: 48px;
  background: white;
  border-radius: 8px;
  color: #007bff;
  flex-shrink: 0;
}

.workflow-item-icon :deep(svg) {
  width: 24px;
  height: 24px;
}

.workflow-item-content {
  flex: 1;
  min-width: 0;
}

.workflow-item-title {
  font-size: 16px;
  font-weight: 600;
  color: #212529;
  margin-bottom: 4px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.workflow-item-subtitle {
  font-size: 14px;
  color: #6c757d;
}

.workflow-empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 60px 20px;
  text-align: center;
  margin-top: 20px;
}

.empty-state-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 80px;
  height: 80px;
  background: #f8f9fa;
  border-radius: 50%;
  color: #6c757d;
  margin-bottom: 24px;
}

.empty-state-icon :deep(svg) {
  width: 40px;
  height: 40px;
}

.empty-state-title {
  font-size: 20px;
  font-weight: 600;
  color: #212529;
  margin-bottom: 8px;
}

.empty-state-description {
  font-size: 14px;
  color: #6c757d;
  max-width: 400px;
  line-height: 1.5;
}

.workflow-actions {
  display: flex;
  gap: 10px;
  align-items: center;
}

.pin-workflow-btn {
  padding: 8px 12px;
  font-size: 13px;
}

.add-navigation-link-btn {
  padding: 8px 16px;
  background: #007bff;
  color: white;
  text-decoration: none;
  border-radius: 4px;
  font-size: 14px;
  transition: background-color 0.2s;
  border: none;
  cursor: pointer;
}

.add-navigation-link-btn:hover {
  background: #0056b3;
  color: white;
  text-decoration: none;
}

.error {
  background: #f8d7da;
  color: #721c24;
  padding: 0.75rem;
  border: 1px solid #f5c6cb;
  border-radius: 4px;
}

/* Modal Styles */
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  padding: 20px;
}

.modal-content {
  background: white;
  border-radius: 8px;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.15);
  max-width: 600px;
  width: 100%;
  max-height: 80vh;
  display: flex;
  flex-direction: column;
}

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20px;
  border-bottom: 1px solid #e9ecef;
}

.modal-header h2 {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
  color: #212529;
}

.modal-close {
  background: none;
  border: none;
  font-size: 28px;
  color: #6c757d;
  cursor: pointer;
  padding: 0;
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
  transition: background-color 0.2s;
}

.modal-close:hover {
  background: #f8f9fa;
  color: #212529;
}

.modal-body {
  padding: 20px;
  overflow-y: auto;
  flex: 1;
}

.no-items-message {
  text-align: center;
  padding: 40px 20px;
  color: #6c757d;
}

.items-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.item-option {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  border: 2px solid #e9ecef;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s ease;
}

.item-option:hover {
  border-color: #007bff;
  background: #f8f9ff;
}

.item-option.selected {
  border-color: #007bff;
  background: #e7f3ff;
}

.item-checkbox {
  flex-shrink: 0;
}

.item-checkbox input[type="checkbox"] {
  width: 18px;
  height: 18px;
  cursor: pointer;
}

.item-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  background: #f8f9fa;
  border-radius: 6px;
  color: #007bff;
  flex-shrink: 0;
}

.item-icon :deep(svg) {
  width: 20px;
  height: 20px;
}

.item-info {
  flex: 1;
  min-width: 0;
}

.item-title {
  font-size: 15px;
  font-weight: 600;
  color: #212529;
  margin-bottom: 4px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.item-subtitle {
  font-size: 13px;
  color: #6c757d;
  display: flex;
  align-items: center;
  gap: 8px;
}

.workflow-badge {
  background: #fff3cd;
  color: #856404;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 500;
}

.modal-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 12px;
  padding: 20px;
  border-top: 1px solid #e9ecef;
}

.modal-footer .btn {
  padding: 8px 16px;
  border-radius: 4px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
  border: none;
}

.modal-footer .btn-secondary {
  background: #6c757d;
  color: white;
}

.modal-footer .btn-secondary:hover:not(:disabled) {
  background: #5a6268;
}

.modal-footer .btn-primary {
  background: #007bff;
  color: white;
}

.modal-footer .btn-primary:hover:not(:disabled) {
  background: #0056b3;
}

.modal-footer .btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.error {
  background: #f8d7da;
  color: #721c24;
  padding: 0.75rem;
  border: 1px solid #f5c6cb;
  border-radius: 4px;
}
</style>
