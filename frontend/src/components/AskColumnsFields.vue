<script setup lang="ts">
// Inputs for the columns a weekly plan asks about when adding an entry (askColumns, e.g. "who"),
// each with the values recently used in that column as one-click choices, styled by the table's
// conditional formatting rules for that column. Renders FormFields, for a FormLayout around it
// (RowPickerDialog's before-search slot).
import FormField from 'picocrank/vue/components/FormField.vue'
import { conditionalStyles, type FormattingRule } from '../utils/conditionalFormatting'

const props = withDefaults(defineProps<{
  fields: Array<{ name: string; label: string; recent: string[] }>
  /** The table's conditional formatting rules. */
  rules?: FormattingRule[]
}>(), { rules: () => [] })

const values = defineModel<Record<string, string>>({ required: true })

const idPrefix = `ask-${Math.random().toString(36).slice(2)}`

function set(column: string, value: string) {
  values.value = { ...values.value, [column]: value }
}

/** Choosing the current value again clears it. */
function choose(column: string, value: string) {
  set(column, values.value[column] === value ? '' : value)
}

/** How the column's rules style this value (e.g. "James" in blue). */
function chipStyle(column: string, value: string) {
  return conditionalStyles(props.rules, () => value, column)
}
</script>

<template>
  <FormField v-for="field in fields" :key="field.name" :label="field.label" :for="`${idPrefix}-${field.name}`">
    <input
      :id="`${idPrefix}-${field.name}`"
      :value="values[field.name] ?? ''"
      type="text"
      autocomplete="off"
      :list="`${idPrefix}-${field.name}-recent`"
      @input="set(field.name, ($event.target as HTMLInputElement).value)"
    />
    <datalist :id="`${idPrefix}-${field.name}-recent`">
      <option v-for="value in field.recent" :key="value" :value="value" />
    </datalist>
    <div v-if="field.recent.length" class="action-row ask-columns-recent" role="group" :aria-label="`Recent ${field.label}`">
      <button
        v-for="value in field.recent"
        :key="value"
        type="button"
        class="button neutral"
        :class="{ chosen: values[field.name] === value }"
        :style="chipStyle(field.name, value)"
        :aria-pressed="values[field.name] === value"
        @click="choose(field.name, value)"
      >{{ value }}</button>
    </div>
  </FormField>
</template>

<style scoped>
.ask-columns-recent {
  justify-content: flex-start;
  margin-block-start: 0.35rem;
}

.ask-columns-recent .button {
  margin: 0;
  padding: 0.2rem 0.6rem;
}

/* The chosen value keeps its rule colours, so it is marked with an outline rather than "good". */
.ask-columns-recent .button.chosen {
  outline: 2px solid currentColor;
  outline-offset: 1px;
  font-weight: bold;
}
</style>
