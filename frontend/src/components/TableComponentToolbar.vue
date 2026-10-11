<script setup lang="ts">
import { HugeiconsIcon } from '@hugeicons/vue'
import {
  Add01Icon,
  AddCircleIcon,
  Download01Icon,
  Settings01Icon,
} from '@hugeicons/core-free-icons'
import ViewsButton from './ViewsButton.vue'
import type { TableView } from '../composables/useTableViewManager'

defineProps<{
  tableId: string
  showViewSwitcher?: boolean
  showViewEdit?: boolean
  showViewCreate?: boolean
  showExport?: boolean
  showStructure?: boolean
  showInsert?: boolean
  showQuickAdd?: boolean
  insertRoute: string | { path: string; query?: Record<string, string> }
  insertButtonText: string
  tableViews: TableView[]
  selectedViewId: number | null
  currentView: TableView | null
  multiSelectEnabled?: boolean
}>()

const emit = defineEmits<{
  'view-selected': [viewId: number]
  'open-quick-add': []
  'toggle-multi-select': []
}>()
</script>

<template>
  <div class="flex-row g1 table-component-toolbar" role="presentation">
  <slot name="toolbar-prefix" />
  <ViewsButton
    v-if="showViewSwitcher || showViewEdit || showViewCreate"
    :table-id="tableId"
    :show-view-create="showViewCreate"
    :show-view-edit="showViewEdit"
    :external-views="tableViews"
    :external-selected-view-id="selectedViewId"
    :external-current-view="currentView"
    @view-selected="emit('view-selected', $event)"
  />
  <button
    type="button"
    class="button inline-icon neutral ss-large"
    :class="{ good: multiSelectEnabled }"
    :aria-pressed="multiSelectEnabled"
    title="Select multiple rows for bulk actions"
    @click="emit('toggle-multi-select')"
  >
    <span>Multi-select</span>
  </button>
  <router-link
    v-if="showExport"
    :to="`/table/${tableId}/export`"
    class="button inline-icon neutral ss-large"
  >
    <HugeiconsIcon :icon="Download01Icon" width="1em" height="1em" aria-hidden="true" />
    <span>Export</span>
  </router-link>
  <router-link
    v-if="showStructure"
    :to="`/table/${tableId}/column-types`"
    class="button inline-icon neutral ss-large design-only"
  >
    <HugeiconsIcon :icon="Settings01Icon" width="1em" height="1em" aria-hidden="true" />
    <span>Structure</span>
  </router-link>
  <div v-if="showInsert" class="insert-button-group">
    <router-link
      :to="insertRoute"
      class="button inline-icon neutral insert-button"
      accesskey="n"
      title="Insert row"
    >
      <HugeiconsIcon :icon="AddCircleIcon" width="1em" height="1em" aria-hidden="true" />
      <span>{{ insertButtonText }}</span>
    </router-link>
    <button
      v-if="showQuickAdd"
      class="button inline-icon good quick-add-button"
      title="Quick Add"
      aria-label="Quick Add"
      @click="emit('open-quick-add')"
    >
      <HugeiconsIcon :icon="Add01Icon" width="1em" height="1em" aria-hidden="true" />
      <span class="quick-add-text" />
    </button>
  </div>
  <slot name="toolbar-suffix" />
  </div>
</template>

<style scoped>
.table-component-toolbar {
  flex-wrap: wrap;
  justify-content: flex-end;
}
</style>
