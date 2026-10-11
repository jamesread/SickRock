<script setup lang="ts">
import { provide, ref, watch } from 'vue'
import { accordionContextKey } from './accordionContext'

const props = withDefaults(
  defineProps<{
    /** Panel ids that start expanded */
    defaultOpenIds?: string[]
  }>(),
  {
    defaultOpenIds: () => [],
  },
)

const openIds = ref(new Set<string>())

function syncDefaultOpen() {
  openIds.value = new Set(props.defaultOpenIds.filter(Boolean))
}

watch(() => props.defaultOpenIds, syncDefaultOpen, { immediate: true, deep: true })

function isOpen(id: string): boolean {
  return openIds.value.has(id)
}

function toggle(id: string) {
  const next = new Set(openIds.value)
  if (next.has(id)) {
    next.delete(id)
  } else {
    next.add(id)
  }
  openIds.value = next
}

provide(accordionContextKey, { isOpen, toggle })
</script>

<template>
  <div class="accordion">
    <slot />
  </div>
</template>
