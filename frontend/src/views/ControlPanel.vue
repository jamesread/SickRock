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
  HomeIcon,
  UserIcon,
  KeyIcon,
  DatabaseIcon
} from '@hugeicons/core-free-icons'

const router = useRouter()
// Transport handled by authenticated client
const client = createApiClient()

// State
const version = ref<string>('')
const commit = ref<string>('')
const buildDate = ref<string>('')
const dbName = ref<string>('')
const loading = ref(false)
const error = ref<string | null>(null)
const resetUser = ref({ username: '', newPassword: '' })
const resetStatus = ref<string>('')

// Database stats
const totalTables = ref<number>(0)
const totalItems = ref<number>(0)

// Load initial data
onMounted(async () => {
  await loadBuildInfo()
  await loadDatabaseStats()
})

async function loadBuildInfo() {
  try {
    const response = await client.init({})
    version.value = response.version
    commit.value = response.commit
    buildDate.value = response.date
    dbName.value = response.dbName || ''
  } catch (err) {
    console.error('Failed to load build info:', err)
  }
}

async function loadDatabaseStats() {
  try {
    const pages = await client.getTableConfigurations({})
    totalTables.value = pages.pages.length

    // Use efficient system info endpoint for approximate total rows
    const sys = await client.getSystemInfo({})
    totalItems.value = Number(sys.approxTotalRows || 0)
  } catch (err) {
    console.error('Failed to load database stats:', err)
  }
}


function refreshData() {
  loading.value = true
  error.value = null

  Promise.all([
    loadBuildInfo(),
    loadDatabaseStats()
  ]).finally(() => {
    loading.value = false
  })
}

async function resetUserPassword() {
  resetStatus.value = ''
  error.value = null
  try {
    if (!resetUser.value.username || !resetUser.value.newPassword) {
      error.value = 'Username and new password are required'
      return
    }
    const resp = await client.resetUserPassword({
      username: resetUser.value.username,
      newPassword: resetUser.value.newPassword
    } as any)
    if ((resp as any).success) {
      resetStatus.value = 'Password updated'
      resetUser.value = { username: '', newPassword: '' }
    } else {
      error.value = (resp as any).message || 'Failed to update password'
    }
  } catch (e) {
    console.error(e)
    error.value = 'Network error'
  }
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
        <div class="stats-grid">
          <div class="stat-card">
            <div class="stat-number">{{ version || '—' }}</div>
            <div class="stat-label">Version</div>
          </div>
          <div class="stat-card">
            <div class="stat-number">{{ commit || '—' }}</div>
            <div class="stat-label">Commit</div>
          </div>
          <div class="stat-card">
            <div class="stat-number">{{ buildDate || '—' }}</div>
            <div class="stat-label">Build Date</div>
          </div>
          <div class="stat-card">
            <div class="stat-number">{{ dbName || 'Unknown' }}</div>
            <div class="stat-label">Database</div>
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

      <!-- Quick Actions -->
      <Section title = "Quick Actions">
        <div class="quick-actions">
          <router-link to="/admin/table/create" class="button">
            <HugeiconsIcon :icon="AddIcon" width="18" height="18" class="action-icon" />
            Create New Table
          </router-link>
          <router-link to="/admin/database-browser" class="button">
            <HugeiconsIcon :icon="DatabaseIcon" width="18" height="18" class="action-icon" />
            Database Browser
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

      <!-- User Management -->
      <Section title = "User Management">
        <div class="add-rule-form">
          <h3>Reset User Password</h3>
          <div class="form-grid">
            <div class="form-group">
              <label>Username:</label>
              <input v-model="resetUser.username" type="text" placeholder="e.g., admin" />
            </div>
            <div class="form-group">
              <label>New Password:</label>
              <input v-model="resetUser.newPassword" type="password" placeholder="new password" />
            </div>
          </div>
          <div class="form-actions">
            <button @click="resetUserPassword" class="save-btn">Reset Password</button>
            <span v-if="resetStatus" class="subtle">{{ resetStatus }}</span>
          </div>
        </div>
      </Section>

      <!-- System Tables -->
      <Section title = "System Tables">
        <div class="quick-actions">
          <router-link to="/table/table_sessions" class="button">
            <HugeiconsIcon :icon="UserIcon" width="18" height="18" class="action-icon" />
            View Sessions
          </router-link>
          <router-link to="/table/device_codes" class="button">
            <HugeiconsIcon :icon="KeyIcon" width="18" height="18" class="action-icon" />
            View Device Codes
          </router-link>
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
