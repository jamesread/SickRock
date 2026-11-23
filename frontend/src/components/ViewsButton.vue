<script setup lang="ts">
import { ref, computed, onMounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { useTableViewManager, type TableView } from '../composables/useTableViewManager'
import { HugeiconsIcon } from '@hugeicons/vue'
import { Add01Icon, Edit03Icon, Settings01Icon } from '@hugeicons/core-free-icons'
import Dialog from 'picocrank/vue/components/Dialog.vue'
import type { DialogApi } from '../utils/dialog'
import { useDesignMode } from '../composables/useDesignMode'
import { viewTypeLabel } from '../utils/viewTypes'

const { designModeEnabled } = useDesignMode()

const props = withDefaults(defineProps<{
  tableId: string
  showViewCreate?: boolean
  showViewEdit?: boolean
  iconOnly?: boolean
  showStructureLink?: boolean
  externalViews?: TableView[]
  externalSelectedViewId?: number | null
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

const dialogRef = ref<DialogApi | null>(null)

function openViewsDialog() {
  nextTick(() => {
    dialogRef.value?.show()
  })
}

function closeViewsDialog() {
  dialogRef.value?.close()
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

/** Saved views can be edited; "All Columns" (id -1) is built in. */
function isEditableView(view: TableView): boolean {
  return view.id !== -1
}

function editView(view: TableView) {
  if (!isEditableView(view)) return
  // Path-based navigation avoids issues with special characters in route params.
  router.push(`/table/${encodeURIComponent(props.tableId)}/edit-view/${encodeURIComponent(String(view.id))}`)
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

const hasFooterActions = computed(() => allowViewCreate.value || allowStructureLink.value)

const dialogTitle = computed(() => (allowStructureLink.value ? 'Table options' : 'Views'))
</script>

<template>
  <button
    type="button"
    class="button inline-icon neutral"
    :title="allowStructureLink ? 'Views and structure' : 'Manage views'"
    @click="openViewsDialog"
  >
    <HugeiconsIcon :icon="Settings01Icon" width="1em" height="1em" aria-hidden="true" />
    <span v-if="!iconOnly">Views</span>
  </button>

  <Dialog ref="dialogRef" :title="dialogTitle" class="views-picker-dialog">
    <p class="subtle views-picker-help">Choose which saved view to display for this table.</p>

    <div class="table-scroll views-picker-scroll">
      <table class="row-hover views-picker-table">
        <thead>
          <tr>
            <th scope="col">View</th>
            <th scope="col">Type</th>
            <th v-if="allowViewEdit" scope="col" class="views-picker-actions"><span class="visually-hidden">Actions</span></th>
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
            <td class="views-picker-type subtle">{{ viewTypeLabel(view.viewType) }}</td>
            <td v-if="allowViewEdit" class="views-picker-actions">
              <button
                v-if="isEditableView(view)"
                type="button"
                class="button inline-icon neutral small design-only"
                :aria-label="`Edit view ${view.viewName}`"
                title="Edit view"
                @click.stop="editView(view)"
                @keydown.enter.stop
              >
                <HugeiconsIcon :icon="Edit03Icon" width="1em" height="1em" aria-hidden="true" />
                <span>Edit</span>
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <template v-if="hasFooterActions" #actions>
      <button
        v-if="allowViewCreate"
        type="button"
        class="button inline-icon design-only"
        @click="createTableView"
      >
        <HugeiconsIcon :icon="Add01Icon" width="1em" height="1em" aria-hidden="true" />
        <span>Create view</span>
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
    </template>
  </Dialog>
</template>

<style scoped>
.views-picker-help {
  margin: 0 0 0.75rem;
}

.views-picker-scroll {
  max-height: 18rem;
}

.views-picker-table td {
  cursor: pointer;
}

.views-picker-name {
  margin-right: 0.5rem;
}

.views-picker-type {
  white-space: nowrap;
}

.views-picker-actions {
  width: 1%;
  text-align: end;
  white-space: nowrap;
}

.views-picker-actions .button {
  margin: 0;
}

.visually-hidden {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
}

.views-picker-selected td {
  font-weight: 600;
}
</style>
