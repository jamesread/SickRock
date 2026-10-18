<script setup lang="ts">
// Picks a Hugeicons icon name. The name catalog is fetched from the server only when
// the dialog opens, and each tile loads just its own icon (see utils/hugeicons.ts).
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { QuestionIcon } from '@hugeicons/core-free-icons'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'
import HugeIcon from './HugeIcon.vue'
import { loadHugeiconCatalog, searchHugeicons, type HugeiconCatalog } from '../utils/hugeicons'

const PAGE_SIZE = 96

const props = withDefaults(defineProps<{
  modelValue?: string | null
  id?: string
  disabled?: boolean
  clearable?: boolean
  /** Open the dialog as soon as the field mounts (used for inline editing). */
  autoOpen?: boolean
}>(), {
  modelValue: '',
  clearable: true,
})

const emit = defineEmits<{
  'update:modelValue': [value: string]
  /** The dialog was closed without choosing an icon. */
  cancel: []
}>()

const dialogEl = ref<HTMLDialogElement | null>(null)
const searchInput = ref<HTMLInputElement | null>(null)
const catalog = ref<HugeiconCatalog | null>(null)
const loadingCatalog = ref(false)
const catalogError = ref<string | null>(null)
const query = ref('')
const debouncedQuery = ref('')
const visibleCount = ref(PAGE_SIZE)
let chosen = false
let debounceTimer: ReturnType<typeof setTimeout> | undefined

const currentName = computed(() => props.modelValue?.trim() ?? '')

const matches = computed(() => (catalog.value ? searchHugeicons(catalog.value, debouncedQuery.value) : []))
const visibleMatches = computed(() => matches.value.slice(0, visibleCount.value))

watch(query, (value) => {
  clearTimeout(debounceTimer)
  debounceTimer = setTimeout(() => {
    debouncedQuery.value = value
    visibleCount.value = PAGE_SIZE
  }, 150)
})

async function loadCatalog() {
  if (catalog.value || loadingCatalog.value) return
  loadingCatalog.value = true
  catalogError.value = null
  try {
    catalog.value = await loadHugeiconCatalog()
  } catch (e) {
    catalogError.value = e instanceof Error ? e.message : String(e)
  } finally {
    loadingCatalog.value = false
  }
}

function open() {
  if (props.disabled) return
  chosen = false
  dialogEl.value?.showModal()
  nextTick(() => searchInput.value?.focus())
  loadCatalog()
}

function choose(name: string) {
  chosen = true
  emit('update:modelValue', name)
  dialogEl.value?.close()
}

function clear() {
  emit('update:modelValue', '')
}

function onDialogClose() {
  if (!chosen) emit('cancel')
}

onMounted(() => {
  if (props.autoOpen) open()
})

defineExpose({ open })
</script>

<template>
  <div class="icon-picker-field">
    <HugeIcon :name="currentName" :fallback="QuestionIcon" width="1.5em" height="1.5em" aria-hidden="true" />
    <code v-if="currentName">{{ currentName }}</code>
    <span v-else class="subtle">No icon</span>
    <button :id="id" type="button" class="neutral" :disabled="disabled" @click="open">Choose…</button>
    <button v-if="clearable && currentName" type="button" class="neutral" :disabled="disabled" @click="clear">Clear</button>

    <dialog ref="dialogEl" class="dialog icon-picker-dialog" @close="onDialogClose">
      <h2>Choose an icon</h2>
      <input
        ref="searchInput"
        v-model="query"
        type="search"
        placeholder="Search icons, e.g. shield, calendar, user"
        aria-label="Search icons"
      />

      <NotificationBlock v-if="catalogError" type="error" :message="catalogError" />
      <p v-else-if="loadingCatalog" class="subtle">Loading icons…</p>
      <template v-else-if="catalog">
        <p class="subtle">{{ matches.length }} icon{{ matches.length === 1 ? '' : 's' }}</p>
        <div class="icon-picker-grid" role="listbox" aria-label="Icons">
          <button
            v-for="name in visibleMatches"
            :key="name"
            type="button"
            role="option"
            class="icon-picker-tile"
            :class="{ selected: name === currentName }"
            :aria-selected="name === currentName"
            :title="name"
            @click="choose(name)"
          >
            <HugeIcon :name="name" :fallback="QuestionIcon" width="1.5em" height="1.5em" aria-hidden="true" />
          </button>
        </div>
        <button
          v-if="visibleMatches.length < matches.length"
          type="button"
          class="neutral"
          @click="visibleCount += PAGE_SIZE"
        >
          Show more
        </button>
      </template>

      <div class="icon-picker-actions">
        <button v-if="catalogError" type="button" class="neutral" @click="loadCatalog">Retry</button>
        <button type="button" class="neutral" @click="dialogEl?.close()">Cancel</button>
      </div>
    </dialog>
  </div>
</template>

<style scoped>
.icon-picker-field {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-wrap: wrap;
}

/* femtocrank dialogs shrink to fit (max 600px), which collapses the auto-fill grid
   to a few columns: give the picker a fixed, viewport-relative width instead. */
.icon-picker-dialog {
  width: min(56rem, calc(100vw - 2rem));
  max-width: none;
  box-sizing: border-box;
}

.icon-picker-dialog input[type="search"] {
  width: 100%;
  box-sizing: border-box;
}

.icon-picker-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(3rem, 1fr));
  gap: 0.25rem;
  max-height: 50vh;
  overflow-y: auto;
  margin-block: 0.5rem;
}

.icon-picker-tile {
  display: flex;
  align-items: center;
  justify-content: center;
  aspect-ratio: 1;
  padding: 0;
}

.icon-picker-tile.selected {
  outline: 2px solid currentColor;
}

.icon-picker-actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
  margin-top: 0.5rem;
}
</style>
