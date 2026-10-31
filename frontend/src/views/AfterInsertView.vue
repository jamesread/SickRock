<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Section from 'picocrank/vue/components/Section.vue'
import { CheckmarkSquare03Icon } from '@hugeicons/core-free-icons'
import { createApiClient } from '../stores/api'
import { rowNameLabel, tableListLinkLabel } from '../utils/rowInsertLabel'

const route = useRoute()
const router = useRouter()
const tableId = route.params.tableName as string
const fromTable = route.query.fromTable as string | undefined
const fromRowId = route.query.fromRowId as string | undefined
const dashboardName = route.query.dashboardName as string | undefined

const client = createApiClient()
const rowLabel = ref('Row')
const rowNamePlural = ref<string | null>(null)

const isFromColumnAddition = ref(false)

const addedTitle = computed(() =>
  isFromColumnAddition.value ? 'Column Added Successfully' : `${rowLabel.value} Added Successfully`,
)
const addedMessage = computed(() =>
  isFromColumnAddition.value
    ? '✅ Column added successfully!'
    : `✅ ${rowLabel.value} created successfully!`,
)
const insertAnotherLabel = computed(() =>
  isFromColumnAddition.value ? '➕ Add Another Column' : `➕ Add Another ${rowLabel.value}`,
)
const returnToTableLabel = computed(() =>
  tableListLinkLabel(rowNamePlural.value, rowLabel.value),
)

onMounted(async () => {
  if (document.referrer.includes('/add-column')) {
    isFromColumnAddition.value = true
  }
  try {
    const structure = await client.getTableStructure({ pageId: tableId })
    rowNamePlural.value = structure.rowNamePlural?.trim() || null
    if (!isFromColumnAddition.value) {
      const name = rowNameLabel(structure.rowName)
      if (name) {
        rowLabel.value = name
      }
    }
  } catch (error) {
    console.warn('Failed to load row label for table:', error)
  }
})

function insertAnother() {
  if (isFromColumnAddition.value) {
    router.push({ name: 'add-column', params: { tableName: tableId } })
  } else {
    router.push({ name: 'insert-row', params: { tableName: tableId } })
  }
}

function returnToTable() {
  router.push({ name: 'table', params: { tableName: tableId } })
}

function returnToOriginRow() {
  if (fromTable && fromRowId) {
    router.push({ name: 'row', params: { tableName: fromTable, rowId: fromRowId } })
  }
}

function returnToDashboard() {
  if (dashboardName) {
    router.push({ name: 'dashboard', params: { dashboardName } })
  }
}
</script>

<template>
  <Section :title="addedTitle" :icon="CheckmarkSquare03Icon">
    <div class="success-message">
      <h3>{{ addedMessage }}</h3>
      <p>What would you like to do next?</p>
    </div>

    <div class="action-row center action-buttons">
      <button type="button" class="button neutral" @click="returnToTable">
        📋 {{ returnToTableLabel }}
      </button>
      <button type="button" class="button neutral" @click="insertAnother">
        {{ insertAnotherLabel }}
      </button>
      <button v-if="dashboardName" type="button" class="button neutral" @click="returnToDashboard">
        📊 Return to Dashboard
      </button>
      <button v-if="fromTable && fromRowId" type="button" class="button neutral" @click="returnToOriginRow">
        🔙 Back to Row
      </button>
    </div>
  </Section>
</template>

<style scoped>
.success-message {
  text-align: center;
  margin-bottom: 2rem;
}

.success-message h3 {
  color: #28a745;
  margin: 0 0 0.5rem 0;
  font-size: 1.5rem;
}

.success-message p {
  color: #666;
  margin: 0;
  font-size: 1.1rem;
}

/* picocrank .action-row stacks the buttons on narrow screens; keep them a readable width. */
@media (max-width: 768px) {
  .action-buttons {
    align-items: center;
  }

  .action-buttons > .button {
    max-width: 300px;
  }
}
</style>
