<script setup lang="ts">
import { HugeiconsIcon } from '@hugeicons/vue'
import {
  Add01Icon,
  AddCircleIcon,
  BarCode01Icon,
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
  /** Shows the "Bulk entry" link (tables with bulk_entry_json set). */
  showBulkEntry?: boolean
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
    class="button inline-icon neutral"
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
    class="button inline-icon neutral"
  >
    <HugeiconsIcon :icon="Download01Icon" width="1em" height="1em" aria-hidden="true" />
    <span>Export</span>
  </router-link>
  <router-link
    v-if="showStructure"
    :to="`/table/${tableId}/column-types`"
    class="button inline-icon neutral design-only"
  >
    <HugeiconsIcon :icon="Settings01Icon" width="1em" height="1em" aria-hidden="true" />
    <span>Structure</span>
  </router-link>
  <router-link
    v-if="showBulkEntry"
    :to="`/table/${tableId}/bulk-entry`"
    class="button inline-icon neutral"
  >
    <HugeiconsIcon :icon="BarCode01Icon" width="1em" height="1em" aria-hidden="true" />
    <span>Bulk entry</span>
  </router-link>
  <div v-if="showInsert" class="insert-button-group">
    <!-- Wide screens show "Create <row>"; narrow screens fall back to the compact quick-add button. -->
    <router-link
      :to="insertRoute"
      class="button inline-icon good insert-button"
      :class="{ 'has-quick-add': showQuickAdd, 'hide-narrow': showQuickAdd }"
      accesskey="n"
      title="Insert row"
    >
      <HugeiconsIcon :icon="AddCircleIcon" width="1em" height="1em" aria-hidden="true" />
      <span>{{ insertButtonText }}</span>
    </router-link>
    <button
      v-if="showQuickAdd"
      type="button"
      class="button inline-icon good quick-add-button show-narrow"
      title="Quick Add"
      aria-label="Quick Add"
      @click="emit('open-quick-add')"
    >
      <HugeiconsIcon :icon="Add01Icon" width="1em" height="1em" aria-hidden="true" />
    </button>
  </div>
  <slot name="toolbar-suffix" />
  </div>
</template>

<style scoped>
/*
 * Unwrapped so the surrounding toolbar (a picocrank Section's) sees each
 * control: it lays them out and folds them into its overflow menu on narrow
 * screens.
 */
.table-component-toolbar {
  display: contents;
}

/*
 * Wide/narrow swap of "Create <row>" and Quick Add uses picocrank's .hide-narrow and
 * .show-narrow (!important), so menu layout rules cannot unhide either.
 */
</style>

<!--
  Unscoped: the overflow menu is picocrank's element. (In a scoped block, ":global(x) y"
  compiles to just "x", which would unwrap the whole menu.)
-->
<style>
/* In the overflow menu, Create and Quick Add are separate full-width items. */
[role="toolbar"].toolbar-overflow-menu .insert-button-group {
  display: contents;
}
</style>
