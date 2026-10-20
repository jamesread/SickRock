<script setup lang="ts">
import { ref, computed, onMounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { useTableViewManager, type TableView } from '../composables/useTableViewManager'
import { HugeiconsIcon } from '@hugeicons/vue'
import { Add01Icon, Edit03Icon, Settings01Icon, Cancel01Icon } from '@hugeicons/core-free-icons'
import { useDesignMode } from '../composables/useDesignMode'

const { designModeEnabled } = useDesignMode()

const props = withDefaults(defineProps<{
  tableId: string
  showViewCreate?: boolean
  showViewEdit?: boolean
  iconOnly?: boolean
  showStructureLink?: boolean
  externalViews?: TableView[]
  externalSelectedViewId?: number | null
  externalCurrentView?: TableView | null
}>(), {
  showViewCreate: true,
  showViewEdit: true,
  iconOnly: false,
  showStructureLink: false,
})

const emit = defineEmits<{
  'view-changed': [viewType: string, viewId?: number]
  'view-selected': [viewId: number]
}>()

const router = useRouter()

const useExternalState = computed(() => !!props.externalViews)

const internalManager = useTableViewManager(props.tableId, (view: TableView | null) => {
  if (!useExternalState.value) {
    const viewType = view?.viewType || 'table'
    emit('view-changed', viewType, view?.id ?? -1)
  }
})

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
        columns: [],
      },
      ...props.externalViews,
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
  },
})

const currentView = computed(() => {
  if (useExternalState.value) {
    return props.externalCurrentView ?? null
  }
  return internalManager.currentView.value
})

const dialogRef = ref<HTMLDialogElement | null>(null)

function openViewsDialog() {
  nextTick(() => {
    dialogRef.value?.showModal()
  })
}

function closeViewsDialog() {
  dialogRef.value?.close()
}

function onDialogBackdropClick(event: MouseEvent) {
  if (event.target === dialogRef.value) {
    closeViewsDialog()
  }
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

const allowViewCreate = computed(() => props.showViewCreate && designModeEnabled.value)
const allowViewEdit = computed(() => props.showViewEdit && designModeEnabled.value)
const allowStructureLink = computed(() => props.showStructureLink && designModeEnabled.value)

const hasFooterActions = computed(
  () => allowViewCreate.value || allowViewEdit.value || allowStructureLink.value,
)

const dialogTitle = computed(() => (allowStructureLink.value ? 'Table options' : 'Views'))
</script>

<template>
  <button
    type="button"
    class="button inline-icon neutral ss-large"
    :title="allowStructureLink ? 'Views and structure' : 'Manage views'"
    @click="openViewsDialog"
  >
    <HugeiconsIcon :icon="Settings01Icon" width="1em" height="1em" aria-hidden="true" />
    <span v-if="!iconOnly">Views</span>
  </button>

  <dialog
    ref="dialogRef"
    class="dialog views-picker-dialog"
    :aria-labelledby="`views-picker-title-${tableId}`"
    @click="onDialogBackdropClick"
  >
    <div class="views-picker-panel" @click.stop>
      <div class="flex-row views-picker-header">
        <h2 :id="`views-picker-title-${tableId}`">{{ dialogTitle }}</h2>
        <button
          type="button"
          class="button inline-icon neutral"
          aria-label="Close"
          @click="closeViewsDialog"
        >
          <HugeiconsIcon :icon="Cancel01Icon" width="1em" height="1em" aria-hidden="true" />
        </button>
      </div>

      <p class="subtle views-picker-help">Choose which saved view to display for this table.</p>

      <div class="table-scroll views-picker-scroll">
        <table class="row-hover views-picker-table">
          <thead>
            <tr>
              <th scope="col">View</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="view in viewsForDialog"
              :key="view.id"
              tabindex="0"
              :aria-selected="selectedViewId === view.id"
              :class="{ 'views-picker-selected': selectedViewId === view.id }"
              @click="selectView(view.id)"
              @keydown.enter.prevent="selectView(view.id)"
            >
              <td>
                <span class="views-picker-name">{{ view.viewName }}</span>
                <span v-if="view.isDefault" class="tag note">Default</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div v-if="hasFooterActions" role="toolbar" class="flex-row g1 views-picker-actions">
        <button
          v-if="allowViewCreate"
          type="button"
          class="button inline-icon design-only"
          @click="createTableView"
        >
          <HugeiconsIcon :icon="Add01Icon" width="1em" height="1em" aria-hidden="true" />
          <span>Create view</span>
        </button>
        <button
          v-if="allowViewEdit && currentView && currentView.id !== -1"
          type="button"
          class="button inline-icon neutral design-only"
          @click="editTableView"
        >
          <HugeiconsIcon :icon="Edit03Icon" width="1em" height="1em" aria-hidden="true" />
          <span>Edit view</span>
        </button>
        <router-link
          v-if="allowStructureLink"
          :to="`/table/${tableId}/column-types`"
          class="button inline-icon neutral design-only"
          @click="closeViewsDialog"
        >
          <HugeiconsIcon :icon="Settings01Icon" width="1em" height="1em" aria-hidden="true" />
          <span>Structure</span>
        </router-link>
      </div>
    </div>
  </dialog>
</template>

<style scoped>
.views-picker-dialog {
  max-width: 32rem;
  width: min(32rem, 92vw);
}

.views-picker-panel h2 {
  margin: 0;
  font-size: 1.25rem;
}

.views-picker-header {
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
  margin-bottom: 0.35rem;
}

.views-picker-help {
  margin: 0 0 0.75rem;
}

.views-picker-scroll {
  max-height: 18rem;
  margin-bottom: 1rem;
}

.views-picker-table td {
  cursor: pointer;
}

.views-picker-name {
  margin-right: 0.5rem;
}

.views-picker-selected td {
  font-weight: 600;
}

.views-picker-actions {
  flex-wrap: wrap;
  justify-content: flex-end;
}
</style>
