<script setup lang="ts">
// The user's own UI usage tracking: turn it on or off, see how much is logged, and download or
// clear the log (a JSONL file kept on the server, readable only by its owner).
import { computed, onMounted, ref } from 'vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { Analytics01Icon, Delete01Icon, Download01Icon } from '@hugeicons/core-free-icons'
import Section from 'picocrank/vue/components/Section.vue'
import FormLayout from 'picocrank/vue/components/FormLayout.vue'
import FormField from 'picocrank/vue/components/FormField.vue'
import RadioGroup from 'picocrank/vue/components/RadioGroup.vue'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'
import { createApiClient, currentSessionToken } from '../stores/api'
import { useUiUsageTracking } from '../composables/useUiUsageTracking'

const client = createApiClient()
const tracking = useUiUsageTracking()

const loading = ref(true)
const saving = ref(false)
const error = ref('')
const enabled = ref(false)
const lineCount = ref(0)
const maxLines = ref(1000)

const summary = computed(() => lineCount.value === 0
  ? 'Nothing is logged yet.'
  : `${lineCount.value} of at most ${maxLines.value} lines logged; the oldest are dropped as new ones arrive.`)

async function load() {
  error.value = ''
  try {
    await tracking.flush()
    const res = await client.getUIUsageSettings({})
    enabled.value = res.trackingEnabled
    lineCount.value = res.lineCount
    maxLines.value = res.maxLines || 1000
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

async function setEnabled(on: boolean) {
  saving.value = true
  error.value = ''
  try {
    const res = await client.setUIUsageTracking({ enabled: on })
    enabled.value = res.trackingEnabled
    tracking.setTrackingEnabled(res.trackingEnabled)
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    saving.value = false
  }
}

async function download() {
  error.value = ''
  try {
    await tracking.flush()
    const token = currentSessionToken()
    const res = await fetch('/api/ui-usage.jsonl', {
      headers: token ? { 'Session-Token': token } : {},
      credentials: 'include',
    })
    if (!res.ok) throw new Error(`Download failed (${res.status})`)
    const url = URL.createObjectURL(await res.blob())
    const a = document.createElement('a')
    a.href = url
    a.download = filenameFrom(res.headers.get('Content-Disposition')) || 'sickrock-ui-usage.jsonl'
    a.click()
    setTimeout(() => URL.revokeObjectURL(url), 1000)
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}

function filenameFrom(disposition: string | null): string {
  return /filename="([^"]+)"/.exec(disposition ?? '')?.[1] ?? ''
}

async function clearLog() {
  if (!confirm('Delete your UI usage log? This cannot be undone.')) return
  error.value = ''
  try {
    await client.clearUIUsageLog({})
    lineCount.value = 0
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}

onMounted(load)
</script>

<template>
  <Section title="UI usage" subtitle="Your own record of how you move around SickRock." :icon="Analytics01Icon">
    <template #toolbar>
      <button type="button" class="inline-icon neutral" :disabled="loading" @click="download">
        <HugeiconsIcon :icon="Download01Icon" width="1em" height="1em" aria-hidden="true" />
        <span>Download log</span>
      </button>
      <button type="button" class="inline-icon bad" :disabled="loading || lineCount === 0" @click="clearLog">
        <HugeiconsIcon :icon="Delete01Icon" width="1em" height="1em" aria-hidden="true" />
        <span>Clear log</span>
      </button>
    </template>

    <p v-if="loading" class="subtle">Loading…</p>
    <FormLayout v-else @submit.prevent>
      <FormField
        label="Track and log UI usage"
        component-has-label
        description="Records the pages you open and the sidebar and header links and buttons you click, with the time and app version, to help find navigation loops, hot spots and things to improve. Nothing you type is recorded. The log is kept on the server and only you can download it."
      >
        <RadioGroup
          :model-value="enabled"
          name="ui-usage-tracking"
          variant="boolean"
          aria-label="Track and log UI usage"
          :disabled="saving"
          :options="[
            { value: true, label: 'Yes' },
            { value: false, label: 'No' },
          ]"
          @update:model-value="setEnabled"
        />
      </FormField>
      <FormField label="Log" component-has-label :description="summary">
        <code>sickrock-ui-usage.jsonl</code>
      </FormField>
    </FormLayout>

    <NotificationBlock v-if="error" type="error" :message="error" />
  </Section>
</template>
