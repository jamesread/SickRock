<script setup lang="ts">
// Edit home: reorder, show and hide the home page sections, and choose the bookmarks shown as quick
// access buttons. Works on a copy; Save hands the new layout to the home page.
import { computed, ref } from 'vue'
import Dialog from 'picocrank/vue/components/Dialog.vue'
import ReorderList from 'picocrank/vue/components/ReorderList.vue'
import RadioGroup from 'picocrank/vue/components/RadioGroup.vue'
import CheckGroup from 'picocrank/vue/components/CheckGroup.vue'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'
import type { DialogApi } from '../utils/dialog'
import {
  defaultHomeLayout,
  homeSectionDef,
  type HomeLayout,
  type HomeLink,
  type HomeSection,
} from '../utils/homeLayout'

const props = defineProps<{
  layout: HomeLayout
  bookmarks: HomeLink[]
  saving?: boolean
  error?: string | null
}>()

const emit = defineEmits<{ save: [layout: HomeLayout] }>()

const dialogEl = ref<DialogApi | null>(null)
const draft = ref<HomeLayout>(defaultHomeLayout())

const SHOW_HIDE = [
  { value: true, label: 'Show' },
  { value: false, label: 'Hide' },
]

const bookmarkOptions = computed(() => props.bookmarks.map(b => ({ value: b.id, label: b.title })))
const chosenBookmarks = computed({
  get: () => draft.value.quickAccess
    .map(id => props.bookmarks.find(b => b.id === id))
    .filter((b): b is HomeLink => !!b),
  set: (links: HomeLink[]) => { draft.value.quickAccess = links.map(b => b.id) },
})

function show() {
  draft.value = JSON.parse(JSON.stringify(props.layout)) as HomeLayout
  dialogEl.value?.show()
}

function close() {
  dialogEl.value?.close()
}

function setVisible(section: HomeSection, visible: boolean) {
  draft.value.sections = draft.value.sections.map(s => (s.id === section.id ? { ...s, visible } : s))
}

function resetToDefault() {
  draft.value = defaultHomeLayout()
}

defineExpose({ show, close })
</script>

<template>
  <Dialog ref="dialogEl" title="Edit home" narrow-layout="fullscreen">
    <h3>Sections</h3>
    <p class="subtle">Drag or use the arrows to change the order, and choose what to show.</p>
    <ReorderList
      v-model="draft.sections"
      item-key="id"
      show-move-buttons
      aria-label="Home sections"
      :item-label="(section: HomeSection) => homeSectionDef(section.id).title"
    >
      <template #item="{ item }">
        <div class="home-section-row">
          <div>
            <strong>{{ homeSectionDef(item.id).title }}</strong>
            <p class="subtle">{{ homeSectionDef(item.id).description }}</p>
          </div>
          <RadioGroup
            :model-value="item.visible"
            variant="boolean"
            :name="`home-section-${item.id}`"
            :aria-label="`Show ${homeSectionDef(item.id).title}`"
            :options="SHOW_HIDE"
            @update:model-value="(visible: boolean) => setVisible(item, visible)"
          />
        </div>
      </template>
    </ReorderList>

    <h3>Quick access</h3>
    <template v-if="bookmarks.length">
      <p class="subtle">Choose bookmarks to show as buttons in the Quick access section.</p>
      <CheckGroup v-model="draft.quickAccess" name="home-quick-access" aria-label="Quick access bookmarks" :options="bookmarkOptions" />
      <template v-if="chosenBookmarks.length > 1">
        <p class="subtle">Button order:</p>
        <ReorderList
          v-model="chosenBookmarks"
          item-key="id"
          show-move-buttons
          aria-label="Quick access order"
          :item-label="(link: HomeLink) => link.title"
        />
      </template>
    </template>
    <p v-else class="inline-notification note">
      You have no bookmarks yet. Bookmark a page with the bookmark button in the header, then choose it here.
    </p>

    <NotificationBlock v-if="error" type="error" :message="error" />

    <template #actions>
      <button type="button" class="good" :disabled="saving" @click="emit('save', draft)">
        {{ saving ? 'Saving…' : 'Save' }}
      </button>
      <button type="button" class="neutral" :disabled="saving" @click="resetToDefault">Reset to default</button>
    </template>
  </Dialog>
</template>

<style scoped>
.home-section-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem 1rem;
}

.home-section-row p {
  margin: 0;
}
</style>
