<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Section from 'picocrank/vue/components/Section.vue'

const route = useRoute()
const router = useRouter()
const tableId = route.params.tableName as string

function insertAnother() {
  // Navigate to insert row view with the same table
  router.push({ name: 'insert-row', params: { tableName: tableId } })
}

function returnToTable() {
  // Navigate back to the table view
  router.push({ name: 'table', params: { tableName: tableId } })
}
</script>

<template>
  <Section title="Row Added Successfully">
    <div class="success-message">
      <h3>✅ Row added successfully!</h3>
      <p>What would you like to do next?</p>
    </div>

    <div class="action-buttons">
      <button @click="insertAnother" class="button primary">
        ➕ Insert Another Row
      </button>
      <button @click="returnToTable" class="button secondary">
        📋 Return to Table
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

.button {
  padding: 0.75rem 1.5rem;
  border: none;
  border-radius: 6px;
  font-size: 1rem;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
  text-decoration: none;
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  min-width: 180px;
  justify-content: center;
}

.button.primary {
  background-color: #007bff;
  color: white;
}

.button.primary:hover {
  background-color: #0056b3;
  transform: translateY(-1px);
  box-shadow: 0 4px 8px rgba(0, 123, 255, 0.3);
}

.button.secondary {
  background-color: #6c757d;
  color: white;
}

.button.secondary:hover {
  background-color: #545b62;
  transform: translateY(-1px);
  box-shadow: 0 4px 8px rgba(108, 117, 125, 0.3);
}

.button:active {
  transform: translateY(0);
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
