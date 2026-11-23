<script setup lang="ts">
// Copies a weekly plan's entries from another week into the shown week (copyWeek in
// utils/weeklyPlan.ts): each keeps its weekday, time, slot and columns. Entries already planned in
// the same cell are skipped, so copying twice adds nothing.
import { computed, ref } from 'vue'
import Dialog from 'picocrank/vue/components/Dialog.vue'
import FormLayout from 'picocrank/vue/components/FormLayout.vue'
import FormField from 'picocrank/vue/components/FormField.vue'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'
import { showNotificationPopup } from 'picocrank/vue/composables/useNotificationPopups.js'
import { createApiClient } from '../stores/api'
import type { Item } from '../gen/sickrock_pb'
import type { DialogApi } from '../utils/dialog'
import { addDays, copyWeek, dayKey, plannedWeeks, type WeeklyPlanConfig } from '../utils/weeklyPlan'

const props = defineProps<{
  tableId: string
  config: WeeklyPlanConfig
  rows: Item[]
  /** The table's columns (what a copy carries over). */
  columns: string[]
  /** The week entries are copied into. */
  weekStart: Date
}>()

const emit = defineEmits<{ copied: [] }>()

const client = createApiClient()
const dialogEl = ref<DialogApi | null>(null)
const fromKey = ref('')
const copying = ref(false)
const errorMessage = ref('')
const formId = `copy-week-form-${Math.random().toString(36).slice(2)}`

const short: Intl.DateTimeFormatOptions = { day: 'numeric', month: 'short' }
function weekRange(start: Date): string {
  return `${start.toLocaleDateString(undefined, short)} – ${addDays(start, 6).toLocaleDateString(undefined, { ...short, year: 'numeric' })}`
}

const columnValue = (row: Item, column: string) => (column === 'id' ? row.id : row.additionalFields?.[column])
const weeks = computed(() => plannedWeeks(props.config, props.rows, columnValue)
  .filter(week => dayKey(week.start) !== dayKey(props.weekStart)))
const fromWeek = computed(() => weeks.value.find(week => dayKey(week.start) === fromKey.value)?.start ?? null)
const copy = computed(() => (fromWeek.value
  ? copyWeek(props.config, props.rows, props.columns, fromWeek.value, props.weekStart)
  : { rows: [], skipped: 0 }))

const summary = computed(() => {
  if (!fromWeek.value) return ''
  const n = copy.value.rows.length
  const added = n === 0 ? 'Nothing to copy' : `Adds ${n} ${n === 1 ? 'entry' : 'entries'} to ${weekRange(props.weekStart)}`
  const skipped = copy.value.skipped ? `; ${copy.value.skipped} already planned there ${copy.value.skipped === 1 ? 'is' : 'are'} skipped` : ''
  return `${added}${skipped}.`
})

/** Opens the dialog on the newest planned week before the shown one (else the newest of all). */
function open() {
  errorMessage.value = ''
  const before = weeks.value.find(week => week.start < props.weekStart) ?? weeks.value[0]
  fromKey.value = before ? dayKey(before.start) : ''
  dialogEl.value?.show()
}

async function submit() {
  if (copying.value || copy.value.rows.length === 0) return
  copying.value = true
  errorMessage.value = ''
  let added = 0
  try {
    for (const fields of copy.value.rows) {
      await client.createItem({ pageId: props.tableId, additionalFields: fields })
      added++
    }
    dialogEl.value?.close()
    showNotificationPopup({ message: `Copied ${added} ${added === 1 ? 'entry' : 'entries'}`, class: 'good' })
  } catch (e) {
    const reason = e instanceof Error ? e.message : String(e)
    errorMessage.value = added ? `Copied ${added}, then failed: ${reason}` : reason
  } finally {
    copying.value = false
    if (added) emit('copied')
  }
}

defineExpose({ open })
</script>

<template>
  <Dialog ref="dialogEl" title="Copy from week">
    <FormLayout :id="formId" @submit.prevent="submit">
      <FormField label="Copy entries from" for="copy-week-from" :disabled="copying">
        <select id="copy-week-from" v-model="fromKey" :disabled="copying || weeks.length === 0">
          <option v-if="weeks.length === 0" value="">No other week has entries</option>
          <option v-for="week in weeks" :key="dayKey(week.start)" :value="dayKey(week.start)">
            {{ weekRange(week.start) }} ({{ week.count }} {{ week.count === 1 ? 'entry' : 'entries' }})
          </option>
        </select>
        <small class="subtle">Each entry keeps its day of the week, slot and details.</small>
      </FormField>
      <p v-if="summary">{{ summary }}</p>
      <NotificationBlock v-if="errorMessage" type="error" :message="errorMessage" />
    </FormLayout>

    <template #actions>
      <button type="submit" :form="formId" class="good" :disabled="copying || copy.rows.length === 0">
        {{ copying ? 'Copying…' : 'Copy' }}
      </button>
    </template>
  </Dialog>
</template>
