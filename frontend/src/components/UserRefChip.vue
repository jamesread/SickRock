<script setup lang="ts">
import { HugeiconsIcon } from '@hugeicons/vue'
import { UserIcon } from '@hugeicons/core-free-icons'

defineProps<{
  label: string
  /** When set, the chip links to the IAM user profile. */
  to?: string
}>()
</script>

<template>
  <router-link v-if="to" :to="to" class="tag note user-ref-chip">
    <HugeiconsIcon :icon="UserIcon" width="1em" height="1em" aria-hidden="true" />
    <span>{{ label }}</span>
  </router-link>
  <span v-else class="tag note user-ref-chip">
    <HugeiconsIcon :icon="UserIcon" width="1em" height="1em" aria-hidden="true" />
    <span>{{ label }}</span>
  </span>
</template>

<style scoped>
.user-ref-chip {
  display: inline-flex;
  align-items: center;
  gap: 0.35em;
  vertical-align: middle;
  text-decoration: none;
}

a.user-ref-chip:hover {
  text-decoration: none;
}
</style>
