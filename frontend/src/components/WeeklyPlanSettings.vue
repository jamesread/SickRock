<script setup lang="ts">
// Settings for a WeeklyPlan view (stored in the view's layout_json, see utils/weeklyPlan.ts).
import { computed } from 'vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { Add01Icon, ArrowDown01Icon, ArrowUp01Icon, Delete02Icon } from '@hugeicons/core-free-icons'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'
import type { ForeignKey } from '../gen/sickrock_pb'
import { weeklyPlanConfigProblems, type WeeklyPlanConfig, type WeeklyPlanSlot } from '../utils/weeklyPlan'

const props = defineProps<{
  fields: Array<{ name: string; type: string }>
  foreignKeys: ForeignKey[]
}>()

const config = defineModel<WeeklyPlanConfig>({ required: true })

const WEEKDAYS = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday']

const dateColumns = computed(() => props.fields.filter(field => /date|time/i.test(field.type)))
const textColumns = computed(() => props.fields.filter(field => /char|text|string/i.test(field.type)))
const itemColumns = computed(() => props.fields
  .filter(field => field.name !== 'id' && field.name !== config.value.dateColumn && field.name !== config.value.slotColumn)
  .map(field => {
    const fk = props.foreignKeys.find(f => f.columnName === field.name)
    const target = fk ? (fk.referencedTableTcName || fk.referencedTable) : ''
    return { name: field.name, label: target ? `${field.name} → ${target}` : field.name }
  }))
const problems = computed(() => weeklyPlanConfigProblems(config.value))

function update(patch: Partial<WeeklyPlanConfig>) {
  config.value = { ...config.value, ...patch }
}

function updateSlot(index: number, patch: Partial<WeeklyPlanSlot>) {
  update({ slots: config.value.slots.map((slot, i) => (i === index ? { ...slot, ...patch } : slot)) })
}

function moveSlot(index: number, offset: number) {
  const slots = [...config.value.slots]
  const target = index + offset
  if (target < 0 || target >= slots.length) return
  ;[slots[index], slots[target]] = [slots[target], slots[index]]
  update({ slots })
}

function removeSlot(index: number) {
  update({ slots: config.value.slots.filter((_, i) => i !== index) })
}

function addSlot() {
  update({ slots: [...config.value.slots, { label: '', time: '' }] })
}
</script>

<template>
  <div class="weekly-plan-settings">
    <label for="wp-date-column">Date column</label>
    <div>
      <select id="wp-date-column" :value="config.dateColumn" @change="update({ dateColumn: ($event.target as HTMLSelectElement).value })">
        <option value="">Choose…</option>
        <option v-for="field in dateColumns" :key="field.name" :value="field.name">{{ field.name }}</option>
      </select>
      <small>The day each row is planned for.</small>
    </div>

    <label for="wp-item-column">Slot contents</label>
    <div>
      <select id="wp-item-column" :value="config.itemColumn" @change="update({ itemColumn: ($event.target as HTMLSelectElement).value })">
        <option value="">Choose…</option>
        <option v-for="field in itemColumns" :key="field.name" :value="field.name">{{ field.label }}</option>
      </select>
      <small>The column that fills a slot. A relation (→) lets you pick rows from the other table, and shows their image.</small>
    </div>

    <label for="wp-slot-column">Slot column</label>
    <div>
      <select id="wp-slot-column" :value="config.slotColumn" @change="update({ slotColumn: ($event.target as HTMLSelectElement).value })">
        <option value="">None: place rows by the time of day in the date column</option>
        <option v-for="field in textColumns" :key="field.name" :value="field.name">{{ field.name }}</option>
      </select>
      <small>Without one, each slot's time is stored in the date column and decides which slot a row is in.</small>
    </div>

    <label>Slots</label>
    <div class="weekly-plan-slots">
      <div v-for="(slot, index) in config.slots" :key="index" class="weekly-plan-slot-row">
        <input
          :value="slot.label"
          type="text"
          placeholder="Slot name, e.g. Lunch"
          :aria-label="`Slot ${index + 1} name`"
          @input="updateSlot(index, { label: ($event.target as HTMLInputElement).value })"
        />
        <input
          :value="slot.time"
          type="time"
          :aria-label="`Slot ${index + 1} time`"
          @input="updateSlot(index, { time: ($event.target as HTMLInputElement).value })"
        />
        <input
          :value="slot.keywords ?? ''"
          type="text"
          placeholder="Initial keywords"
          :aria-label="`Slot ${index + 1} initial keywords`"
          title="Search text the picker starts with when adding to this slot"
          @input="updateSlot(index, { keywords: ($event.target as HTMLInputElement).value })"
        />
        <button type="button" class="button neutral" title="Move up" :disabled="index === 0" @click="moveSlot(index, -1)">
          <HugeiconsIcon :icon="ArrowUp01Icon" width="1em" height="1em" aria-hidden="true" />
        </button>
        <button type="button" class="button neutral" title="Move down" :disabled="index === config.slots.length - 1" @click="moveSlot(index, 1)">
          <HugeiconsIcon :icon="ArrowDown01Icon" width="1em" height="1em" aria-hidden="true" />
        </button>
        <button type="button" class="button neutral" title="Remove slot" @click="removeSlot(index)">
          <HugeiconsIcon :icon="Delete02Icon" width="1em" height="1em" aria-hidden="true" />
        </button>
      </div>
      <button type="button" class="button inline-icon neutral" @click="addSlot">
        <HugeiconsIcon :icon="Add01Icon" width="1em" height="1em" aria-hidden="true" />
        <span>Add slot</span>
      </button>
      <small>Each slot has a name, a time and optional initial keywords, which fill in the picker's search when adding to that slot (e.g. "breakfast").</small>
    </div>

    <label for="wp-week-start">Week starts on</label>
    <div>
      <select id="wp-week-start" :value="config.weekStartsOn" @change="update({ weekStartsOn: Number(($event.target as HTMLSelectElement).value) })">
        <option v-for="(day, index) in WEEKDAYS" :key="day" :value="index">{{ day }}</option>
      </select>
      <NotificationBlock v-if="problems.length > 0" type="warning" :message="problems.join(' ')" />
    </div>
  </div>
</template>

<style scoped>
/* Sits in one cell of the view form, so it lays out its own label / control rows. */
.weekly-plan-settings {
  display: grid;
  grid-template-columns: max-content minmax(0, 1fr);
  gap: 0.75rem 1rem;
  align-items: start;
}

.weekly-plan-settings > div > select {
  display: block;
  max-width: 100%;
}

.weekly-plan-settings small {
  display: block;
  margin-top: 0.25rem;
}

.weekly-plan-slots {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 0.4rem;
}

/* Name, time, initial keywords, then the three icon buttons, on one line. */
.weekly-plan-slot-row {
  display: grid;
  grid-template-columns: minmax(5rem, 9rem) 7.5rem minmax(6rem, 12rem) auto auto auto;
  align-items: center;
  gap: 0.3rem;
}

.weekly-plan-slot-row > * {
  margin: 0;
  min-width: 0;
}

.weekly-plan-slot-row input[type="text"] {
  width: 100%;
  box-sizing: border-box;
}

.weekly-plan-slot-row .button {
  padding: 0.4rem 0.45rem;
}
</style>
