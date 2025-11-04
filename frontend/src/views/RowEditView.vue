<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { createApiClient } from '../stores/api'
import { SickRock } from '../gen/sickrock_pb'
import InsertRow from '../components/InsertRow.vue'
import Section from 'picocrank/vue/components/Section.vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { ArrowLeft01Icon } from '@hugeicons/core-free-icons'

const route = useRoute()
const router = useRouter()
const tableName = route.params.tableName as string
const rowId = route.params.rowId as string

// Transport handled by authenticated client
const client = createApiClient()

const item = ref<Record<string, unknown> | null>(null)
const fieldDefs = ref<Array<{ name: string; type: string; required: boolean }>>([])
const loading = ref(false)
const error = ref<string | null>(null)
const createButtonText = ref<string>('Edit Row')

onMounted(async () => {
  loading.value = true
  try {
    // Load the item data
    const res = await client.getItem({ pageId: tableName, id: rowId })
    item.value = res.item as any ?? null

    if (!item.value) {
      error.value = 'Item not found'
      return
    }

    // Load table structure
    const structureRes = await client.getTableStructure({ pageId: tableName })
    fieldDefs.value = (structureRes.fields ?? [])
      .filter(f => f.name !== 'sr_created' && f.name !== 'sr_updated') // Hide sr_created and sr_updated fields
      .map(f => ({ name: f.name, type: f.type, required: !!f.required }))

    // Set the create button text as the title
    if (structureRes.CreateButtonText) {
      createButtonText.value = `Edit ${structureRes.CreateButtonText}`
    }
  } catch (e) {
    error.value = String(e)
  } finally {
    loading.value = false
  }
})

function handleUpdated() {
  router.push(`/table/${tableName}/${rowId}`)
}

function handleCancelled() {
  router.push(`/table/${tableName}/${rowId}`)
}

function goBack() {
  router.push(`/table/${tableName}/${rowId}`)
}
</script>

<template>
  <Section :title="createButtonText">
    <template #toolbar>
      <button @click="goBack" class="button back-button">
        <HugeiconsIcon :icon="ArrowLeft01Icon" width="16" height="16" />
        Back to Row
      </button>
    </template>

    <div v-if="error" class="error">{{ error }}</div>
    <div v-else-if="loading">Loading…</div>
    <InsertRow
      v-else
      :table-id="tableName"
      :field-defs="fieldDefs"
      :edit-mode="true"
      :item-id="rowId"
      :existing-item="item"
      @updated="handleUpdated"
      @cancelled="handleCancelled"
    />
  </Section>
</template>

<style scoped>
.error {
  background-color: #f8d7da;
  color: #721c24;
  padding: 0.75rem;
  border: 1px solid #f5c6cb;
  border-radius: 4px;
  margin: 1rem 0;
}
</style>
