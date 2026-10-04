<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'

import Section from 'picocrank/vue/components/Section.vue'
import FormField from 'picocrank/vue/components/FormField.vue'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'
import { Calendar03Icon } from '@hugeicons/core-free-icons'
import type { Field, RssCalendarFeed, RssFieldMapping } from '../gen/sickrock_pb'
import { createApiClient } from '../stores/api'
import { columnFieldKind, feedFieldKind, mappingMismatch } from '../utils/feedFieldMapping'

const route = useRoute()
const router = useRouter()
const client = createApiClient()

const isNew = computed(() => route.params.feedId === 'new')
const feedId = computed(() => (isNew.value ? 0 : Number(route.params.feedId)))

const name = ref('')
const feedUrl = ref('')
const tableConfiguration = ref('')
const uniqueColumn = ref('')
const refreshIntervalMinutes = ref(60)
const enabled = ref(true)
const mappings = ref<RssFieldMapping[]>([])

const tableOptions = ref<Array<{ value: string; label: string }>>([])
const tableColumns = ref<Field[]>([])
const rssFields = ref<string[]>([])
const previewLoading = ref(false)
const loading = ref(true)
const saving = ref(false)
const refreshing = ref(false)
const deleting = ref(false)
const error = ref<string | null>(null)
const mappingError = ref<string | null>(null)
const success = ref<string | null>(null)

const draggedRssField = ref<string | null>(null)

const knownRssFields = [
  'title',
  'link',
  'description',
  'content',
  'guid',
  'pubDate',
  'updated',
  'author',
  'startDate',
  'endDate',
  'location',
  'status',
]

const displayRssFields = computed(() => {
  const seen = new Set<string>()
  const out: string[] = []
  for (const f of [...knownRssFields, ...rssFields.value]) {
    if (seen.has(f)) continue
    seen.add(f)
    out.push(f)
  }
  return out
})

const mappedColumns = computed(() => new Set(mappings.value.map(m => m.column)))

async function loadTables() {
  const res = await client.getTableConfigurations({})
  tableOptions.value = (res.pages ?? [])
    .map(page => ({
      value: page.id,
      label: page.title?.trim() ? `${page.title} (${page.id})` : page.id,
    }))
    .filter(opt => opt.value && !opt.value.startsWith('table_'))
    .sort((a, b) => a.label.localeCompare(b.label))
}

async function loadTableColumns(tc: string) {
  if (!tc) {
    tableColumns.value = []
    return
  }
  const res = await client.getTableStructure({ pageId: tc })
  tableColumns.value = [...(res.fields ?? [])].sort((a, b) => a.name.localeCompare(b.name))
}

async function loadFeed() {
  if (isNew.value) return
  const res = await client.getRssCalendarFeed({ id: feedId.value })
  const feed = res.feed
  if (!feed) return
  name.value = feed.name
  feedUrl.value = feed.feedUrl
  tableConfiguration.value = feed.tableConfiguration
  uniqueColumn.value = feed.uniqueColumn
  refreshIntervalMinutes.value = feed.refreshIntervalMinutes || 60
  enabled.value = feed.enabled
  mappings.value = [...(feed.fieldMappings ?? [])]
}

async function previewFeed() {
  if (!feedUrl.value.trim()) return
  previewLoading.value = true
  error.value = null
  try {
    const res = await client.previewRssCalendarFeed({ feedUrl: feedUrl.value.trim() })
    rssFields.value = res.rssFields ?? []
  } catch (e) {
    error.value = String(e)
  } finally {
    previewLoading.value = false
  }
}

function onDragStart(rssField: string) {
  draggedRssField.value = rssField
}

function onDragEnd() {
  draggedRssField.value = null
}

function columnByName(column: string): Field | undefined {
  return tableColumns.value.find(col => col.name === column)
}

function dropMismatch(feedField: string, column: string): string | null {
  const col = columnByName(column)
  if (!col) return `${feedField} requires a matching column`
  return mappingMismatch(feedField, col.type)
}

function onDropColumn(column: string) {
  const rssField = draggedRssField.value
  if (!rssField || !column) return
  const mismatch = dropMismatch(rssField, column)
  if (mismatch) {
    mappingError.value = mismatch
    draggedRssField.value = null
    return
  }
  mappingError.value = null
  mappings.value = mappings.value.filter(m => m.column !== column && m.rssField !== rssField)
  mappings.value.push({ rssField, column })
  draggedRssField.value = null
}

function columnAcceptsDrag(column: Field): boolean {
  if (!draggedRssField.value) return true
  return mappingMismatch(draggedRssField.value, column.type) == null
}

function removeMapping(column: string) {
  mappings.value = mappings.value.filter(m => m.column !== column)
}

function mappingForColumn(column: string): RssFieldMapping | undefined {
  return mappings.value.find(m => m.column === column)
}

function buildFeedPayload(): RssCalendarFeed {
  return {
    id: feedId.value,
    name: name.value.trim(),
    feedUrl: feedUrl.value.trim(),
    tableConfiguration: tableConfiguration.value,
    fieldMappings: mappings.value,
    uniqueColumn: uniqueColumn.value.trim(),
    refreshIntervalMinutes: refreshIntervalMinutes.value,
    enabled: enabled.value,
  } as RssCalendarFeed
}

async function save() {
  error.value = null
  success.value = null
  for (const mapping of mappings.value) {
    const mismatch = dropMismatch(mapping.rssField, mapping.column)
    if (mismatch) {
      mappingError.value = mismatch
      return
    }
  }
  saving.value = true
  try {
    const res = await client.saveRssCalendarFeed({ feed: buildFeedPayload() })
    success.value = 'Feed saved.'
    if (isNew.value && res.feed?.id) {
      await router.replace(`/admin/rss-calendar-feeds/${res.feed.id}`)
    }
  } catch (e) {
    error.value = String(e)
  } finally {
    saving.value = false
  }
}

async function refreshNow() {
  if (!feedId.value) return
  refreshing.value = true
  error.value = null
  try {
    const res = await client.refreshRssCalendarFeed({ id: feedId.value })
    success.value = res.message || 'Refreshed.'
  } catch (e) {
    error.value = String(e)
  } finally {
    refreshing.value = false
  }
}

async function deleteFeed() {
  if (!feedId.value || !window.confirm('Delete this feed configuration?')) return
  deleting.value = true
  try {
    await client.deleteRssCalendarFeed({ id: feedId.value })
    await router.push('/admin/rss-calendar-feeds')
  } catch (e) {
    error.value = String(e)
  } finally {
    deleting.value = false
  }
}

watch(tableConfiguration, tc => void loadTableColumns(tc))

onMounted(async () => {
  loading.value = true
  try {
    await loadTables()
    await loadFeed()
    if (tableConfiguration.value) {
      await loadTableColumns(tableConfiguration.value)
    }
    if (feedUrl.value) {
      await previewFeed()
    }
  } catch (e) {
    error.value = String(e)
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <Section :title="isNew ? 'New calendar feed' : 'Edit calendar feed'" :icon="Calendar03Icon">
    <template #toolbar>
      <RouterLink to="/admin/rss-calendar-feeds" class="button neutral">All feeds</RouterLink>
      <button v-if="!isNew" type="button" class="button neutral" :disabled="refreshing" @click="refreshNow">
        {{ refreshing ? 'Refreshing…' : 'Refresh now' }}
      </button>
      <button type="button" class="button" :disabled="saving" @click="save">
        {{ saving ? 'Saving…' : 'Save' }}
      </button>
      <button v-if="!isNew" type="button" class="button bad" :disabled="deleting" @click="deleteFeed">
        Delete
      </button>
    </template>

    <NotificationBlock v-if="error" variant="bad">{{ error }}</NotificationBlock>
    <NotificationBlock v-if="success" variant="good">{{ success }}</NotificationBlock>

    <div v-if="loading">Loading…</div>
    <form v-else class="feed-form" @submit.prevent="save">
      <FormField label="Name" required>
        <input v-model="name" type="text" required class="input" />
      </FormField>

      <FormField label="Feed URL" required>
        <div class="url-row">
          <input v-model="feedUrl" type="url" required class="input" placeholder="https://…" />
          <button type="button" class="button neutral" :disabled="previewLoading" @click="previewFeed">
            {{ previewLoading ? 'Loading…' : 'Load fields' }}
          </button>
        </div>
      </FormField>

      <FormField label="Table configuration" required>
        <select v-model="tableConfiguration" required class="input">
          <option value="">Select table…</option>
          <option v-for="opt in tableOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
        </select>
      </FormField>

      <FormField label="Unique column (for updates)">
        <select v-model="uniqueColumn" class="input">
          <option value="">None (always insert)</option>
          <option v-for="col in tableColumns" :key="col.name" :value="col.name">{{ col.name }}</option>
        </select>
        <span class="text-sm muted">Usually map <code>guid</code> to this column to avoid duplicates.</span>
      </FormField>

      <FormField label="Refresh interval (minutes)">
        <input v-model.number="refreshIntervalMinutes" type="number" min="5" class="input" />
      </FormField>

      <FormField label="Enabled">
        <label class="checkbox-row">
          <input v-model="enabled" type="checkbox" />
          <span>Run scheduled refresh</span>
        </label>
      </FormField>

      <h3>Field mapping</h3>
      <p class="text-sm">
        Drag a feed field onto any table column of the same type. For example, <code>startDate</code> can map to a datetime column such as <code>event_start_date</code>.
      </p>
      <NotificationBlock v-if="mappingError" variant="bad">{{ mappingError }}</NotificationBlock>

      <div class="mapping-grid">
        <div class="mapping-panel">
          <h4>Feed fields</h4>
          <ul class="rss-field-list">
            <li
              v-for="field in displayRssFields"
              :key="field"
              class="rss-field-chip"
              draggable="true"
              @dragstart="onDragStart(field)"
              @dragend="onDragEnd"
            >
              {{ field }}
              <span class="field-kind">{{ feedFieldKind(field) }}</span>
            </li>
          </ul>
        </div>

        <div class="mapping-panel">
          <h4>Table columns</h4>
          <p v-if="!tableConfiguration" class="text-sm muted">Select a table first.</p>
          <ul v-else class="column-drop-list">
            <li
              v-for="col in tableColumns"
              :key="col.name"
              class="column-drop-target"
              :class="{
                mapped: mappedColumns.has(col.name),
                incompatible: draggedRssField && !columnAcceptsDrag(col),
              }"
              @dragover.prevent
              @drop.prevent="onDropColumn(col.name)"
            >
              <span class="column-name">{{ col.name }}</span>
              <span class="field-kind">{{ columnFieldKind(col.type) }}</span>
              <span v-if="mappingForColumn(col.name)" class="mapped-from">
                ← {{ mappingForColumn(col.name)?.rssField }}
                <button type="button" class="clear-map" title="Remove mapping" @click="removeMapping(col.name)">×</button>
              </span>
              <span v-else class="drop-hint">Drop feed field here</span>
            </li>
          </ul>
        </div>
      </div>
    </form>
  </Section>
</template>

<style scoped>
.feed-form {
  max-width: 52rem;
}

.url-row {
  display: flex;
  gap: 0.5rem;
  align-items: center;
}

.url-row .input {
  flex: 1;
}

.mapping-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 1rem;
  margin-top: 1rem;
}

.mapping-panel {
  border: 1px solid var(--border-color, #ddd);
  border-radius: 6px;
  padding: 0.75rem;
}

.mapping-panel h4 {
  margin: 0 0 0.5rem;
}

.rss-field-list,
.column-drop-list {
  list-style: none;
  margin: 0;
  padding: 0;
}

.rss-field-chip {
  display: inline-block;
  margin: 0.25rem;
  padding: 0.35rem 0.6rem;
  border-radius: 4px;
  background: var(--note-background, #eef4ff);
  cursor: grab;
  font-size: 0.9rem;
}

.column-drop-target {
  margin: 0.35rem 0;
  padding: 0.5rem;
  border: 1px dashed #bbb;
  border-radius: 4px;
  min-height: 2.25rem;
}

.column-drop-target.mapped {
  border-style: solid;
  background: var(--hover-background-color, #f8f9fa);
}

.column-drop-target.incompatible {
  opacity: 0.55;
  border-color: #dc3545;
}

.field-kind {
  margin-left: 0.4rem;
  font-size: 0.75rem;
  color: #666;
  text-transform: lowercase;
}

.column-name {
  font-weight: 600;
  margin-right: 0.5rem;
}

.mapped-from {
  font-size: 0.9rem;
}

.drop-hint {
  font-size: 0.85rem;
  color: #888;
}

.clear-map {
  margin-left: 0.35rem;
  border: none;
  background: transparent;
  cursor: pointer;
  font-size: 1.1rem;
  line-height: 1;
}

.checkbox-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

@media (max-width: 768px) {
  .mapping-grid {
    grid-template-columns: 1fr;
  }
}
</style>
