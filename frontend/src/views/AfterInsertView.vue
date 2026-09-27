<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Section from 'picocrank/vue/components/Section.vue'
import { CheckmarkSquare03Icon } from '@hugeicons/core-free-icons'

const route = useRoute()
const router = useRouter()
const tableId = route.params.tableName as string
const fromTable = route.query.fromTable as string | undefined
const fromRowId = route.query.fromRowId as string | undefined
const dashboard = route.query.dashboard as string | undefined
const dashboardName = route.query.dashboardName as string | undefined

// Check if this is from column addition or row insertion based on the referrer
const isFromColumnAddition = ref(false)

onMounted(() => {
  // Check if we came from add-column route
  if (document.referrer.includes('/add-column')) {
    isFromColumnAddition.value = true
  }
})

function insertAnother() {
  if (isFromColumnAddition.value) {
    // Navigate to add column view
    router.push({ name: 'add-column', params: { tableName: tableId } })
  } else {
    // Navigate to insert row view with the same table
    router.push({ name: 'insert-row', params: { tableName: tableId } })
  }
}

function returnToTable() {
  // Navigate back to the table view
  router.push({ name: 'table', params: { tableName: tableId } })
}

function returnToOriginRow() {
  if (fromTable && fromRowId) {
    router.push({ name: 'row', params: { tableName: fromTable, rowId: fromRowId } })
  }
}

function returnToDashboard() {
  if (dashboardName) {
    router.push({ name: 'dashboard', params: { dashboardName: dashboardName } })
  }
}
</script>

<template>
  <Section
    :title="isFromColumnAddition ? 'Column Added Successfully' : 'Row Added Successfully'"
    :icon="CheckmarkSquare03Icon"
  >
    <div class="success-message">
      <h3>{{ isFromColumnAddition ? '✅ Column added successfully!' : '✅ Row added successfully!' }}</h3>
      <p>What would you like to do next?</p>
    </div>

    <div class="action-buttons">
      <button @click="returnToTable" class="button neutral">
        📋 Return to Table
      </button>
      <button @click="insertAnother" class="button neutral">
        {{ isFromColumnAddition ? '➕ Add Another Column' : '➕ Insert Another Row' }}
      </button>
      <button v-if="dashboardName" @click="returnToDashboard" class="button neutral">
        📊 Return to Dashboard
      </button>
      <button v-if="fromTable && fromRowId" @click="returnToOriginRow" class="button neutral">
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

.action-buttons {
  display: flex;
  gap: 1rem;
  justify-content: center;
  flex-wrap: wrap;
}

@media (max-width: 768px) {
  .action-buttons {
    flex-direction: column;
    align-items: center;
  }

  .button {
    width: 100%;
    max-width: 300px;
  }
}
</style>
