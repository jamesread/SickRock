<script setup lang="ts">
import { ref, onMounted } from 'vue'

import { createApiClient } from '../stores/api'
import { SickRock } from '../gen/sickrock_pb'
import { useRouter } from 'vue-router'
import Section from 'picocrank/vue/components/Section.vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import {
  RefreshIcon,
  AddIcon,
  BashIcon,
  HomeIcon
} from '@hugeicons/core-free-icons'

const router = useRouter()
// Transport handled by authenticated client
const client = createApiClient()

// State
const version = ref<string>('')
const commit = ref<string>('')
const buildDate = ref<string>('')
const loading = ref(false)
const error = ref<string | null>(null)

// Database stats
const totalTables = ref<number>(0)
const totalItems = ref<number>(0)

// Conditional formatting rules
const formattingRules = ref<any[]>([])
const showAddRuleForm = ref(false)
const newRule = ref({
  tableName: '',
  columnName: '',
  conditionType: 'equals',
  conditionValue: '',
  formatType: 'color',
  formatValue: '#ff0000',
  priority: 0
})

// Load initial data
onMounted(async () => {
  await loadBuildInfo()
  await loadDatabaseStats()
  await loadFormattingRules()
})

async function loadBuildInfo() {
  try {
    const response = await client.init({})
    version.value = response.version
    commit.value = response.commit
    buildDate.value = response.date
  } catch (err) {
    console.error('Failed to load build info:', err)
  }
}

async function loadDatabaseStats() {
  try {
    const pages = await client.getPages({})
    totalTables.value = pages.pages.length

    // Count total items across all tables
    let total = 0
    for (const page of pages.pages) {
      try {
        const items = await client.listItems({ pageId: page.slug })
        total += items.items.length
      } catch (err) {
        console.warn(`Failed to count items for table ${page.slug}:`, err)
      }
    }
    totalItems.value = total
  } catch (err) {
    console.error('Failed to load database stats:', err)
  }
}

async function loadFormattingRules() {
  // TODO: Implement when we have the RPC method
  // For now, just show placeholder data
  formattingRules.value = [
    {
      id: 1,
      tableName: 'computers',
      columnName: 'status',
      conditionType: 'equals',
      conditionValue: 'active',
      formatType: 'color',
      formatValue: '#00ff00',
      priority: 1,
      isActive: true
    }
  ]
}

function addFormattingRule() {
  if (!newRule.value.tableName || !newRule.value.columnName) {
    error.value = 'Table name and column name are required'
    return
  }

  // TODO: Implement when we have the RPC method
  console.log('Adding formatting rule:', newRule.value)

  // Reset form
  newRule.value = {
    tableName: '',
    columnName: '',
    conditionType: 'equals',
    conditionValue: '',
    formatType: 'color',
    formatValue: '#ff0000',
    priority: 0
  }
  showAddRuleForm.value = false
}

function deleteFormattingRule(id: number) {
  // TODO: Implement when we have the RPC method
  console.log('Deleting formatting rule:', id)
  formattingRules.value = formattingRules.value.filter(rule => rule.id !== id)
}

function refreshData() {
  loading.value = true
  error.value = null

  Promise.all([
    loadBuildInfo(),
    loadDatabaseStats(),
    loadFormattingRules()
  ]).finally(() => {
    loading.value = false
  })
}
</script>

<template>
  <div class="control-panel">
    <Section title = "Control Panel">
        <div class = "section-content">
            <h2>System Information</h2>
        </div>
              <button @click="refreshData" :disabled="loading" class="refresh-btn">
          <HugeiconsIcon :icon="RefreshIcon" width="16" height="16" />
          {{ loading ? 'Refreshing...' : 'Refresh' }}
        </button>

    </Section>

    <div v-if="error" class="error-message">
      {{ error }}
    </div>

    <div class="control-sections">
      <!-- System Information -->
      <Section title = "System Information">
        <div class="grid-boxed">
          <div class = "stat-display">
            <span class = "subtle">Version</span>
            <span class = "stat">{{ version || 'Loading...' }}</span>
          </div>
          <div class="stat-display">
            <span class = "subtle">Commit</span>
            <span class = "stat">{{ commit || 'Loading...' }}</span>
          </div>
          <div class="stat-display">
            <span class = "subtle">Build Date</span>
            <span class = "stat">{{ buildDate || 'Loading...' }}</span>
          </div>
        </div>
      </Section>

      <!-- Database Statistics -->
      <Section title = "Database Statistics">
        <div class="stats-grid">
          <div class="stat-card">
            <div class="stat-number">{{ totalTables }}</div>
            <div class="stat-label">Total Tables</div>
          </div>
          <div class="stat-card">
            <div class="stat-number">{{ totalItems }}</div>
            <div class="stat-label">Total Items</div>
          </div>
        </div>
      </Section>

      <!-- Conditional Formatting Rules -->
      <Section title = "Conditional Formatting Rules">
        <template #toolbar>
          <button @click="showAddRuleForm = !showAddRuleForm" class="add-btn">
            {{ showAddRuleForm ? 'Cancel' : '+ Add Rule' }}
          </button>
        </template>

        <!-- Add Rule Form -->
        <div v-if="showAddRuleForm" class="add-rule-form">
          <h3>Add New Formatting Rule</h3>
          <div class="form-grid">
            <div class="form-group">
              <label>Table Name:</label>
              <input v-model="newRule.tableName" type="text" placeholder="e.g., computers" />
            </div>
            <div class="form-group">
              <label>Column Name:</label>
              <input v-model="newRule.columnName" type="text" placeholder="e.g., status" />
            </div>
            <div class="form-group">
              <label>Condition Type:</label>
              <select v-model="newRule.conditionType">
                <option value="equals">Equals</option>
                <option value="contains">Contains</option>
                <option value="greater_than">Greater Than</option>
                <option value="less_than">Less Than</option>
              </select>
            </div>
            <div class="form-group">
              <label>Condition Value:</label>
              <input v-model="newRule.conditionValue" type="text" placeholder="e.g., active" />
            </div>
            <div class="form-group">
              <label>Format Type:</label>
              <select v-model="newRule.formatType">
                <option value="color">Background Color</option>
                <option value="text_color">Text Color</option>
                <option value="bold">Bold</option>
                <option value="italic">Italic</option>
              </select>
            </div>
            <div class="form-group">
              <label>Format Value:</label>
              <input v-model="newRule.formatValue" type="text" placeholder="e.g., #ff0000" />
            </div>
            <div class="form-group">
              <label>Priority:</label>
              <input v-model.number="newRule.priority" type="number" min="0" />
            </div>
          </div>
          <div class="form-actions">
            <button @click="addFormattingRule" class="save-btn">Save Rule</button>
            <button @click="showAddRuleForm = false" class="cancel-btn">Cancel</button>
          </div>
        </div>

        <!-- Rules List -->
        <div class="rules-list">
          <div v-if="formattingRules.length === 0" class="empty-state">
            No formatting rules configured
          </div>
          <div v-else>
            <div v-for="rule in formattingRules" :key="rule.id" class="rule-item">
              <div class="rule-info">
                <div class="rule-title">{{ rule.tableName }}.{{ rule.columnName }}</div>
                <div class="rule-details">
                  {{ rule.conditionType }} "{{ rule.conditionValue }}" → {{ rule.formatType }}: {{ rule.formatValue }}
                </div>
                <div class="rule-meta">
                  Priority: {{ rule.priority }} | {{ rule.isActive ? 'Active' : 'Inactive' }}
                </div>
              </div>
              <div class="rule-actions">
                <button @click="deleteFormattingRule(rule.id)" class="delete-btn">
                  <HugeiconsIcon :icon="BashIcon" width="16" height="16" />
                </button>
              </div>
            </div>
          </div>
        </div>
      </Section>

      <!-- Quick Actions -->
      <Section title = "Quick Actions">
        <div class="quick-actions">
          <router-link to="/admin/table/create" class="button">
            <HugeiconsIcon :icon="AddIcon" width="18" height="18" class="action-icon" />
            Create New Table
          </router-link>
          <button @click="refreshData" class="button" :disabled="loading">
            <HugeiconsIcon :icon="RefreshIcon" width="18" height="18" class="action-icon" />
            Refresh All Data
          </button>
          <button @click="router.push('/')" class="button">
            <HugeiconsIcon :icon="HomeIcon" width="18" height="18" class="action-icon" />
            Go to Home
          </button>
        </div>
      </Section>
    </div>
  </div>
</template>

<style scoped>
.control-panel {
  max-width: 1200px;
  margin: 0 auto;
  padding: 20px;
}

.control-panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 30px;
  padding-bottom: 20px;
  border-bottom: 2px solid #e0e0e0;
}

.control-panel-header h1 {
  margin: 0;
  color: #333;
}

.refresh-btn {
  background: #007bff;
  color: white;
  border: none;
  padding: 10px 20px;
  border-radius: 5px;
  cursor: pointer;
  font-size: 14px;
}

.refresh-btn:disabled {
  background: #ccc;
  cursor: not-allowed;
}

.error-message {
  background: #f8d7da;
  color: #721c24;
  padding: 15px;
  border-radius: 5px;
  margin-bottom: 20px;
  border: 1px solid #f5c6cb;
}

.control-sections {
  display: grid;
  gap: 30px;
}

.control-section {
  background: white;
  border: 1px solid #e0e0e0;
  border-radius: 8px;
  padding: 20px;
  box-shadow: 0 2px 4px rgba(0,0,0,0.1);
}

.control-section h2 {
  margin: 0 0 20px 0;
  color: #333;
  font-size: 1.5em;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.add-btn {
  background: #28a745;
  color: white;
  border: none;
  padding: 8px 16px;
  border-radius: 4px;
  cursor: pointer;
  font-size: 14px;
}

.info-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 15px;
}

.info-item {
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.info-item label {
  font-weight: bold;
  color: #666;
  font-size: 14px;
}

.info-item span {
  color: #333;
  font-size: 16px;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 20px;
}

.stat-card {
  text-align: center;
  padding: 20px;
  background: #f8f9fa;
  border-radius: 8px;
  border: 1px solid #e9ecef;
}

.stat-number {
  font-size: 2.5em;
  font-weight: bold;
  color: #007bff;
  margin-bottom: 5px;
}

.stat-label {
  color: #666;
  font-size: 14px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.add-rule-form {
  background: #f8f9fa;
  padding: 20px;
  border-radius: 8px;
  margin-bottom: 20px;
  border: 1px solid #e9ecef;
}

.add-rule-form h3 {
  margin: 0 0 20px 0;
  color: #333;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 15px;
  margin-bottom: 20px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.form-group label {
  font-weight: bold;
  color: #666;
  font-size: 14px;
}

.form-group input,
.form-group select {
  padding: 8px 12px;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-size: 14px;
}

.form-actions {
  display: flex;
  gap: 10px;
}

.save-btn {
  background: #28a745;
  color: white;
  border: none;
  padding: 10px 20px;
  border-radius: 4px;
  cursor: pointer;
}

.cancel-btn {
  background: #6c757d;
  color: white;
  border: none;
  padding: 10px 20px;
  border-radius: 4px;
  cursor: pointer;
}

.rules-list {
  max-height: 400px;
  overflow-y: auto;
}

.empty-state {
  text-align: center;
  color: #666;
  padding: 40px;
  font-style: italic;
}

.rule-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 15px;
  border: 1px solid #e0e0e0;
  border-radius: 6px;
  margin-bottom: 10px;
  background: white;
}

.rule-info {
  flex: 1;
}

.rule-title {
  font-weight: bold;
  color: #333;
  margin-bottom: 5px;
}

.rule-details {
  color: #666;
  font-size: 14px;
  margin-bottom: 3px;
}

.rule-meta {
  color: #999;
  font-size: 12px;
}

.rule-actions {
  display: flex;
  gap: 5px;
}

.delete-btn {
  background: #dc3545;
  color: white;
  border: none;
  padding: 5px 10px;
  border-radius: 4px;
  cursor: pointer;
  font-size: 12px;
}

.quick-actions {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 15px;
}

.button {
  padding: 1rem;
  display: flex;
  gap: .5em;
  align-items: center;
  justify-content: center;
  box-shadow: 0 1px 3px 0 rgba(0, 0, 0, 0.1), 0 1px 2px 0 rgba(0, 0, 0, 0.06);
  border: 0;
}

.action-icon {
  font-size: 18px;
}

.stat-display .subtle {
  letter-spacing: 1px;
  text-transform: uppercase;
}

.stat-display {
  text-align: center;
}

@media (max-width: 768px) {
  .control-panel {
    padding: 10px;
  }

  .control-panel-header {
    flex-direction: column;
    gap: 15px;
    align-items: stretch;
  }

  .form-grid {
    grid-template-columns: 1fr;
  }

  .quick-actions {
    grid-template-columns: 1fr;
  }
}
</style>
