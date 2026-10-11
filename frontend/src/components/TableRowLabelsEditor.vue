<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import Section from 'picocrank/vue/components/Section.vue'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'
import { TextIcon } from '@hugeicons/core-free-icons'
import type { createApiClient } from '../stores/api'
import {
  rowInsertActionLabel,
  rowNamePluralLabel,
  tableEmptyHeading,
} from '../utils/rowInsertLabel'

const props = defineProps<{
  tableId: string
  client: ReturnType<typeof createApiClient>
  designModeEnabled: boolean
}>()

const emit = defineEmits<{
  saved: []
}>()

const loading = ref(true)
const saving = ref(false)
const error = ref<string | null>(null)
const saveMessage = ref<string | null>(null)
const rowNameDraft = ref('')
const rowNamePluralDraft = ref('')

const previewCreate = computed(() => rowInsertActionLabel(rowNameDraft.value))
const previewPlural = computed(() =>
  rowNamePluralLabel(rowNamePluralDraft.value, rowNameDraft.value),
)
const previewEmpty = computed(() =>
  tableEmptyHeading(rowNamePluralDraft.value, rowNameDraft.value),
)

async function load() {
  loading.value = true
  error.value = null
  saveMessage.value = null
  try {
    const structure = await props.client.getTableStructure({ pageId: props.tableId })
    rowNameDraft.value = structure.rowName?.trim() ?? ''
    rowNamePluralDraft.value = structure.rowNamePlural?.trim() ?? ''
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

async function save() {
  if (!props.designModeEnabled || saving.value) {
    return
  }
  saving.value = true
  error.value = null
  saveMessage.value = null
  try {
    const res = await props.client.setTableRowLabels({
      tableConfiguration: props.tableId,
      rowName: rowNameDraft.value.trim(),
      rowNamePlural: rowNamePluralDraft.value.trim(),
    })
    if (!res.success) {
      throw new Error(res.message || 'Failed to save row labels')
    }
    saveMessage.value = res.message || 'Saved'
    emit('saved')
    await load()
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  void load()
})

watch(
  () => props.tableId,
  () => {
    void load()
  },
)
</script>

<template>
  <Section title="Row labels" :icon="TextIcon">
    <template #toolbar>
      <button
        type="button"
        class="button inline-icon good"
        :disabled="!designModeEnabled || loading || saving"
        @click="save"
      >
        <span>{{ saving ? 'Saving…' : 'Save row labels' }}</span>
      </button>
    </template>

    <p class="subtle">
      Singular and plural names for buttons and empty states (for example
      <strong>Create customer</strong> and <strong>No customers in this table</strong>).
      Leave blank to use the table title or generic defaults.
    </p>

    <div v-if="error" class="banner-pad">
      <NotificationBlock type="error" :message="error" />
    </div>
    <div v-if="saveMessage" class="banner-pad">
      <NotificationBlock type="success" :message="saveMessage" />
    </div>

    <p v-if="loading" class="muted">Loading row labels…</p>
    <div v-else class="row-labels-grid">
      <label class="row-label-field">
        <span class="row-label-caption">Row name (singular)</span>
        <input
          v-model="rowNameDraft"
          type="text"
          placeholder="e.g. customer"
          autocomplete="off"
          :disabled="!designModeEnabled || saving"
        />
      </label>
      <label class="row-label-field">
        <span class="row-label-caption">Row name (plural)</span>
        <input
          v-model="rowNamePluralDraft"
          type="text"
          placeholder="e.g. customers"
          autocomplete="off"
          :disabled="!designModeEnabled || saving"
        />
      </label>
    </div>

    <p v-if="!loading" class="preview muted">
      Preview:
      <code>{{ previewCreate }}</code>,
      list heading <code>{{ previewPlural }}</code>,
      empty state <code>{{ previewEmpty }}</code>
    </p>
  </Section>
</template>

<style scoped>
.subtle {
  color: var(--muted-text-color, #666);
  font-size: 0.95rem;
  margin: 0 0 0.75rem;
}

.banner-pad {
  margin-bottom: 0.75rem;
}

.row-labels-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(14rem, 1fr));
  gap: 1rem;
  margin-bottom: 0.75rem;
}

.row-label-field {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.row-label-caption {
  font-size: 0.9rem;
  font-weight: 500;
}

.row-label-field input {
  width: 100%;
  box-sizing: border-box;
}

.preview {
  margin: 0;
  font-size: 0.9em;
  line-height: 1.5;
}
</style>
