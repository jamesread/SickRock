<script setup lang="ts">
import { ref, computed, onMounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { useTableViewManager, type TableView } from '../composables/useTableViewManager'
import { HugeiconsIcon } from '@hugeicons/vue'
import { Add01Icon, Edit03Icon, Settings01Icon, StarIcon } from '@hugeicons/core-free-icons'
import Dialog from 'picocrank/vue/components/Dialog.vue'
import ContextMenu from 'picocrank/vue/components/ContextMenu.vue'
import { showNotificationPopup } from 'picocrank/vue/composables/useNotificationPopups.js'
import { createApiClient } from '../stores/api'
import { useTableAccess } from '../composables/useTableAccess'
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
  loadDefaults()
  nextTick(() => {
    dialogRef.value?.show()
  })
}

// Defaults: the caller's own (table_view_user_defaults) and the table's, for everyone (is_default).
// Read fresh when the dialog opens, so the tags are right after a change from elsewhere.
const client = createApiClient()
const { access } = useTableAccess(computed(() => props.tableId))
const myDefaultId = ref(0)
const tableDefaultId = ref(0)
const defaultMenuOpen = ref(false)
const defaultMenuX = ref(0)
const defaultMenuY = ref(0)
const defaultMenuView = ref<TableView | null>(null)

async function loadDefaults() {
  try {
    const res = await client.getTableViews({ tableName: props.tableId })
    myDefaultId.value = res.userDefaultViewId ?? 0
    tableDefaultId.value = res.views.find(view => view.isDefault)?.id ?? 0
  } catch (error) {
    console.warn('Could not load default views:', error)
  }
}

/** "Default" marks the view everyone opens on; with none set, the built-in All Columns. */
function isTableDefault(view: TableView): boolean {
  return view.id === -1 ? tableDefaultId.value === 0 : view.id === tableDefaultId.value
}

const defaultMenuItems = computed(() => {
  const view = defaultMenuView.value
  if (!view) return []
  const mine = view.id === myDefaultId.value
  const everyone = view.id === tableDefaultId.value
  return [
    mine
      ? { id: 'mine-clear', label: 'Stop opening on this view' }
      : { id: 'mine', label: 'Open on this view (just me)' },
    ...(access.value.canEdit
      ? [everyone
          ? { id: 'everyone-clear', label: 'Remove as default for everyone' }
          : { id: 'everyone', label: 'Make default for everyone' }]
      : []),
  ]
})

function openDefaultMenu(view: TableView, event: MouseEvent) {
  const rect = (event.currentTarget as HTMLElement).getBoundingClientRect()
  defaultMenuView.value = view
  defaultMenuX.value = rect.left
  defaultMenuY.value = rect.bottom
  defaultMenuOpen.value = true
}

async function onDefaultMenuSelect(item: { id?: string }) {
  const view = defaultMenuView.value
  if (!view) return
  try {
    if (item.id === 'mine' || item.id === 'mine-clear') {
      const viewId = item.id === 'mine' ? view.id : 0
      await client.setUserDefaultTableView({ tableName: props.tableId, viewId })
      showNotificationPopup({ message: viewId ? `This table now opens on ${view.viewName} for you` : 'Your default view is cleared', class: 'good' })
    } else if (item.id === 'everyone' || item.id === 'everyone-clear') {
      const viewId = item.id === 'everyone' ? view.id : 0
      await client.setTableDefaultView({ tableName: props.tableId, viewId })
      showNotificationPopup({ message: viewId ? `${view.viewName} is now the default view` : 'The default view is cleared', class: 'good' })
    }
    await loadDefaults()
  } catch (error) {
    showNotificationPopup({ message: `Could not change the default view: ${error instanceof Error ? error.message : String(error)}`, class: 'bad' })
  }
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
            <th scope="col" class="views-picker-actions"><span class="visually-hidden">Actions</span></th>
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
              <span v-if="isTableDefault(view)" class="tag note" title="Everyone opens the table on this view, unless they chose their own">Default</span>
              <span v-if="view.id > 0 && view.id === myDefaultId" class="tag good" title="You open the table on this view">Yours</span>
            </td>
            <td class="views-picker-type subtle">{{ viewTypeLabel(view.viewType) }}</td>
            <td class="views-picker-actions">
              <button
                v-if="isEditableView(view)"
                type="button"
                class="button inline-icon neutral small"
                :aria-label="`Default view options for ${view.viewName}`"
                title="Open the table on this view"
                aria-haspopup="menu"
                @click.stop="openDefaultMenu(view, $event)"
                @keydown.enter.stop
              >
                <HugeiconsIcon :icon="StarIcon" width="1em" height="1em" aria-hidden="true" />
                <span>Default…</span>
              </button>
              <button
                v-if="allowViewEdit && isEditableView(view)"
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

    <ContextMenu
      v-model:open="defaultMenuOpen"
      aria-label="Default view"
      :items="defaultMenuItems"
      :client-x="defaultMenuX"
      :client-y="defaultMenuY"
      @select="onDefaultMenuSelect"
    />

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

.views-picker-actions .button + .button {
  margin-inline-start: 0.35rem;
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
