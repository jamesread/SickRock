<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import CalendarComponent from '../components/CalendarComponent.vue'

const route = useRoute()
const slug = computed(() => String(route.params.slug ?? '').trim())
</script>

<template>
  <CalendarComponent
    v-if="slug"
    :export-slug="slug"
    read-only
  />
</template>
