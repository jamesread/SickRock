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
import RollupButtons from './RollupButtons.vue'
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
  multiSelectEnabled?: boolean
  /** Rows a rollup run covers (those the filters leave); undefined for every row. */
  rollupSourceIds?: string[]
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
  <RollupButtons :table-id="tableId" :source-ids="rollupSourceIds" />
  <!-- picocrank .toolbar-split: together beside the title, separate items in the overflow menu. -->
  <div v-if="showInsert" class="insert-button-group toolbar-split">
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
