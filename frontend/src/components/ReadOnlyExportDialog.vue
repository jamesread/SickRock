<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { RouterLink } from 'vue-router'
import FormLayout from 'picocrank/vue/components/FormLayout.vue'
import FormField from 'picocrank/vue/components/FormField.vue'
import CheckGroup from 'picocrank/vue/components/CheckGroup.vue'
import { createApiClient } from '../stores/api'

const props = defineProps<{
  open: boolean
  tableConfiguration: string
  tableViewId?: number | null
  defaultTitle?: string
}>()

const emit = defineEmits<{
  close: []
  created: [slug: string]
}>()

const client = createApiClient()

const slug = ref('')
const title = ref('')
const displayMode = ref<'full' | 'free_busy'>('full')
const weekendsOnly = ref(false)
const groupIds = ref<number[]>([])
const groupOptions = ref<Array<{ value: number; label: string }>>([])
const groupsLoadFailed = ref(false)
const saving = ref(false)
const error = ref<string | null>(null)
const createdSlug = ref<string | null>(null)

const exportPath = computed(() =>
  createdSlug.value ? `/exports/${encodeURIComponent(createdSlug.value)}` : '',
)

function slugify(input: string): string {
  return input
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, '_')
    .replace(/^_+|_+$/g, '')
    .slice(0, 120)
}

function resetForm() {
  error.value = null
  createdSlug.value = null
  saving.value = false
  const base = slugify(props.defaultTitle || props.tableConfiguration)
  slug.value = base || slugify(props.tableConfiguration)
  title.value = props.defaultTitle || props.tableConfiguration
  displayMode.value = 'full'
  weekendsOnly.value = false
  groupIds.value = []
}

async function loadGroups() {
  groupsLoadFailed.value = false
  try {
    const res = await client.listUserGroups({})
    groupOptions.value = (res.groups ?? []).map(g => ({
      label: g.name,
      value: Number(g.id),
    }))
  } catch {
    groupsLoadFailed.value = true
    groupOptions.value = []
  }
}

watch(
  () => props.open,
  open => {
    if (open) {
      resetForm()
      void loadGroups()
    }
  },
)

async function submit() {
  const normalizedSlug = slugify(slug.value)
  if (!normalizedSlug) {
    error.value = 'Slug is required (letters, numbers, underscores).'
    return
  }
  if (!props.tableConfiguration) {
    error.value = 'Table configuration is missing.'
    return
  }

  saving.value = true
  error.value = null
  try {
    const fields: Record<string, string> = {
      slug: normalizedSlug,
      title: title.value.trim() || normalizedSlug,
      table_configuration: props.tableConfiguration,
      where_json: '{}',
      display_mode: displayMode.value,
      weekends_only: weekendsOnly.value ? '1' : '0',
      enabled: '1',
      allowed_group_ids: JSON.stringify(groupIds.value),
    }
    if (props.tableViewId != null && props.tableViewId > 0) {
      fields.table_view_id = String(props.tableViewId)
    }

    await client.createItem({
      pageId: 'table_read_only_exports',
      additionalFields: fields,
    })

    createdSlug.value = normalizedSlug
    emit('created', normalizedSlug)
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    saving.value = false
  }
}

function close() {
  emit('close')
}
</script>

<template>
  <div
    v-if="open"
    class="modal-overlay"
    @click.self="close"
    @keydown.escape="close"
    tabindex="0"
  >
    <div class="modal-content" @click.stop>
      <div class="modal-header flex-row g1">
        <h3>Create read-only calendar export</h3>
        <button type="button" class="button neutral" title="Close" @click="close">✕</button>
      </div>

      <div class="modal-body">
        <p v-if="!createdSlug" class="muted text-sm">
          Share this calendar at <code>/exports/{{ slug || 'your_slug' }}</code>. Users must sign in and belong to an allowed group (or hold exports.manage).
        </p>

        <p v-if="error" class="inline-notification error">{{ error }}</p>

        <div v-if="createdSlug" class="inline-notification good">
          <p>Export created.</p>
          <p>
            Open
            <RouterLink :to="exportPath">{{ exportPath }}</RouterLink>
            or manage it under Control Panel → Read-only calendar exports.
          </p>
          <div class="flex-row g1" style="margin-top: 0.75rem">
            <RouterLink :to="exportPath" class="button good">Open export</RouterLink>
            <button type="button" class="button neutral" @click="close">Close</button>
          </div>
        </div>

        <FormLayout v-else @submit.prevent="submit">
          <FormField label="Slug" for="export-slug">
            <input id="export-slug" v-model="slug" required autocomplete="off" />
            <span class="muted text-sm">URL segment: /exports/{{ slugify(slug || 'slug') }}</span>
          </FormField>

          <FormField label="Title" for="export-title">
            <input id="export-title" v-model="title" required autocomplete="off" />
          </FormField>

          <FormField label="Display mode" for="export-display-mode">
            <select id="export-display-mode" v-model="displayMode">
              <option value="full">Full (event titles visible)</option>
              <option value="free_busy">Free/busy (shows “Busy” only)</option>
            </select>
          </FormField>

          <FormField label="Weekends only" for="export-weekends">
            <input id="export-weekends" v-model="weekendsOnly" type="checkbox" />
          </FormField>

          <FormField
            v-if="groupOptions.length > 0"
            label="Allowed groups"
            component-has-label
          >
            <CheckGroup
              v-model="groupIds"
              name="export-allowed-groups"
              :options="groupOptions"
              :disabled="saving"
            />
            <span class="muted text-sm">Leave empty to restrict to superuser / exports.manage only.</span>
          </FormField>
          <p v-else-if="groupsLoadFailed" class="muted text-sm">
            Could not load groups (need usergroups.view). Set allowed groups later in the admin table.
          </p>

          <template #actions>
            <button type="button" class="button neutral" :disabled="saving" @click="close">Cancel</button>
            <button type="submit" class="button good" :disabled="saving">
              {{ saving ? 'Creating…' : 'Create export' }}
            </button>
          </template>
        </FormLayout>
      </div>
    </div>
  </div>
</template>

<style scoped>
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.45);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  padding: 1rem;
}

.modal-content {
  background: var(--surface-bg, #fff);
  color: var(--text, inherit);
  border-radius: 8px;
  max-width: 32rem;
  width: 100%;
  max-height: 90vh;
  overflow: auto;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.2);
}

.modal-header {
  justify-content: space-between;
  align-items: center;
  padding: 1rem 1.25rem;
  border-bottom: 1px solid var(--border-muted, #ddd);
}

.modal-header h3 {
  margin: 0;
  font-size: 1.1rem;
}

.modal-body {
  padding: 1rem 1.25rem 1.25rem;
}
</style>
