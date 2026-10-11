<script setup lang="ts">
import { computed, inject } from 'vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { ArrowDown01Icon } from '@hugeicons/core-free-icons'
import { accordionContextKey } from './accordionContext'

const props = defineProps<{
  itemId: string
  title: string
}>()

const accordion = inject(accordionContextKey)

const open = computed(() => accordion?.isOpen(props.itemId) ?? false)

function onToggle() {
  accordion?.toggle(props.itemId)
}
</script>

<template>
  <div class="accordion-item" :data-open="open ? 'true' : 'false'">
    <div class="section-subheader">
      <button
        type="button"
        class="accordion-trigger flex-row"
        :aria-expanded="open"
        :id="`accordion-trigger-${itemId}`"
        :aria-controls="`accordion-panel-${itemId}`"
        @click="onToggle"
      >
        <HugeiconsIcon
          class="accordion-chevron"
          :icon="ArrowDown01Icon"
          width="1em"
          height="1em"
          aria-hidden="true"
        />
        <h3>{{ title }}</h3>
      </button>
    </div>
    <div
      v-show="open"
      :id="`accordion-panel-${itemId}`"
      class="section-content padding"
      role="region"
      :aria-labelledby="`accordion-trigger-${itemId}`"
    >
      <slot />
    </div>
  </div>
</template>

<style scoped>
.accordion-trigger {
  width: 100%;
  margin: 0;
  padding: 0;
  border: none;
  background: none;
  cursor: pointer;
  font: inherit;
  color: inherit;
  text-align: left;
  align-items: center;
  gap: 0.35rem;
}

.accordion-trigger h3 {
  margin: 0;
}

.accordion-chevron {
  flex-shrink: 0;
  transition: transform 0.15s ease;
}

.accordion-item[data-open='true'] .accordion-chevron {
  transform: rotate(180deg);
}
</style>
