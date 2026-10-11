<script setup lang="ts">
import { ref, onMounted, computed, inject } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Section from 'picocrank/vue/components/Section.vue'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { ArrowLeft01Icon, LayoutIcon, PlusSignIcon, Delete01Icon, Link01Icon } from '@hugeicons/core-free-icons'
import type { createApiClient } from '../stores/api'
import type { ForeignKey } from '../gen/sickrock_pb'
import { tableListLinkLabel } from '../utils/rowInsertLabel'

type RelationBlockConfig = { constraint_name: string }
type RelationTabsBlockConfig = {
  section_title?: string
  tabs: Array<{ constraint_name: string; label?: string }>
}
type QuickJumpBlockConfig = { section_title?: string }

type EditorBlock = {
  key: string
  blockType: 'relation' | 'relation_tabs' | 'quick_jump'
  config: RelationBlockConfig | RelationTabsBlockConfig | QuickJumpBlockConfig
}

const route = useRoute()
const router = useRouter()
const tableName = route.params.tableName as string
const client = inject<ReturnType<typeof createApiClient>>('apiClient')

const loading = ref(true)
const saving = ref(false)
const error = ref<string | null>(null)
const saveMessage = ref<string | null>(null)
const foreignKeys = ref<ForeignKey[]>([])
const tableRowName = ref<string | null>(null)
const tableRowNamePlural = ref<string | null>(null)
const backToTableLabel = computed(() => tableListLinkLabel(tableRowNamePlural.value, tableRowName.value))
const blocks = ref<EditorBlock[]>([])

const dragIndex = ref<number | null>(null)
const overIndex = ref<number | null>(null)

let nextKey = 1
function newKey() {
  return `block-${nextKey++}`
}

function relationLabel(fk: ForeignKey): string {
  const dn = fk.displayName?.trim()
  if (dn) return dn
  return `${fk.tableName} (${fk.columnName})`
}

const constraintOptions = computed(() =>
  foreignKeys.value.map(fk => ({
    value: fk.constraintName,
    label: relationLabel(fk),
  })),
)

const hasQuickJumpBlock = computed(() => blocks.value.some(b => b.blockType === 'quick_jump'))

async function load() {
  loading.value = true
  error.value = null
  try {
    const structure = await client.getTableStructure({ pageId: tableName })
    tableRowName.value = structure.rowName?.trim() || null
    tableRowNamePlural.value = structure.rowNamePlural?.trim() || null
    foreignKeys.value = structure.foreignKeys ?? []
    const raw = structure.itemPageLayout?.blocks ?? []
    const sorted = [...raw].sort((a, b) => (a.ordinal ?? 0) - (b.ordinal ?? 0))
    blocks.value = []
    for (const b of sorted) {
      try {
        const cfg = JSON.parse(b.configJson || '{}')
        if (b.blockType === 'quick_jump') {
          blocks.value.push({
            key: newKey(),
            blockType: 'quick_jump',
            config: { section_title: cfg.section_title ?? '' },
          })
        } else if (b.blockType === 'relation') {
          blocks.value.push({
            key: newKey(),
            blockType: 'relation',
            config: { constraint_name: String(cfg.constraint_name ?? '') },
          })
        } else if (b.blockType === 'relation_tabs') {
          blocks.value.push({
            key: newKey(),
            blockType: 'relation_tabs',
            config: {
              section_title: cfg.section_title ?? '',
              tabs: (cfg.tabs ?? []).map((t: { constraint_name?: string; label?: string }) => ({
                constraint_name: String(t.constraint_name ?? ''),
                label: t.label ?? '',
              })),
            },
          })
        }
      } catch {
        // skip invalid block
      }
    }
  } catch (e) {
    error.value = String(e)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void load()
})

function addQuickJumpBlock() {
  if (hasQuickJumpBlock.value) return
  blocks.value.push({
    key: newKey(),
    blockType: 'quick_jump',
    config: { section_title: '' },
  })
}

function addRelationBlock() {
  const first = constraintOptions.value[0]?.value ?? ''
  blocks.value.push({
    key: newKey(),
    blockType: 'relation',
    config: { constraint_name: first },
  })
}

function addTabsBlock() {
  blocks.value.push({
    key: newKey(),
    blockType: 'relation_tabs',
    config: {
      section_title: '',
      tabs: constraintOptions.value.length
        ? [{ constraint_name: constraintOptions.value[0].value, label: '' }]
        : [],
    },
  })
}

function removeBlock(index: number) {
  blocks.value.splice(index, 1)
}

function addTab(block: EditorBlock) {
  if (block.blockType !== 'relation_tabs') return
  const cfg = block.config as RelationTabsBlockConfig
  const first = constraintOptions.value[0]?.value ?? ''
  cfg.tabs.push({ constraint_name: first, label: '' })
}

function removeTab(block: EditorBlock, tabIndex: number) {
  if (block.blockType !== 'relation_tabs') return
  const cfg = block.config as RelationTabsBlockConfig
  cfg.tabs.splice(tabIndex, 1)
}

function onDragStart(index: number, ev: DragEvent) {
  dragIndex.value = index
  ev.dataTransfer?.setData('text/plain', String(index))
}

function onDragOverAt(index: number, ev: DragEvent) {
  ev.preventDefault()
  const el = ev.currentTarget as HTMLElement | null
  if (el) {
    const rect = el.getBoundingClientRect()
    overIndex.value = ev.clientY - rect.top < rect.height / 2 ? index : index + 1
  }
  if (ev.dataTransfer) ev.dataTransfer.dropEffect = 'move'
}

function onDrop(targetIndex: number, ev: DragEvent) {
  ev.preventDefault()
  const from = dragIndex.value ?? parseInt(ev.dataTransfer?.getData('text/plain') || '-1', 10)
  const to = overIndex.value ?? targetIndex
  if (Number.isNaN(from) || from < 0 || from === to) return
  const list = [...blocks.value]
  const [moved] = list.splice(from, 1)
  const boundedTo = Math.max(0, Math.min(list.length, to))
  list.splice(boundedTo, 0, moved)
  blocks.value = list
  dragIndex.value = null
  overIndex.value = null
}

function onDragEnd() {
  dragIndex.value = null
  overIndex.value = null
}

async function saveLayout() {
  saving.value = true
  error.value = null
  saveMessage.value = null
  try {
    const payload = blocks.value.map((b, i) => {
      let configJson: string
      if (b.blockType === 'quick_jump') {
        const cfg = b.config as QuickJumpBlockConfig
        configJson = JSON.stringify(
          cfg.section_title?.trim() ? { section_title: cfg.section_title.trim() } : {},
        )
      } else if (b.blockType === 'relation') {
        configJson = JSON.stringify({
          constraint_name: (b.config as RelationBlockConfig).constraint_name,
        })
      } else {
        configJson = JSON.stringify({
          section_title: (b.config as RelationTabsBlockConfig).section_title || undefined,
          tabs: (b.config as RelationTabsBlockConfig).tabs.map(t => ({
            constraint_name: t.constraint_name,
            ...(t.label?.trim() ? { label: t.label.trim() } : {}),
          })),
        })
      }
      return { ordinal: i + 1, blockType: b.blockType, configJson }
    })
    const res = await client.setItemPageLayout({
      tableConfiguration: tableName,
      blocks: payload,
    })
    if (!res.success) {
      throw new Error(res.message || 'Save failed')
    }
    saveMessage.value = res.message || 'Layout saved'
    await load()
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    saving.value = false
  }
}

async function resetLayout() {
  if (!confirm('Clear the custom layout? All relations will use the default stacked sections on item view.')) {
    return
  }
  saving.value = true
  error.value = null
  try {
    const res = await client.setItemPageLayout({
      tableConfiguration: tableName,
      blocks: [],
    })
    if (!res.success) {
      throw new Error(res.message || 'Reset failed')
    }
    blocks.value = []
    saveMessage.value = 'Layout reset'
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    saving.value = false
  }
}

function goBack() {
  router.push({ name: 'table', params: { tableName } })
}
</script>

<template>
  <Section :title="`Item view layout: ${tableName}`" :icon="LayoutIcon">
    <template #toolbar>
      <button type="button" class="button inline-icon neutral" @click="goBack">
        <HugeiconsIcon :icon="ArrowLeft01Icon" width="1em" height="1em" aria-hidden="true" />
        <span>{{ backToTableLabel }}</span>
      </button>
      <button type="button" class="button inline-icon good" :disabled="saving || loading" @click="saveLayout">
        {{ saving ? 'Saving…' : 'Save layout' }}
      </button>
      <button type="button" class="button inline-icon neutral" :disabled="saving || loading" @click="resetLayout">
        Reset layout
      </button>
    </template>

    <NotificationBlock v-if="error" variant="bad" :message="error" />
    <NotificationBlock v-else-if="saveMessage" variant="good" :message="saveMessage" />

    <p v-if="loading">Loading…</p>
    <template v-else>
      <p class="layout-editor-help">
        Arrange how related tables appear on each item row page. Relations not placed here still appear below in the default order.
      </p>

      <div role="toolbar" class="layout-editor-palette">
        <button
          type="button"
          class="button inline-icon neutral"
          :disabled="!constraintOptions.length || hasQuickJumpBlock"
          @click="addQuickJumpBlock"
        >
          <HugeiconsIcon :icon="Link01Icon" width="1em" height="1em" aria-hidden="true" />
          <span>Quick jump</span>
        </button>
        <button type="button" class="button inline-icon neutral" :disabled="!constraintOptions.length" @click="addRelationBlock">
          <HugeiconsIcon :icon="PlusSignIcon" width="1em" height="1em" aria-hidden="true" />
          <span>Single relation</span>
        </button>
        <button type="button" class="button inline-icon neutral" :disabled="!constraintOptions.length" @click="addTabsBlock">
          <HugeiconsIcon :icon="PlusSignIcon" width="1em" height="1em" aria-hidden="true" />
          <span>Tab group</span>
        </button>
      </div>

      <p v-if="!constraintOptions.length" class="note">This table has no relations to lay out.</p>

      <div v-else class="layout-blocks-list" @dragover.prevent>
        <div
          v-for="(block, index) in blocks"
          :key="block.key"
          class="layout-block-card"
          @dragover="onDragOverAt(index, $event)"
          @drop="onDrop(index, $event)"
          @dragend="onDragEnd"
        >
          <div class="layout-block-header">
            <span
              class="drag-handle"
              title="Drag to reorder"
              draggable="true"
              @dragstart="onDragStart(index, $event)"
            >⠿</span>
            <strong>{{
              block.blockType === 'quick_jump'
                ? 'Quick jump'
                : block.blockType === 'relation'
                  ? 'Single relation'
                  : 'Tab group'
            }}</strong>
            <button type="button" class="button small bad inline-icon" @click="removeBlock(index)">
              <HugeiconsIcon :icon="Delete01Icon" width="1em" height="1em" aria-hidden="true" />
              <span>Remove</span>
            </button>
          </div>

          <template v-if="block.blockType === 'quick_jump'">
            <p class="note layout-quick-jump-help">
              Inline links to every relation section below this block on the item page (including unplaced relations).
            </p>
            <label :for="`jump-title-${block.key}`">Section title (optional)</label>
            <input
              :id="`jump-title-${block.key}`"
              v-model="(block.config as QuickJumpBlockConfig).section_title"
              type="text"
              placeholder="On this page"
            />
          </template>

          <template v-else-if="block.blockType === 'relation'">
            <label :for="`rel-${block.key}`">Relation</label>
            <select :id="`rel-${block.key}`" v-model="(block.config as RelationBlockConfig).constraint_name">
              <option v-for="opt in constraintOptions" :key="opt.value" :value="opt.value">
                {{ opt.label }}
              </option>
            </select>
          </template>

          <template v-else>
            <label :for="`title-${block.key}`">Section title (optional)</label>
            <input
              :id="`title-${block.key}`"
              v-model="(block.config as RelationTabsBlockConfig).section_title"
              type="text"
              placeholder="Optional heading above tabs"
            />
            <p class="layout-tabs-label">Tabs</p>
            <div
              v-for="(tab, tabIndex) in (block.config as RelationTabsBlockConfig).tabs"
              :key="`${block.key}-tab-${tabIndex}`"
              class="layout-tab-row"
            >
              <select v-model="tab.constraint_name">
                <option v-for="opt in constraintOptions" :key="opt.value" :value="opt.value">
                  {{ opt.label }}
                </option>
              </select>
              <input v-model="tab.label" type="text" placeholder="Tab label (optional)" />
              <button
                type="button"
                class="button small"
                :disabled="(block.config as RelationTabsBlockConfig).tabs.length <= 1"
                @click="removeTab(block, tabIndex)"
              >
                Remove tab
              </button>
            </div>
            <button type="button" class="button small neutral" @click="addTab(block)">
              Add tab
            </button>
          </template>
        </div>
      </div>

      <p v-if="constraintOptions.length && !blocks.length" class="note">
        No custom layout yet — related sections use the default stacked order on item view.
      </p>
    </template>
  </Section>
</template>

<style scoped>
.layout-editor-help {
  margin-bottom: 1rem;
}
.layout-editor-palette {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  margin-bottom: 1rem;
}
.layout-blocks-list {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}
.layout-block-card {
  border: 1px solid var(--border-color, #ccc);
  border-radius: 0.25rem;
  padding: 0.75rem 1rem;
}
.layout-block-header {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  margin-bottom: 0.75rem;
}
.drag-handle {
  cursor: grab;
  user-select: none;
}
.layout-tab-row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  align-items: center;
  margin-bottom: 0.5rem;
}
.layout-tabs-label {
  font-weight: 600;
  margin: 0.5rem 0 0.25rem;
}
.layout-quick-jump-help {
  margin: 0 0 0.75rem;
}
</style>
