<script setup lang="ts">
// The table structure page's Export section: which view the Export page starts on
// (table_configurations.default_export_view_id, see utils/exportView.ts).
import { computed, onMounted, ref, watch } from 'vue'
import Section from 'picocrank/vue/components/Section.vue'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'
import FormLayout from 'picocrank/vue/components/FormLayout.vue'
import FormField from 'picocrank/vue/components/FormField.vue'
import { Download01Icon } from '@hugeicons/core-free-icons'
import type { createApiClient } from '../stores/api'
import { defaultExportViewOptions } from '../utils/exportView'

const props = defineProps<{
  tableId: string
  client: ReturnType<typeof createApiClient>
  designModeEnabled: boolean
}>()

const loading = ref(true)
const saving = ref(false)
const error = ref<string | null>(null)
const saveMessage = ref<string | null>(null)
const views = ref<Array<{ id: number; viewName: string; isDefault: boolean }>>([])
const viewIdDraft = ref(0)

const options = computed(() => defaultExportViewOptions(views.value))

async function load() {
  loading.value = true
  error.value = null
  saveMessage.value = null
  try {
    const [structure, viewsRes] = await Promise.all([
      props.client.getTableStructure({ pageId: props.tableId }),
      props.client.getTableViews({ tableName: props.tableId }),
    ])
    views.value = (viewsRes.views ?? []).map(view => ({ id: view.id, viewName: view.viewName, isDefault: view.isDefault }))
    // A view that no longer exists shows as the automatic choice, which is what Export does with it.
    const stored = structure.defaultExportViewId ?? 0
    viewIdDraft.value = options.value.some(option => option.value === stored) ? stored : 0
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

async function save() {
  if (!props.designModeEnabled || saving.value) return
  saving.value = true
  error.value = null
  saveMessage.value = null
  try {
    const res = await props.client.setTableDefaultExportView({ tableConfiguration: props.tableId, viewId: viewIdDraft.value })
    if (!res.success) throw new Error(res.message || 'Failed to save the default export view')
    saveMessage.value = res.message || 'Saved'
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  void load()
})

watch(() => props.tableId, () => {
  void load()
})
</script>

<template>
  <Section title="Export" :icon="Download01Icon">
    <template #toolbar>
      <button
        type="button"
        class="button inline-icon good"
        :disabled="!designModeEnabled || loading || saving"
        @click="save"
      >
        <span>{{ saving ? 'Saving…' : 'Save export settings' }}</span>
      </button>
    </template>

    <div v-if="error" class="banner-pad">
      <NotificationBlock type="error" :message="error" />
    </div>
    <div v-if="saveMessage" class="banner-pad">
      <NotificationBlock type="success" :message="saveMessage" />
    </div>

    <p v-if="loading" class="muted">Loading export settings…</p>
    <FormLayout v-else @submit.prevent="save">
      <FormField
        label="Default export view"
        for="default-export-view"
        description="The view the Export page starts on, which sets the columns exported. It can still be changed there."
      >
        <select id="default-export-view" v-model.number="viewIdDraft" :disabled="!designModeEnabled || saving">
          <option v-for="option in options" :key="option.value" :value="option.value">{{ option.label }}</option>
        </select>
      </FormField>
    </FormLayout>
  </Section>
</template>

<style scoped>
.banner-pad {
  margin-bottom: 0.75rem;
}
</style>
