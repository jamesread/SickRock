<script setup lang="ts">
// A relation absorbed into the item view ("first row"): the first related row's fields shown
// read-only as a subsection of the item's main section.
import { computed } from 'vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { DatabaseIcon, Link01Icon } from '@hugeicons/core-free-icons'
import type { Field } from '../gen/sickrock_pb'
import { fieldDisplayLabel } from '../utils/fieldColumnDisplay'
import type { IconData } from '../utils/hugeicons'

const props = defineProps<{
  title: string
  tableName: string
  /** The first related row (ListItems item), or undefined when there are none. */
  row?: { id?: string; additionalFields?: Record<string, unknown> } | null
  /** The related table's fields (for order and labels); falls back to the row's own keys. */
  fields: Field[]
  /** Columns not worth repeating, e.g. the foreign key pointing back at this item. */
  hiddenColumns: string[]
  /** The related table configuration's icon (defaults to DatabaseIcon). */
  icon?: IconData
  /** Show the "Manage relation" button (design mode, with edit access). */
  canManage?: boolean
  /** Route to the related table filtered to this item's rows, and its link text. */
  viewAllTo: string | { path: string; query?: Record<string, string> }
  viewAllLabel: string
}>()

const emit = defineEmits<{ manage: [] }>()

const HIDDEN = new Set(['id', 'sr_created', 'sr_updated'])

const entries = computed(() => {
  const values = props.row?.additionalFields ?? {}
  const hidden = new Set([...HIDDEN, ...props.hiddenColumns])
  const names = props.fields.length > 0
    ? props.fields.map(field => field.name)
    : Object.keys(values)
  return names
    .filter(name => !hidden.has(name))
    .map(name => {
      const value = values[name]
      const empty = value === null || value === undefined || value === ''
      return {
        name,
        label: fieldDisplayLabel(props.fields.find(field => field.name === name), name),
        value: empty ? '(empty)' : String(value),
        empty,
      }
    })
})
</script>

<template>
  <div class="absorbed-relation">
    <h3 class="absorbed-relation-title">
      <HugeiconsIcon :icon="icon ?? DatabaseIcon" width="1.1em" height="1.1em" aria-hidden="true" />
      <span>{{ title }}</span>
      <router-link v-if="row?.id" :to="`/table/${tableName}/${row.id}`" class="absorbed-relation-link">Open</router-link>
      <button
        v-if="canManage"
        type="button"
        class="inline-icon neutral design-only absorbed-relation-manage"
        title="Set a friendly name or absorb settings for this relation"
        @click="emit('manage')"
      >
        <HugeiconsIcon :icon="Link01Icon" width="1em" height="1em" aria-hidden="true" />
        <span>Manage relation</span>
      </button>
    </h3>
    <p v-if="!row" class="subtle absorbed-relation-empty">No related row.</p>
    <dl v-else class="absorbed-relation-fields">
      <template v-for="entry in entries" :key="entry.name">
        <dt>{{ entry.label }}</dt>
        <dd :class="{ empty: entry.empty }">{{ entry.value }}</dd>
      </template>
    </dl>
    <p class="absorbed-relation-footer">
      <router-link :to="viewAllTo" class="absorbed-relation-link">{{ viewAllLabel }}</router-link>
    </p>
  </div>
</template>

<style scoped>
.absorbed-relation {
  margin-top: 1rem;
}

/* The heading lines up with the section content; the field list lines up with the item's own fields. */
.absorbed-relation-title,
.absorbed-relation-empty,
.absorbed-relation-footer {
  display: flex;
  align-items: baseline;
  gap: 0.75rem;
  margin: 0;
  padding: 0 1rem 0.25rem;
}

.absorbed-relation-title {
  flex-wrap: wrap;
  align-items: center;
  font-size: 1rem;
}

.absorbed-relation-link {
  font-size: 0.9rem;
  font-weight: normal;
}

.absorbed-relation-manage {
  margin-left: auto;
  font-size: 0.85rem;
}

.absorbed-relation-footer {
  padding-top: 0.5rem;
}

/* Same layout as the item's own field list (RowViewFieldEntries .row-fields). */
.absorbed-relation-fields {
  display: grid;
  grid-template-columns: 12rem minmax(0, 1fr);
  column-gap: 1rem;
  margin: 0;
}

.absorbed-relation-fields dt {
  width: 12rem;
  margin: 0;
  font-weight: 600;
}

.absorbed-relation-fields dd {
  margin: 0;
  min-width: 0;
}

.absorbed-relation-fields dd.empty {
  color: var(--muted-text-color, #8a8a8a);
}
</style>
