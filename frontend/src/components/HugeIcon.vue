<script setup lang="ts">
// Renders a Hugeicon stored by name, showing `fallback` until it has loaded (or if it
// cannot be loaded). Extra attributes (width, height, aria-*) pass through.
import { onBeforeUnmount, ref, watch } from 'vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { whenHugeiconAvailable, type IconData } from '../utils/hugeicons'

defineOptions({ inheritAttrs: false })

const props = defineProps<{
  name?: string | null
  fallback: IconData
}>()

const loaded = ref<IconData | null>(null)
let cancel: (() => void) | null = null

watch(
  () => props.name,
  (name) => {
    cancel?.()
    loaded.value = null
    cancel = whenHugeiconAvailable(name, (icon) => {
      loaded.value = icon
    })
  },
  { immediate: true },
)

onBeforeUnmount(() => cancel?.())
</script>

<template>
  <HugeiconsIcon :icon="loaded ?? fallback" v-bind="$attrs" />
</template>
