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

</script>

<template>
  <div class="control-panel">
    <div v-if="error" class="error-message">
      {{ error }}
    </div>

    <div class="control-sections">
      <!-- Control Panel -->
      <Section title = "Control Panel">
        <div class="quick-actions-grid">
          <router-link to="/admin/table/create" class="quick-action-card">
            <div class="card-icon">
              <HugeiconsIcon :icon="AddIcon" />
            </div>
            <div class="card-content">
              <h3>Create New Table</h3>
              <p>Create a new database table</p>
            </div>
            <div class="card-arrow">
              →
            </div>
          </router-link>
          <router-link to="/admin/database-browser" class="quick-action-card">
            <div class="card-icon">
              <HugeiconsIcon :icon="DatabaseIcon" />
            </div>
            <div class="card-content">
              <h3>Database Browser</h3>
              <p>Browse and explore database structure</p>
            </div>
            <div class="card-arrow">
              →
            </div>
          </router-link>
          <router-link to="/admin/user-management" class="quick-action-card">
            <div class="card-icon">
              <HugeiconsIcon :icon="UserIcon" />
            </div>
            <div class="card-content">
              <h3>User Management</h3>
              <p>Reset user passwords and manage user accounts</p>
            </div>
            <div class="card-arrow">
              →
            </div>
          </router-link>
          <router-link to="/table/table_sessions" class="quick-action-card">
            <div class="card-icon">
              <HugeiconsIcon :icon="UserIcon" />
            </div>
            <div class="card-content">
              <h3>View Sessions</h3>
              <p>View active user sessions</p>
            </div>
            <div class="card-arrow">
              →
            </div>
          </router-link>
          <router-link to="/table/device_codes" class="quick-action-card">
            <div class="card-icon">
              <HugeiconsIcon :icon="KeyIcon" />
            </div>
            <div class="card-content">
              <h3>View Device Codes</h3>
              <p>Manage device authentication codes</p>
            </div>
            <div class="card-arrow">
              →
            </div>
          </router-link>
          <button @click="refreshData" class="quick-action-card" :disabled="loading">
            <div class="card-icon">
              <HugeiconsIcon :icon="RefreshIcon" />
            </div>
            <div class="card-content">
              <h3>Refresh All Data</h3>
              <p>{{ loading ? 'Refreshing...' : 'Reload system information' }}</p>
            </div>
            <div class="card-arrow">
              →
            </div>
          </button>
          <button @click="router.push('/')" class="quick-action-card">
            <div class="card-icon">
              <HugeiconsIcon :icon="HomeIcon" />
            </div>
            <div class="card-content">
              <h3>Go to Home</h3>
              <p>Return to the home dashboard</p>
            </div>
            <div class="card-arrow">
              →
            </div>
          </button>
        </div>
      </Section>

      <!-- System Diagnostics -->
      <Section title = "System Diagnostics">
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

      <!-- Database Diagnostics -->
      <Section title = "Database Diagnostics">
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
    </div>
  </div>
</template>

<style scoped>
.control-panel {
  max-width: 1200px;
  margin: 0 auto;
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

.quick-actions-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 1.5rem;
}

.quick-action-card {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  padding: 1.5rem;
  background: white;
  border: 1px solid #e9ecef;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s ease;
  position: relative;
  min-height: 160px;
  text-decoration: none;
  color: inherit;
  margin: 0;
  font-family: inherit;
  font-size: inherit;
  text-align: left;
  width: 100%;
  box-sizing: border-box;
  text-indent: 0;
  -webkit-appearance: none;
  -moz-appearance: none;
  appearance: none;
}

.quick-action-card:hover:not(:disabled) {
  border-color: #007bff;
  box-shadow: 0 4px 12px rgba(0, 123, 255, 0.15);
  transform: translateY(-2px);
}

.quick-action-card:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.quick-action-card .card-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 48px;
  height: 48px;
  background: #f8f9fa;
  border-radius: 8px;
  margin-bottom: 1rem;
  color: #007bff;
}

.quick-action-card .card-icon :deep(svg) {
  width: 24px;
  height: 24px;
}

.quick-action-card .card-content {
  flex: 1;
}

.quick-action-card .card-content h3 {
  margin: 0 0 0.5rem 0;
  color: #212529;
  font-size: 1.125rem;
  font-weight: 600;
}

.quick-action-card .card-content p {
  margin: 0;
  color: #6c757d;
  font-size: 0.9rem;
  line-height: 1.5;
}

.quick-action-card .card-arrow {
  position: absolute;
  top: 1.5rem;
  right: 1.5rem;
  color: #6c757d;
  font-size: 1.25rem;
  font-weight: 300;
  transition: all 0.2s ease;
}

.quick-action-card:hover:not(:disabled) .card-arrow {
  color: #007bff;
  transform: translateX(4px);
}

.stat-display .subtle {
  letter-spacing: 1px;
  text-transform: uppercase;
}

.stat-display {
  text-align: center;
}

@media (max-width: 768px) {
  .control-panel-header {
    flex-direction: column;
    gap: 15px;
    align-items: stretch;
  }

  .form-grid {
    grid-template-columns: 1fr;
  }

  .quick-actions-grid {
    grid-template-columns: 1fr;
  }
}
</style>
