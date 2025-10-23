<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'

import { createApiClient } from '../stores/api'
import Section from 'picocrank/vue/components/Section.vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { Delete01Icon, ArrowLeft01Icon, Edit03Icon } from '@hugeicons/core-free-icons'

const route = useRoute()
const tableId = route.params.tableName as string

// Transport handled by authenticated client
const client = createApiClient()

// State
const formattingRules = ref<any[]>([])
const showAddRuleForm = ref(false)
const showEditRuleForm = ref(false)
const editingRuleId = ref<number | null>(null)
const loading = ref(false)
const error = ref<string | null>(null)
const columns = ref<Array<{ name: string; type: string; required: boolean }>>([])

const newRule = ref({
  tableName: '',
  columnName: '',
  conditionType: 'equals',
  conditionValue: '',
  formatType: 'color',
  formatValue: '#ff0000',
  priority: 0
})

// Load table columns for the dropdown
async function loadColumns() {
  try {
    const response = await client.getTableStructure({ pageId: tableId })
    columns.value = response.fields?.map(field => ({
      name: field.name,
      type: field.type,
      required: !!field.required
    })) || []
  } catch (err) {
    console.error('Failed to load columns:', err)
    columns.value = []
  }
}

// Conditional formatting functions
async function loadFormattingRules() {
  try {
    const response = await client.getConditionalFormattingRules({ tableName: tableId })
    formattingRules.value = response.rules.map(rule => ({
      id: rule.id,
      tableName: rule.tableName,
      columnName: rule.columnName,
      conditionType: rule.conditionType,
      conditionValue: rule.conditionValue,
      formatType: rule.formatType,
      formatValue: rule.formatValue,
      priority: rule.priority,
      isActive: rule.isActive
    }))
  } catch (err) {
    console.error('Failed to load formatting rules:', err)
    formattingRules.value = []
  }
}

function addFormattingRule() {
  if (!newRule.value.columnName) {
    error.value = 'Column name is required'
    return
  }

  // For "always" condition, set condition value to empty string
  const conditionValue = newRule.value.conditionType === 'always' ? '' : newRule.value.conditionValue

  loading.value = true
  error.value = null

  client.createConditionalFormattingRule({
    tableName: tableId,
    columnName: newRule.value.columnName,
    conditionType: newRule.value.conditionType,
    conditionValue: conditionValue,
    formatType: newRule.value.formatType,
    formatValue: newRule.value.formatValue,
    priority: newRule.value.priority
  }).then(() => {
    // Reload rules after successful creation
    loadFormattingRules()
    // Reset form
    newRule.value = {
      tableName: tableId,
      columnName: '',
      conditionType: 'equals',
      conditionValue: '',
      formatType: 'color',
      formatValue: '#ff0000',
      priority: 0
    }
    showAddRuleForm.value = false
  }).catch(err => {
    error.value = `Failed to create formatting rule: ${err}`
  }).finally(() => {
    loading.value = false
  })
}

function deleteFormattingRule(id: number) {
  loading.value = true
  error.value = null

  client.deleteConditionalFormattingRule({ ruleId: id }).then(() => {
    // Reload rules after successful deletion
    loadFormattingRules()
  }).catch(err => {
    error.value = `Failed to delete formatting rule: ${err}`
  }).finally(() => {
    loading.value = false
  })
}

function startEditRule(rule: any) {
  editingRuleId.value = rule.id
  newRule.value = {
    tableName: rule.tableName,
    columnName: rule.columnName,
    conditionType: rule.conditionType,
    conditionValue: rule.conditionValue,
    formatType: rule.formatType,
    formatValue: rule.formatValue,
    priority: rule.priority
  }
  showEditRuleForm.value = true
  showAddRuleForm.value = false
}

function cancelEditRule() {
  showEditRuleForm.value = false
  editingRuleId.value = null
  newRule.value = {
    tableName: tableId,
    columnName: '',
    conditionType: 'equals',
    conditionValue: '',
    formatType: 'color',
    formatValue: '#ff0000',
    priority: 0
  }
}

function updateFormattingRule() {
  if (!newRule.value.columnName) {
    error.value = 'Column name is required'
    return
  }

  // For "always" condition, set condition value to empty string
  const conditionValue = newRule.value.conditionType === 'always' ? '' : newRule.value.conditionValue

  loading.value = true
  error.value = null

  client.updateConditionalFormattingRule({
    ruleId: editingRuleId.value!,
    tableName: tableId,
    columnName: newRule.value.columnName,
    conditionType: newRule.value.conditionType,
    conditionValue: conditionValue,
    formatType: newRule.value.formatType,
    formatValue: newRule.value.formatValue,
    priority: newRule.value.priority,
    isActive: true
  }).then(() => {
    // Reload rules after successful update
    loadFormattingRules()
    // Reset form and close edit mode
    cancelEditRule()
  }).catch(err => {
    error.value = `Failed to update formatting rule: ${err}`
  }).finally(() => {
    loading.value = false
  })
}

function getFormatValuePlaceholder(formatType: string): string {
  switch (formatType) {
    case 'color':
      return 'e.g., #ff0000'
    case 'text_color':
      return 'e.g., #0000ff'
    case 'bold':
      return 'true or false'
    case 'italic':
      return 'true or false'
    case 'markdown':
      return 'Not needed for markdown format'
    default:
      return 'e.g., #ff0000'
  }
}

function getFormatDisplayText(formatType: string, formatValue: string): string {
  switch (formatType) {
    case 'color':
      return `Background Color: ${formatValue}`
    case 'text_color':
      return `Text Color: ${formatValue}`
    case 'bold':
      return `Bold: ${formatValue}`
    case 'italic':
      return `Italic: ${formatValue}`
    case 'markdown':
      return `Markdown${formatValue ? `: ${formatValue}` : ''}`
    default:
      return `${formatType}: ${formatValue}`
  }
}

// Lifecycle
onMounted(async () => {
  await Promise.all([
    loadColumns(),
    loadFormattingRules(),
  ])
  
  // Preselect column from query string
  const preselect = route.query.column as string
  if (preselect) {
    newRule.value.columnName = preselect
    showAddRuleForm.value = true
  }
})

// Watch for format type changes to clear format value when switching to markdown
watch(() => newRule.value.formatType, (newFormatType) => {
  if (newFormatType === 'markdown') {
    newRule.value.formatValue = ''
  } else if (newFormatType === 'color' || newFormatType === 'text_color') {
    // If switching to a color type and format value is not a valid hex color, set a default
    if (!newRule.value.formatValue || !/^#[0-9A-Fa-f]{6}$/.test(newRule.value.formatValue)) {
      newRule.value.formatValue = '#ff0000'
    }
  }
})
</script>

<template>
  <Section :title="`Conditional Formatting Rules: ${tableId}`">
    <template #toolbar>
      <router-link
        :to="`/table/${tableId}`"
        class="button"
      >
        <HugeiconsIcon :icon="ArrowLeft01Icon" width="16" height="16" />
        Back to Table
      </router-link>
      <router-link :to="`/table/${tableId}/column-types`" class="button neutral">
        <HugeiconsIcon :icon="Edit03Icon" />
        Structure
      </router-link>
      <button @click="showAddRuleForm = !showAddRuleForm" class="button neutral" :disabled="showEditRuleForm">
        {{ showAddRuleForm ? 'Cancel' : '+ Add Rule' }}
      </button>
    </template>

    <div v-if="error" class="error-message">
      {{ error }}
    </div>

    <!-- Add Rule Form -->
    <div v-if="showAddRuleForm" class="add-rule-form">
      <h3>Add New Formatting Rule</h3>
      <div class="form-grid">
        <div class="form-group">
          <label>Table Name:</label>
          <input type="text" :value="tableId" disabled />
        </div>
        <div class="form-group">
          <label>Column Name:</label>
          <select v-model="newRule.columnName">
            <option value="">Select a column</option>
            <option v-for="column in columns" :key="column.name" :value="column.name">
              {{ column.name }}
            </option>
          </select>
        </div>
        <div class="form-group">
          <label>Condition Type:</label>
          <select v-model="newRule.conditionType">
            <option value="always">Always</option>
            <option value="equals">Equals</option>
            <option value="contains">Contains</option>
            <option value="greater_than">Greater Than</option>
            <option value="less_than">Less Than</option>
          </select>
        </div>
        <div class="form-group">
          <label>Condition Value:</label>
          <input 
            v-model="newRule.conditionValue" 
            type="text" 
            :placeholder="newRule.conditionType === 'always' ? 'Not needed for always condition' : 'e.g., active'" 
            :disabled="newRule.conditionType === 'always'" 
          />
        </div>
        <div class="form-group">
          <label>Format Type:</label>
          <select v-model="newRule.formatType">
            <option value="color">Background Color</option>
            <option value="text_color">Text Color</option>
            <option value="bold">Bold</option>
            <option value="italic">Italic</option>
            <option value="markdown">Markdown</option>
          </select>
        </div>
        <div class="form-group">
          <label>Format Value:</label>
          <input 
            v-if="newRule.formatType === 'color' || newRule.formatType === 'text_color'"
            v-model="newRule.formatValue" 
            type="color" 
            class="color-input"
          />
          <input 
            v-else
            v-model="newRule.formatValue" 
            type="text" 
            :placeholder="getFormatValuePlaceholder(newRule.formatType)" 
            :disabled="newRule.formatType === 'markdown'"
          />
        </div>
        <div class="form-group">
          <label>Priority:</label>
          <input v-model.number="newRule.priority" type="number" min="0" />
        </div>
      </div>
      <div class="form-actions">
        <button @click="addFormattingRule" class="button good" :disabled="loading">
          {{ loading ? 'Saving...' : 'Save Rule' }}
        </button>
        <button @click="showAddRuleForm = false" class="button neutral" :disabled="loading">Cancel</button>
      </div>
    </div>

    <!-- Edit Rule Form -->
    <div v-if="showEditRuleForm" class="add-rule-form">
      <h3>Edit Formatting Rule</h3>
      <div class="form-grid">
        <div class="form-group">
          <label>Table Name:</label>
          <input type="text" :value="tableId" disabled />
        </div>
        <div class="form-group">
          <label>Column Name:</label>
          <select v-model="newRule.columnName">
            <option value="">Select a column</option>
            <option v-for="column in columns" :key="column.name" :value="column.name">
              {{ column.name }}
            </option>
          </select>
        </div>
        <div class="form-group">
          <label>Condition Type:</label>
          <select v-model="newRule.conditionType">
            <option value="always">Always</option>
            <option value="equals">Equals</option>
            <option value="contains">Contains</option>
            <option value="greater_than">Greater Than</option>
            <option value="less_than">Less Than</option>
          </select>
        </div>
        <div class="form-group">
          <label>Condition Value:</label>
          <input 
            v-model="newRule.conditionValue" 
            type="text" 
            :placeholder="newRule.conditionType === 'always' ? 'Not needed for always condition' : 'e.g., active'" 
            :disabled="newRule.conditionType === 'always'" 
          />
        </div>
        <div class="form-group">
          <label>Format Type:</label>
          <select v-model="newRule.formatType">
            <option value="color">Background Color</option>
            <option value="text_color">Text Color</option>
            <option value="bold">Bold</option>
            <option value="italic">Italic</option>
            <option value="markdown">Markdown</option>
          </select>
        </div>
        <div class="form-group">
          <label>Format Value:</label>
          <input 
            v-if="newRule.formatType === 'color' || newRule.formatType === 'text_color'"
            v-model="newRule.formatValue" 
            type="color" 
            class="color-input"
          />
          <input 
            v-else
            v-model="newRule.formatValue" 
            type="text" 
            :placeholder="getFormatValuePlaceholder(newRule.formatType)" 
            :disabled="newRule.formatType === 'markdown'"
          />
        </div>
        <div class="form-group">
          <label>Priority:</label>
          <input v-model.number="newRule.priority" type="number" min="0" />
        </div>
      </div>
      <div class="form-actions">
        <button @click="updateFormattingRule" class="button good" :disabled="loading">
          {{ loading ? 'Updating...' : 'Update Rule' }}
        </button>
        <button @click="cancelEditRule" class="button neutral" :disabled="loading">Cancel</button>
      </div>
    </div>

    <!-- Rules List -->
    <div class="rules-list">
      <div v-if="formattingRules.length === 0" class="empty-state">
        No formatting rules configured for this table
      </div>
      <div v-else>
        <div v-for="rule in formattingRules" :key="rule.id" class="rule-item">
          <div class="rule-info">
            <div class="rule-title">{{ rule.tableName }}.{{ rule.columnName }}</div>
            <div class="rule-details">
              <span v-if="rule.conditionType === 'always'">
                Always → {{ getFormatDisplayText(rule.formatType, rule.formatValue) }}
              </span>
              <span v-else>
                {{ rule.conditionType }} "{{ rule.conditionValue }}" → {{ getFormatDisplayText(rule.formatType, rule.formatValue) }}
              </span>
            </div>
            <div class="rule-meta">
              Priority: {{ rule.priority }} | {{ rule.isActive ? 'Active' : 'Inactive' }}
            </div>
          </div>
          <div class="rule-actions">
            <button @click="startEditRule(rule)" class="button small neutral" :disabled="loading">
              <HugeiconsIcon :icon="Edit03Icon" width="16" height="16" />
            </button>
            <button @click="deleteFormattingRule(rule.id)" class="button small bad" :disabled="loading">
              <HugeiconsIcon :icon="Delete01Icon" width="16" height="16" />
            </button>
          </div>
        </div>
      </div>
    </div>
  </Section>
</template>

<style scoped>
.error-message {
  background: #f8d7da;
  color: #721c24;
  padding: 1rem;
  border-radius: 4px;
  margin-bottom: 1rem;
}

.empty-state {
  text-align: center;
  padding: 2rem;
  color: #666;
}

/* Conditional Formatting Styles */
.add-rule-form {
  background: #f8f9fa;
  padding: 1.5rem;
  border-radius: 8px;
  margin-bottom: 1rem;
  border: 1px solid #e9ecef;
}

.add-rule-form h3 {
  margin: 0 0 1rem 0;
  color: #333;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 1rem;
  margin-bottom: 1rem;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.form-group label {
  font-weight: 600;
  color: #555;
  font-size: 0.9rem;
}

.form-group input,
.form-group select {
  padding: 0.5rem;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-size: 0.9rem;
}

.form-group input:disabled {
  background: #e9ecef;
  color: #6c757d;
  cursor: not-allowed;
}

.color-input {
  width: 60px;
  height: 40px;
  padding: 0;
  border: 1px solid #ddd;
  border-radius: 4px;
  cursor: pointer;
}

.form-actions {
  display: flex;
  gap: 0.5rem;
  justify-content: flex-end;
}

.rules-list {
  margin-top: 1rem;
}

.rule-item {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  padding: 1rem;
  border: 1px solid #ddd;
  border-radius: 4px;
  margin-bottom: 0.5rem;
  background: #f9f9f9;
}

.rule-info {
  flex: 1;
  margin-right: 1rem;
}

.rule-title {
  font-weight: 600;
  color: #333;
  margin-bottom: 0.25rem;
}

.rule-details {
  color: #666;
  font-size: 0.9rem;
  margin-bottom: 0.25rem;
}

.rule-meta {
  color: #888;
  font-size: 0.8rem;
  font-style: italic;
}

.rule-actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}
</style>
