<script setup lang="ts">
// The Cards view type: the table's rows as a grid of cards with an image, a title and a few
// details (see utils/cardsView.ts). A card opens its row.
import { computed, onMounted, ref, watch } from 'vue'
import Section from 'picocrank/vue/components/Section.vue'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { Add01Icon, DashboardSquare01Icon, Search01Icon } from '@hugeicons/core-free-icons'
import { createApiClient } from '../stores/api'
import { useTableAccess } from '../composables/useTableAccess'
import { useDesignMode } from '../composables/useDesignMode'
import ViewsButton from './ViewsButton.vue'
import ImageThumb from './ImageThumb.vue'
import type { Item } from '../gen/sickrock_pb'
import { getItemNameFieldValue } from '../utils/itemViewSectionTitle'
import { hasImageValue } from '../utils/fieldTypes'
import { formatMySQLDateTime, formatUnixTimestamp } from '../utils/dateFormatting'
import { CARDS_VIEW_TYPE, filterByText, parseCardsConfig, type CardsConfig } from '../utils/cardsView'

const props = defineProps<{
  tableId: string
  viewId?: number
  viewName?: string | null
}>()

const emit = defineEmits<{
  'view-changed': [viewType: string, viewId?: number]
}>()

const client = createApiClient()
const { designModeEnabled } = useDesignMode()
const { access } = useTableAccess(computed(() => props.tableId))

const config = ref<CardsConfig>(parseCardsConfig(''))
const rows = ref<Item[]>([])
const fieldTypes = ref<Record<string, string>>({})
const loading = ref(true)
const error = ref<string | null>(null)
const query = ref('')

type Card = {
  id: string
  title: string
  imageId: string
  details: Array<{ column: string; label: string; value: string }>
}

const cards = computed<Card[]>(() => rows.value.map(row => ({
  id: String(row.id),
  title: titleOf(row),
  imageId: imageOf(row),
  details: config.value.detailColumns
    .map(column => ({ column, label: detailLabel(column), value: detailValue(row, column) }))
    .filter(detail => detail.value !== ''),
})))
// The search covers everything shown on a card: its title and its details.
const shownCards = computed(() => filterByText(cards.value, query.value, card => [
  card.title,
  ...card.details.map(detail => detail.value),
]))
const searchPlaceholder = computed(() => {
  const fields = ['name', ...config.value.detailColumns.map(column => detailLabel(column).toLowerCase())]
  return `Search ${fields.join(', ')}…`
})

function titleOf(row: Item): string {
  const fields = row.additionalFields ?? {}
  const chosen = config.value.titleColumn ? fields[config.value.titleColumn]?.trim() : ''
  return chosen
    || getItemNameFieldValue(row as unknown as Record<string, unknown>)
    || fields.title?.trim()
    || `#${row.id}`
}

function imageOf(row: Item): string {
  const value = config.value.imageColumn ? row.additionalFields?.[config.value.imageColumn] : ''
  return hasImageValue(value) ? String(value) : ''
}

/** "sr_created" → "Created", "cook_time" → "Cook time". */
function detailLabel(column: string): string {
  const words = column.replace(/^sr_/, '').replace(/_/g, ' ').trim()
  return words.charAt(0).toUpperCase() + words.slice(1)
}

function detailValue(row: Item, column: string): string {
  // id and the sr_ timestamps are fields of the item itself, not additionalFields.
  if (column === 'id') return String(row.id)
  if (column === 'sr_created') return formatUnixTimestamp(row.srCreated)
  if (column === 'sr_updated') return formatUnixTimestamp(row.srUpdated)
  const value = (row.additionalFields?.[column] ?? '').trim()
  if (!value) return ''
  if (/date|time/i.test(fieldTypes.value[column] ?? '')) return formatMySQLDateTime(value) || value
  return value
}

/** First letter of the title, for cards without an image. */
function initialOf(title: string): string {
  return (title.replace(/^#/, '').trim()[0] ?? '?').toUpperCase()
}

async function load() {
  loading.value = true
  error.value = null
  try {
    const [views, structure, items] = await Promise.all([
      client.getTableViews({ tableName: props.tableId }),
      client.getTableStructure({ pageId: props.tableId }),
      client.listItems({ tcName: props.tableId }),
    ])
    const view = views.views.find(v => v.id === props.viewId)
      ?? views.views.find(v => v.viewType === CARDS_VIEW_TYPE)
    config.value = parseCardsConfig(view?.layoutJson)
    fieldTypes.value = Object.fromEntries((structure.fields ?? []).map(field => [field.name, field.type]))
    rows.value = items.items ?? []
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

watch(() => [props.tableId, props.viewId], load)
onMounted(load)
</script>

<template>
  <Section :title="viewName || tableId" :icon="DashboardSquare01Icon" :padding="false">
    <template #toolbar>
      <ViewsButton
        :table-id="tableId"
        :show-view-create="access.canEdit"
        :show-view-edit="access.canEdit"
        @view-changed="(viewType: string, viewId?: number) => emit('view-changed', viewType, viewId)"
      />
      <router-link
        v-if="designModeEnabled"
        :to="`/table/${tableId}/column-types`"
        class="button neutral design-only"
      >Structure</router-link>
      <router-link v-if="access.canInsert" :to="`/table/${tableId}/insert-row`" class="button inline-icon good">
        <HugeiconsIcon :icon="Add01Icon" width="1em" height="1em" aria-hidden="true" />
        <span>Add</span>
      </router-link>
    </template>

    <p v-if="loading" class="padding">Loading…</p>
    <div v-else-if="error" class="padding">
      <NotificationBlock type="error" :message="error" />
    </div>
    <template v-else>
      <label v-if="cards.length > 0" class="cards-search padding">
        <HugeiconsIcon :icon="Search01Icon" width="1em" height="1em" aria-hidden="true" />
        <span class="visually-hidden">{{ searchPlaceholder }}</span>
        <input v-model="query" type="text" enterkeyhint="search" :placeholder="searchPlaceholder" />
      </label>

      <ul v-if="shownCards.length > 0" class="cards-grid" :class="`cards-${config.size}`">
        <li v-for="card in shownCards" :key="card.id">
          <router-link :to="`/table/${tableId}/${card.id}`" class="card">
            <template v-if="config.imageColumn">
              <ImageThumb v-if="card.imageId" :image-id="card.imageId" :alt="card.title" size="tile" class="card-image" />
              <span v-else class="card-image card-placeholder" aria-hidden="true">{{ initialOf(card.title) }}</span>
            </template>
            <span class="card-body">
              <span class="card-title">{{ card.title }}</span>
              <dl v-if="card.details.length > 0" class="card-details">
                <template v-for="detail in card.details" :key="detail.column">
                  <dt>{{ detail.label }}</dt>
                  <dd>{{ detail.value }}</dd>
                </template>
              </dl>
            </span>
          </router-link>
        </li>
      </ul>
      <p v-else-if="cards.length === 0" class="padding subtle">There is nothing here yet.</p>
      <p v-else class="padding subtle">Nothing matches “{{ query.trim() }}”.</p>
    </template>
  </Section>
</template>

<style scoped>
.cards-search {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  max-width: 28rem;
}

.cards-search input {
  flex: 1;
  min-width: 0;
  margin: 0;
}

.cards-grid {
  --card-min: 15rem;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(var(--card-min), 100%), 1fr));
  gap: 1rem;
  margin: 0;
  padding: 0 1rem 1rem;
  list-style: none;
}

.cards-small {
  --card-min: 10rem;
}

.cards-large {
  --card-min: 21rem;
}

.card {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
  border: 1px solid color-mix(in srgb, currentColor 15%, transparent);
  border-radius: 0.75rem;
  background: var(--section-bg-color, transparent);
  color: inherit;
  text-decoration: none;
  box-shadow: 0 1px 3px rgb(0 0 0 / 8%);
  transition: transform 0.15s ease, box-shadow 0.15s ease, border-color 0.15s ease;
}

.card:hover,
.card:focus-visible {
  transform: translateY(-2px);
  border-color: color-mix(in srgb, currentColor 35%, transparent);
  box-shadow: 0 6px 16px rgb(0 0 0 / 14%);
}

.card-image {
  width: 100%;
  aspect-ratio: 4 / 3;
  border-radius: 0;
}

.card-placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 2.5rem;
  font-weight: bold;
  background: linear-gradient(135deg, color-mix(in srgb, currentColor 6%, transparent), color-mix(in srgb, currentColor 16%, transparent));
  opacity: 0.7;
}

.card-body {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
  padding: 0.75rem 0.9rem 0.9rem;
}

.card-title {
  font-size: 1.05rem;
  font-weight: 600;
  overflow-wrap: anywhere;
}

.card-details {
  display: grid;
  grid-template-columns: max-content minmax(0, 1fr);
  gap: 0.15rem 0.6rem;
  margin: 0;
  font-size: 0.85rem;
}

.card-details dt {
  padding: 0;
  border: 0;
  text-align: start;
  opacity: 0.65;
  font-weight: normal;
}

.card-details dt::after {
  content: none;
}

.card-details dd {
  margin: 0;
  padding: 0;
  overflow-wrap: anywhere;
}

.visually-hidden {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
}

@media (prefers-reduced-motion: reduce) {
  .card {
    transition: none;
  }

  .card:hover,
  .card:focus-visible {
    transform: none;
  }
}
</style>
