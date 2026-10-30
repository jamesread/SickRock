<script setup lang="ts">
// Picks a row of another table from tiles showing its name and image, with a search on the name.
// Call open(); the chosen option is emitted as "pick".
import { computed, nextTick, ref } from 'vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { Cancel01Icon, Image01Icon, Search01Icon } from '@hugeicons/core-free-icons'
import ImageThumb from './ImageThumb.vue'
import { filterPickerOptions, type WeeklyPlanPickerOption } from '../utils/weeklyPlan'

const props = defineProps<{
  title: string
  options: WeeklyPlanPickerOption[]
}>()

const emit = defineEmits<{
  pick: [option: WeeklyPlanPickerOption]
}>()

const dialogEl = ref<HTMLDialogElement | null>(null)
const searchInput = ref<HTMLInputElement | null>(null)
const query = ref('')

const matches = computed(() => filterPickerOptions(props.options, query.value))

async function open() {
  query.value = ''
  dialogEl.value?.showModal()
  await nextTick()
  searchInput.value?.focus()
}

function close() {
  dialogEl.value?.close()
}

function pick(option: WeeklyPlanPickerOption) {
  close()
  emit('pick', option)
}

/** Enter in the search box picks the first match. */
function pickFirst() {
  const first = matches.value[0]
  if (first) pick(first)
}

defineExpose({ open, close })
</script>

<template>
  <dialog ref="dialogEl" class="dialog row-picker-dialog">
    <div class="row-picker-titlebar">
      <h2>{{ title }}</h2>
      <button type="button" class="button inline-icon neutral" aria-label="Close" @click="close">
        <HugeiconsIcon :icon="Cancel01Icon" width="1.25em" height="1.25em" aria-hidden="true" />
      </button>
    </div>

    <label class="row-picker-search">
      <HugeiconsIcon :icon="Search01Icon" width="1em" height="1em" aria-hidden="true" />
      <span class="visually-hidden">Search by name</span>
      <input
        ref="searchInput"
        v-model="query"
        type="text"
        enterkeyhint="search"
        placeholder="Search by name…"
        @keydown.enter.prevent="pickFirst"
      />
    </label>

    <ul v-if="matches.length > 0" class="row-picker-tiles">
      <li v-for="option in matches" :key="option.value">
        <button type="button" class="row-picker-tile" @click="pick(option)">
          <ImageThumb v-if="option.imageId" :image-id="option.imageId" :alt="option.label" size="tile" />
          <span v-else class="row-picker-no-image" aria-hidden="true">
            <HugeiconsIcon :icon="Image01Icon" width="2rem" height="2rem" />
          </span>
          <span class="row-picker-tile-label">{{ option.label }}</span>
        </button>
      </li>
    </ul>
    <p v-else class="subtle">{{ options.length === 0 ? 'There is nothing to choose from yet.' : `Nothing is named “${query.trim()}”.` }}</p>
  </dialog>
</template>

<style scoped>
.row-picker-dialog {
  width: min(52rem, 96vw);
  max-width: none;
  max-height: 90vh;
}

.row-picker-titlebar {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.row-picker-titlebar h2 {
  margin: 0;
}

.row-picker-search {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-block: 0.75rem;
}

.row-picker-search input {
  flex: 1;
  min-width: 0;
  margin: 0;
}

.row-picker-tiles {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(9rem, 1fr));
  gap: 0.75rem;
  margin: 0;
  padding: 0;
  list-style: none;
}

.row-picker-tile {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  margin: 0;
  padding: 0.4rem;
  gap: 0.4rem;
  text-align: start;
  border: 1px solid color-mix(in srgb, currentColor 20%, transparent);
  border-radius: 0.5rem;
  background: none;
  color: inherit;
  cursor: pointer;
}

.row-picker-tile:hover,
.row-picker-tile:focus-visible {
  border-color: currentColor;
}

.row-picker-no-image {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  aspect-ratio: 4 / 3;
  border-radius: 0.25rem;
  background: color-mix(in srgb, currentColor 8%, transparent);
  opacity: 0.6;
}

.row-picker-tile-label {
  overflow-wrap: anywhere;
}

.visually-hidden {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
}
</style>
