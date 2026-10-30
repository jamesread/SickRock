<script setup lang="ts">
// Settings for a Cards view (stored in the view's layout_json, see utils/cardsView.ts).
import { computed } from 'vue'
import { isImageFieldType } from '../utils/fieldTypes'
import { CARD_SIZES, MAX_CARD_DETAILS, type CardsConfig } from '../utils/cardsView'

const props = defineProps<{
  fields: Array<{ name: string; type: string }>
}>()

const config = defineModel<CardsConfig>({ required: true })

const SIZE_LABELS: Record<CardsConfig['size'], string> = { small: 'Small', medium: 'Medium', large: 'Large' }

const imageColumns = computed(() => props.fields.filter(field => isImageFieldType(field.type)))
const otherColumns = computed(() => props.fields.filter(field => !isImageFieldType(field.type)))
const detailChoices = computed(() => otherColumns.value.filter(field => field.name !== config.value.titleColumn))

function update(patch: Partial<CardsConfig>) {
  config.value = { ...config.value, ...patch }
}

function toggleDetail(name: string, checked: boolean) {
  const details = config.value.detailColumns.filter(column => column !== name)
  update({ detailColumns: checked ? [...details, name] : details })
}

function detailDisabled(name: string): boolean {
  return !config.value.detailColumns.includes(name) && config.value.detailColumns.length >= MAX_CARD_DETAILS
}
</script>

<template>
  <div class="cards-settings">
    <label for="cards-image-column">Image column</label>
    <div>
      <select id="cards-image-column" :value="config.imageColumn" @change="update({ imageColumn: ($event.target as HTMLSelectElement).value })">
        <option value="">None</option>
        <option v-for="field in imageColumns" :key="field.name" :value="field.name">{{ field.name }}</option>
      </select>
      <small v-if="imageColumns.length === 0">This table has no image columns yet. Add one with type Image to show pictures on the cards.</small>
      <small v-else>Shown across the top of each card.</small>
    </div>

    <label for="cards-title-column">Title column</label>
    <div>
      <select id="cards-title-column" :value="config.titleColumn" @change="update({ titleColumn: ($event.target as HTMLSelectElement).value })">
        <option value="">Automatic (name, title or id)</option>
        <option v-for="field in otherColumns" :key="field.name" :value="field.name">{{ field.name }}</option>
      </select>
    </div>

    <span class="fake-label">Details</span>
    <div>
      <div class="cards-settings-details">
        <label v-for="field in detailChoices" :key="field.name" class="cards-settings-detail">
          <input
            type="checkbox"
            :checked="config.detailColumns.includes(field.name)"
            :disabled="detailDisabled(field.name)"
            @change="toggleDetail(field.name, ($event.target as HTMLInputElement).checked)"
          />
          <span>{{ field.name }}</span>
        </label>
      </div>
      <small>Up to {{ MAX_CARD_DETAILS }} columns shown under the title, in the order you tick them.</small>
    </div>

    <label for="cards-size">Card size</label>
    <div>
      <select id="cards-size" :value="config.size" @change="update({ size: ($event.target as HTMLSelectElement).value as CardsConfig['size'] })">
        <option v-for="size in CARD_SIZES" :key="size" :value="size">{{ SIZE_LABELS[size] }}</option>
      </select>
    </div>
  </div>
</template>

<style scoped>
/* Sits in one cell of the view form, so it lays out its own label / control rows. */
.cards-settings {
  display: grid;
  grid-template-columns: max-content minmax(0, 1fr);
  gap: 0.75rem 1rem;
  align-items: start;
}

.cards-settings > div > select {
  display: block;
  max-width: 100%;
}

.cards-settings small {
  display: block;
  margin-top: 0.25rem;
}

.cards-settings-details {
  display: flex;
  flex-wrap: wrap;
  gap: 0.25rem 1rem;
}

.cards-settings-detail {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  font-weight: normal;
}

.cards-settings-detail input {
  margin: 0;
}
</style>
