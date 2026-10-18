<script setup lang="ts">
import { computed } from 'vue'
import type { Field } from '../gen/sickrock_pb'
import { HugeiconsIcon } from '@hugeicons/vue'
import { DatabaseIcon, Link01Icon } from '@hugeicons/core-free-icons'
import Table from '../components/TableComponent.vue'
import Section from 'picocrank/vue/components/Section.vue'
import { rowInsertActionLabel } from '../utils/rowInsertLabel'
import type { IconData } from '../utils/hugeicons'

export type RelatedTableEntry = {
  tableName: string
  columnName: string
  rows: any[]
  title: string
  direction: 'referencing' | 'referenced'
  filterColumn: string
  filterValue: string
  relationName: string
  constraintName: string
  displayName: string
  pullThroughExtraFields?: Field[]
  /** Shown inside the item's main section instead of as its own section. */
  absorbIntoItem?: boolean
  absorbType?: string
}

const props = defineProps<{
  table: RelatedTableEntry
  parentTableName: string
  parentRowId: string
  parentRowLabel: string
  loadingRelated: boolean
  relatedRowName: string
  canEditRelation: boolean
  /** Render table without an outer Section (for tab panel content). */
  embedInTabs?: boolean
  /** Icon of the related table configuration, for the section title (defaults to DatabaseIcon). */
  icon?: IconData
}>()

const relatedInsertRoute = computed(() => ({
  path: `/table/${props.table.tableName}/insert-row`,
  query: Object.assign(
    { fromTable: props.parentTableName, fromRowId: props.parentRowId },
    props.table.filterValue ? { [props.table.filterColumn]: props.table.filterValue } : {},
  ),
}))

const embedInTabs = computed(() => props.embedInTabs === true)

const emit = defineEmits<{
  rowsUpdated: []
  rowDeleted: [id: string]
  manageRelation: [constraintName: string, displayName?: string]
}>()
</script>

<template>
  <Section v-if="loadingRelated && !embedInTabs" :title="table.title" :icon="icon ?? DatabaseIcon">
    <p class="loading-related">Loading related rows…</p>
  </Section>
  <p v-else-if="loadingRelated && embedInTabs" class="loading-related">Loading related rows…</p>
  <Table
    v-else
    :title="embedInTabs ? '' : table.title"
    :wrap-in-section="!embedInTabs"
    :table-id="table.tableName"
    :items="table.rows"
    :show-toolbar="true"
    :show-view-edit="false"
    :show-view-create="false"
    :show-export="false"
    :show-structure="false"
    :show-insert="false"
    :show-pagination="false"
    :show-view-switcher="true"
    :related-empty-parent-label="parentRowLabel"
    :insert-route-override="relatedInsertRoute"
    :extra-fields="table.pullThroughExtraFields"
    :section-icon="icon"
    @rows-updated="emit('rowsUpdated')"
    @row-deleted="(id: string) => emit('rowDeleted', id)"
  >
    <template #toolbar-prefix>
      <router-link :to="`/table/${table.tableName}`" class="button inline-icon neutral">
        Open table
      </router-link>
      <router-link
        :to="{ path: `/table/${table.tableName}/export`, query: { where: JSON.stringify(table.filterValue ? { [table.filterColumn]: table.filterValue } : {}) } }"
        class="button inline-icon neutral"
      >
        Export
      </router-link>
      <button
        v-if="table.constraintName && canEditRelation"
        type="button"
        class="inline-icon neutral"
        title="Set a friendly name for this relation"
        @click="emit('manageRelation', table.constraintName, table.displayName)"
      >
        <HugeiconsIcon :icon="Link01Icon" width="1em" height="1em" aria-hidden="true" />
        <span>Manage relation</span>
      </button>
    </template>
    <template #toolbar-suffix>
      <router-link
        :to="relatedInsertRoute"
        class="button inline-icon good"
      >
        {{ rowInsertActionLabel(relatedRowName) }}
      </router-link>
    </template>
  </Table>
</template>
