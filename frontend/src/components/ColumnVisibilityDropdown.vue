<script setup lang="ts">
import DropdownMenu from './DropdownMenu.vue'
import { onMounted } from 'vue'
import { ref } from 'vue'

const props = defineProps<{ columns: string[]; modelValue: string[] }>()
const emit = defineEmits<{ 'update:modelValue': [string[]] }>()

const dropdown = ref<InstanceType<typeof DropdownMenu> | null>(null);

function onChange(col: string) {
  const next = new Set(props.modelValue)
  next.has(col) ? next.delete(col) : next.add(col)
  emit('update:modelValue', Array.from(next))
}

onMounted(() => {
  props.columns.forEach(col => {
    dropdown.value.addCallback(col, () => onChange(col))
  })
})
</script>

<template>
  <DropdownMenu ref="dropdown" />
</template>


