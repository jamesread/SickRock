<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import Section from 'picocrank/vue/components/Section.vue'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { GridIcon, PlusSignIcon, Delete01Icon } from '@hugeicons/core-free-icons'
import type { createApiClient } from '../stores/api'

type EditorGroup = {
  key: string
  title: string
  columnNames: string[]
}

const props = withDefaults(defineProps<{
  tableId: string
  columnNames: string[]
  client: ReturnType<typeof createApiClient>
  /** When true, omit Section wrapper (for dialogs). */
  embedded?: boolean
}>(), {
  embedded: false,
})

const emit = defineEmits<{
  saved: []
}>()

const loading = ref(true)
const saving = ref(false)
const error = ref<string | null>(null)
const saveMessage = ref<string | null>(null)
const groups = ref<EditorGroup[]>([])

let nextKey = 1
function newKey() {
  return `group-${nextKey++}`
}

const assignedColumns = computed(() => {
  const set = new Set<string>()
  for (const g of groups.value) {
    for (const c of g.columnNames) {
      set.add(c)
    }
  }
  return set
})

const unassignedColumns = computed(() =>
  props.columnNames.filter(name => !assignedColumns.value.has(name)),
)

async function load() {
  loading.value = true
  error.value = null
  try {
    const structure = await props.client.getTableStructure({ pageId: props.tableId })
    const raw = structure.itemFieldGroups?.groups ?? []
    const sorted = [...raw].sort((a, b) => (a.ordinal ?? 0) - (b.ordinal ?? 0))
    groups.value = sorted.map(g => ({
      key: newKey(),
      title: g.title ?? '',
      columnNames: [...(g.columnNames ?? [])],
    }))
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void load()
})

watch(
  () => props.columnNames.join('\0'),
  () => {
    for (const g of groups.value) {
      g.columnNames = g.columnNames.filter(c => props.columnNames.includes(c))
    }
  },
)

function addGroup() {
  groups.value.push({ key: newKey(), title: '', columnNames: [] })
}

function removeGroup(index: number) {
  groups.value.splice(index, 1)
}

function addColumnToGroup(groupIndex: number, columnName: string) {
  if (!columnName || assignedColumns.value.has(columnName)) return
  groups.value[groupIndex].columnNames.push(columnName)
}

function removeColumnFromGroup(groupIndex: number, columnIndex: number) {
  groups.value[groupIndex].columnNames.splice(columnIndex, 1)
}

function moveGroup(groupIndex: number, direction: 'up' | 'down') {
  const target = direction === 'up' ? groupIndex - 1 : groupIndex + 1
  if (target < 0 || target >= groups.value.length) {
    return
  }
  const list = [...groups.value]
  const [moved] = list.splice(groupIndex, 1)
  list.splice(target, 0, moved)
  groups.value = list
}

function moveColumnInGroup(groupIndex: number, columnIndex: number, direction: 'up' | 'down') {
  const cols = groups.value[groupIndex]?.columnNames
  if (!cols) {
    return
  }
  const target = direction === 'up' ? columnIndex - 1 : columnIndex + 1
  if (target < 0 || target >= cols.length) {
    return
  }
  const next = [...cols]
  const [moved] = next.splice(columnIndex, 1)
  next.splice(target, 0, moved)
  groups.value[groupIndex].columnNames = next
}

async function saveGroups() {
  saving.value = true
  error.value = null
  saveMessage.value = null
  try {
    for (let i = 0; i < groups.value.length; i++) {
      const g = groups.value[i]
      if (!g.title.trim()) {
        throw new Error(`Group ${i + 1} needs a title`)
      }
      if (g.columnNames.length === 0) {
        throw new Error(`Group "${g.title}" needs at least one column`)
      }
    }
    const res = await props.client.setItemFieldGroups({
      tableConfiguration: props.tableId,
      groups: groups.value.map((g, i) => ({
        ordinal: i + 1,
        title: g.title.trim(),
        columnNames: [...g.columnNames],
      })),
    })
    if (!res.success) {
      throw new Error(res.message || 'Save failed')
    }
    saveMessage.value = res.message || 'Field groups saved'
    await load()
    emit('saved')
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    saving.value = false
  }
}

async function resetGroups() {
  if (!confirm('Clear all field groups? Item view will show a single flat field list again.')) {
    return
  }
  saving.value = true
  error.value = null
  try {
    const res = await props.client.setItemFieldGroups({
      tableConfiguration: props.tableId,
      groups: [],
    })
    if (!res.success) {
      throw new Error(res.message || 'Reset failed')
    }
    groups.value = []
    saveMessage.value = 'Field groups cleared'
    emit('saved')
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <component :is="embedded ? 'div' : Section" v-bind="embedded ? {} : { title: 'Field grouping', icon: GridIcon }">
    <NotificationBlock v-if="error" variant="bad" :message="error" />
    <NotificationBlock v-else-if="saveMessage" variant="good" :message="saveMessage" />

    <p class="field-grouping-help">
      Organize columns into titled groups on each item row page. Use the arrows to reorder groups and fields.
      Columns not placed in a group still appear at the bottom.
    </p>

    <p v-if="loading">Loading field groups…</p>
    <template v-else-if="columnNames.length">
      <div class="field-grouping-layout">
        <div class="field-groups-panel">
          <div role="toolbar" class="groups-toolbar">
            <button type="button" class="button inline-icon neutral design-only" @click="addGroup">
              <HugeiconsIcon :icon="PlusSignIcon" width="1em" height="1em" aria-hidden="true" />
              <span>Add group</span>
            </button>
          </div>

          <p v-if="!groups.length" class="note">No groups yet — fields show in the default flat list on item view.</p>

          <div
            v-for="(group, groupIndex) in groups"
            :key="group.key"
            class="field-group-card"
          >
            <div class="field-group-header">
              <div class="order-controls" role="group" aria-label="Reorder group">
                <button
                  type="button"
                  class="button small design-only"
                  title="Move group up"
                  :disabled="groupIndex === 0"
                  @click="moveGroup(groupIndex, 'up')"
                >
                  ↑
                </button>
                <button
                  type="button"
                  class="button small design-only"
                  title="Move group down"
                  :disabled="groupIndex === groups.length - 1"
                  @click="moveGroup(groupIndex, 'down')"
                >
                  ↓
                </button>
              </div>
              <input v-model="group.title" type="text" placeholder="Group title (e.g. Contact details)" />
              <button type="button" class="button small bad inline-icon" @click="removeGroup(groupIndex)">
                <HugeiconsIcon :icon="Delete01Icon" width="1em" height="1em" aria-hidden="true" />
                <span>Remove</span>
              </button>
            </div>
            <ul v-if="group.columnNames.length" class="assigned-column-list">
              <li
                v-for="(col, columnIndex) in group.columnNames"
                :key="`${group.key}-${col}`"
                class="assigned-column-row"
              >
                <code class="assigned-column-name">{{ col }}</code>
                <div class="order-controls" role="group" :aria-label="`Reorder ${col}`">
                  <button
                    type="button"
                    class="button small design-only"
                    title="Move field up"
                    :disabled="columnIndex === 0"
                    @click="moveColumnInGroup(groupIndex, columnIndex, 'up')"
                  >
                    ↑
                  </button>
                  <button
                    type="button"
                    class="button small design-only"
                    title="Move field down"
                    :disabled="columnIndex === group.columnNames.length - 1"
                    @click="moveColumnInGroup(groupIndex, columnIndex, 'down')"
                  >
                    ↓
                  </button>
                </div>
                <button
                  type="button"
                  class="button small neutral design-only"
                  title="Remove from group"
                  @click="removeColumnFromGroup(groupIndex, columnIndex)"
                >
                  Remove
                </button>
              </li>
            </ul>
            <p v-else class="note">Add at least one column to this group.</p>
            <label :for="`add-col-${group.key}`" class="add-col-label">Add column</label>
            <select
              :id="`add-col-${group.key}`"
              :disabled="!unassignedColumns.length"
              @change="(ev) => {
                const sel = ev.target as HTMLSelectElement
                addColumnToGroup(groupIndex, sel.value)
                sel.value = ''
              }"
            >
              <option value="">Choose column…</option>
              <option v-for="col in unassignedColumns" :key="col" :value="col">{{ col }}</option>
            </select>
          </div>
        </div>

        <div class="field-pool">
          <p class="pool-label">Unassigned columns</p>
          <p v-if="!unassignedColumns.length" class="note">All columns are assigned to groups.</p>
          <div class="column-chips">
            <span
              v-for="col in unassignedColumns"
              :key="col"
              class="column-chip pool"
            >{{ col }}</span>
          </div>
        </div>
      </div>
    </template>

    <div v-if="!loading" class="field-grouping-actions">
      <button
        type="button"
        class="button inline-icon neutral design-only"
        :disabled="saving || !groups.length"
        @click="resetGroups"
      >
        Clear groups
      </button>
      <button type="button" class="button inline-icon design-only" :disabled="saving" @click="saveGroups">
        {{ saving ? 'Saving…' : 'Save groups' }}
      </button>
    </div>
  </component>
</template>

<style scoped>
.field-grouping-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 0.5rem;
  margin-top: 1.25rem;
}

.field-grouping-help {
  margin: 0 0 1rem;
}
.field-grouping-layout {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}
.field-pool {
  margin-top: 0.25rem;
}
.pool-label {
  font-weight: 600;
  margin: 0 0 0.5rem;
}
.column-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}
.column-chip {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  padding: 0.2rem 0.5rem;
  border: 1px solid var(--border-color, #ccc);
  border-radius: 0.25rem;
  font-size: 0.9rem;
}
.column-chip.pool {
  background: var(--muted-surface, #f8f9fa);
}
.order-controls {
  display: flex;
  gap: 0.25rem;
  flex-shrink: 0;
}
.assigned-column-list {
  list-style: none;
  margin: 0 0 0.5rem;
  padding: 0;
}
.assigned-column-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
  padding: 0.35rem 0;
  border-bottom: 1px solid var(--border-color, #eee);
}
.assigned-column-row:last-child {
  border-bottom: none;
}
.assigned-column-name {
  flex: 1;
  min-width: 8rem;
  font-size: 0.9rem;
}
.groups-toolbar {
  margin-bottom: 0.75rem;
}
.field-group-card {
  border: 1px solid var(--border-color, #ccc);
  border-radius: 0.25rem;
  padding: 0.75rem 1rem;
  margin-bottom: 0.75rem;
}
.field-group-header {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
  margin-bottom: 0.5rem;
}
.field-group-header input {
  flex: 1;
  min-width: 10rem;
}
.add-col-label {
  display: block;
  margin: 0.5rem 0 0.25rem;
  font-size: 0.85rem;
}
</style>
