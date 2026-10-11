<script setup lang="ts">
import { computed } from 'vue'
import Section from 'picocrank/vue/components/Section.vue'
import { Link01Icon } from '@hugeicons/core-free-icons'
import type { createApiClient } from '../stores/api'
import type { ForeignKey } from '../gen/sickrock_pb'
import RelationExtraColumnsEditor from './RelationExtraColumnsEditor.vue'

const props = defineProps<{
  tableId: string
  foreignKeys: ForeignKey[]
  client: ReturnType<typeof createApiClient>
  designModeEnabled: boolean
}>()

const emit = defineEmits<{
  saved: []
}>()

const outgoingForeignKeys = computed(() =>
  props.foreignKeys.filter(fk => {
    const source = (fk.tableTcName?.trim() || fk.tableName).trim()
    return source === props.tableId.trim()
  }),
)

function relationHeading(fk: ForeignKey): string {
  const ref = fk.referencedTableTcName?.trim() || fk.referencedTable
  return `${fk.columnName} → ${ref}`
}
</script>

<template>
  <Section
    v-if="outgoingForeignKeys.length > 0"
    title="Pull-through columns (table view)"
    :icon="Link01Icon"
  >
    <p class="muted">
      Extra columns on <code>/table/{{ tableId }}/</code> and exports. Customize labels under Column display below.
      Enable design mode to edit.
    </p>
    <article
      v-for="fk in outgoingForeignKeys"
      :key="fk.constraintName || fk.columnName"
      class="relation-extra-block"
    >
      <h3 class="relation-extra-block-title">
        <code>{{ relationHeading(fk) }}</code>
      </h3>
      <p v-if="fk.constraintName" class="muted relation-constraint">
        Constraint: <code>{{ fk.constraintName }}</code>
      </p>
      <RelationExtraColumnsEditor
        :source-table-id="tableId"
        :constraint-name="fk.constraintName"
        :pull-through-fields="fk.pullThroughFields ?? []"
        :display-name="fk.displayName"
        :client="client"
        :disabled="!designModeEnabled"
        :id-prefix="`table-pt-${fk.constraintName}`"
        @saved="emit('saved')"
      />
    </article>
  </Section>
</template>

<style scoped>
.relation-extra-block {
  margin-top: 1.25rem;
  padding-top: 1rem;
  border-top: 1px solid var(--border-color, #dee2e6);
}

.relation-extra-block:first-of-type {
  margin-top: 0.5rem;
  padding-top: 0;
  border-top: none;
}

.relation-extra-block-title {
  margin: 0 0 0.35rem;
  font-size: 1rem;
}

.relation-constraint {
  margin: 0 0 0.75rem;
  font-size: 0.9em;
}
</style>
