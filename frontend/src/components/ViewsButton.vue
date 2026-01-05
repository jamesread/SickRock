<script setup lang="ts">
import { ref, computed, onMounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { useTableViewManager, type TableView } from '../composables/useTableViewManager'
import { HugeiconsIcon } from '@hugeicons/vue'
import { Add01Icon, Edit03Icon, Settings01Icon } from '@hugeicons/core-free-icons'

const props = withDefaults(defineProps<{
  tableId: string
  showViewCreate?: boolean
  showViewEdit?: boolean
  // Optional: use external view state (for TableComponent)
  externalViews?: TableView[]
  externalSelectedViewId?: number | null
  externalCurrentView?: TableView | null
}>(), {
  showViewCreate: true,
  showViewEdit: true
})

const emit = defineEmits<{
  'view-changed': [viewType: string]
  'view-selected': [viewId: number]
}>()

const router = useRouter()

// Use external state if provided, otherwise use internal state
const useExternalState = computed(() => !!props.externalViews)

const internalManager = useTableViewManager(props.tableId, (view: TableView | null) => {
  if (!useExternalState.value) {
    // Only handle view changes if using internal state
    const viewType = view?.viewType || 'table'
    emit('view-changed', viewType)
  }
})

// Use external or internal state
const viewsForDialog = computed(() => {
  if (useExternalState.value && props.externalViews) {
    const defaultView = props.externalViews.find(v => v.isDefault)
    return [
      {
        id: -1,
        tableName: props.tableId,
        viewName: 'All Columns',
        isDefault: !props.externalViews.length || !!defaultView,
        viewType: 'table',
        columns: []
      },
      ...props.externalViews
    ]
  }
  return internalManager.viewsForDialog.value
})

const selectedViewId = computed({
  get: () => useExternalState.value ? (props.externalSelectedViewId ?? null) : internalManager.selectedViewId.value,
  set: (val) => {
    if (useExternalState.value) {
      emit('view-selected', val ?? -1)
    } else {
      internalManager.selectedViewId.value = val
    }
  }
})

const currentView = computed(() => {
  if (useExternalState.value) {
    return props.externalCurrentView ?? null
  }
  return internalManager.currentView.value
})

const showViewsDialog = ref(false)
const dialogRef = ref<HTMLDialogElement | null>(null)

function openViewsDialog() {
  showViewsDialog.value = true
  nextTick(() => {
    dialogRef.value?.showModal()
  })
}

function closeViewsDialog() {
  dialogRef.value?.close()
  showViewsDialog.value = false
}

function selectView(viewId: number) {
  if (useExternalState.value) {
    emit('view-selected', viewId)
  } else {
    internalManager.selectView(viewId)
  }
  closeViewsDialog()
}

function createTableView() {
  internalManager.createTableView()
  closeViewsDialog()
}

function editTableView() {
  if (useExternalState.value) {
    // When using external state, we need to navigate directly since internalManager doesn't have the current view
    if (currentView.value && currentView.value.id !== -1) {
      const viewId = encodeURIComponent(currentView.value.id.toString())
      const tableName = encodeURIComponent(props.tableId)
      router.push(`/table/${tableName}/edit-view/${viewId}`)
    }
  } else {
    internalManager.editTableView()
  }
  closeViewsDialog()
}

onMounted(() => {
  if (!useExternalState.value) {
    internalManager.loadTableViews()
  }
})
</script>

<template>
  <button
    @click="openViewsDialog"
    class="button neutral ss-large"
    title="Manage views"
  >
    <HugeiconsIcon :icon="Settings01Icon" />
    Views
  </button>

  <!-- Views Dialog - Teleported to body to render as overlay -->
  <Teleport to="body">
    <dialog
      v-if="showViewsDialog"
      ref="dialogRef"
      class="views-dialog"
      @click="(e) => { if (e.target === e.currentTarget) closeViewsDialog() }"
      @close="showViewsDialog = false"
    >
      <div class="modal-content views-modal" @click.stop>
        <div class="modal-header">
          <div class="modal-header-left">
            <h3>Views</h3>
          </div>
          <button @click="closeViewsDialog" class="button neutral" title="Close">
            ✕
          </button>
        </div>
        <div class="modal-body views-body">
          <div class="views-list">
            <div
              v-for="view in viewsForDialog"
              :key="view.id"
              class="view-row"
              :class="{ active: selectedViewId === view.id }"
              @click="selectView(view.id)"
            >
              <div class="view-name">{{ view.viewName }}</div>
              <div class="view-meta" v-if="view.isDefault">Default</div>
            </div>
          </div>
          <div class="views-actions">
            <button
              v-if="showViewCreate"
              class="button primary"
              @click="() => { closeViewsDialog(); createTableView(); }"
            >
              <HugeiconsIcon :icon="Add01Icon" />
              Create View
            </button>
            <button
              v-if="showViewEdit && currentView && currentView.id !== -1"
              class="button neutral"
              @click="() => { closeViewsDialog(); editTableView(); }"
            >
              <HugeiconsIcon :icon="Edit03Icon" />
              Edit View
            </button>
          </div>
        </div>
      </div>
    </dialog>
  </Teleport>
</template>

<style>
/* Views dialog styles - shared between TableComponent and CalendarComponent */
.views-dialog {
  border: none;
  border-radius: 8px;
  padding: 1em;
  background: #fff;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.3);
  z-index: 3000;
  max-width: 520px;
  width: 90%;
}

h3 {
  margin: 0;
  padding: 0;
}

.views-dialog::backdrop {
  background: rgba(0, 0, 0, 0.5);
}

.views-modal {
  width: 100%;
  background: white;
}

.views-body {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.views-list {
  border: 1px solid #e9ecef;
  border-radius: 6px;
  max-height: 320px;
  overflow-y: auto;
}

.view-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px;
  cursor: pointer;
  border-bottom: 1px solid #f1f3f5;
  transition: background 0.15s;
}

.view-row:last-child {
  border-bottom: none;
}

.view-row:hover {
  background: #f8f9fb;
}

.view-row.active {
  background: #e7f3ff;
  border-left: 3px solid #007bff;
}

.view-name {
  font-weight: 600;
  color: #212529;
}

.view-meta {
  font-size: 12px;
  color: #6c757d;
}

.views-actions {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
}
</style>
