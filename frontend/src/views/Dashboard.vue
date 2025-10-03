<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { inject } from 'vue'
import type { createApiClient } from '../stores/api'
import { Edit02Icon } from '@hugeicons/core-free-icons'
import Section from 'picocrank/vue/components/Section.vue'
import { HugeiconsIcon } from '@hugeicons/vue'

const client = inject<ReturnType<typeof createApiClient>>('apiClient')
const route = useRoute()

const loading = ref(true)
const error = ref<string | null>(null)
const dashboardId = ref<number | null>(null)
const dashboardName = ref<string>('')
const components = ref<Array<{
  id: number;
  name: string;
  dataString?: string;
  dataNumber?: number;
  error?: string;
  suffix?: string;
}>>([])

async function load() {
  loading.value = true
  error.value = null
  try {
    const nameParam = String(route.params.dashboardName || '')
    dashboardName.value = nameParam
    const res = await client.getDashboards({})
    const match = (res.dashboards || []).find(d => (d.name || '') === nameParam)
    if (!match) {
      throw new Error(`Dashboard not found: ${nameParam}`)
    }
    dashboardId.value = match.id
    components.value = (match.components || []).map(c => ({
      id: c.id,
      name: c.name,
      dataString: (c as any).dataString,
      dataNumber: (c as any).dataNumber,
      error: (c as any).error,
      suffix: (c as any).suffix,
    }))
  } catch (e: any) {
    error.value = String(e?.message || e)
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <Section :title="dashboardName">
    <template #toolbar>
      <div class="dashboard-actions">
        <router-link
          v-if="dashboardId"
          :to="`/table/table_dashboards/${dashboardId}`"
          class="btn btn-outline-primary edit-dashboard-btn"
        >
          View Dashboard
        </router-link>
        <router-link
          v-if="dashboardId"
          :to="`/table/table_dashboard_components/insert-row/?dashboard=${dashboardId}&dashboardName=${encodeURIComponent(dashboardName)}`"
          class="btn btn-primary add-widget-btn"
        >
          Add Widget
        </router-link>
      </div>
    </template>
    <div v-if="loading">Loading…</div>
    <div v-else>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-else>
        <div class="stats-grid">
          <template v-for="c in components" :key="c.id">
            <div v-if="c.error" class="error-card stat-card">{{ c.name }}
              <div class="component-error">
                <div class="error-message">{{ c.error }}</div>
              </div>
              <router-link
                :to="`/table/table_dashboard_components/${c.id}`"
                class="edit-icon-btn"
              >
                <HugeiconsIcon :icon="Edit02Icon" />
              </router-link>

            </div>
            <div v-else-if="!c.dataString" class = "title-card">{{ c.name }}</div>
            <div v-else class = "stat-card">
              <router-link
                :to="`/table/table_dashboard_components/${c.id}`"
                class="edit-icon-btn"
              >
                <HugeiconsIcon :icon="Edit02Icon" />
              </router-link>

                <div class="stat-label">{{ c.name }}</div>
                <div class="stat-number">
                  {{ c.dataString ?? (c.dataNumber ?? '—') }}
                  <span v-if="c.suffix" class="stat-suffix">{{ c.suffix }}</span>
                </div>
            </div>
          </template>
        </div>
      </div>
    </div>
  </Section>
</template>

<style scoped>
.dashboard-view { max-width: 1200px; margin: 0 auto; padding: 20px; }
.dashboard-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}
.dashboard-header h2 { margin: 0; }
.dashboard-actions {
  display: flex;
  gap: 10px;
  align-items: center;
}
.add-widget-btn {
  padding: 8px 16px;
  background: #007bff;
  color: white;
  text-decoration: none;
  border-radius: 4px;
  font-size: 14px;
  transition: background-color 0.2s;
}
.add-widget-btn:hover {
  background: #0056b3;
  color: white;
  text-decoration: none;
}
.edit-dashboard-btn {
  padding: 8px 16px;
  background: transparent;
  color: #007bff;
  border: 1px solid #007bff;
  text-decoration: none;
  border-radius: 4px;
  font-size: 14px;
  transition: all 0.2s;
}
.edit-dashboard-btn:hover {
  background: #007bff;
  color: white;
  text-decoration: none;
}
.meta { margin: 0.25rem 0 1rem 0; color: #6c757d; }
.title-card {
  grid-column: 1 / -1;
  font-size: 1.2em;
  font-weight: bold;
  margin-bottom: 10px;
}
.stats-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(275px, 1fr)); gap: 20px; }
.stat-card {
  text-align: center;
  padding: 20px;
  background: #f8f9fa;
  border-radius: 8px;
  border: 1px solid #e9ecef;
  position: relative;
}
.stat-card.error-card { background: #f8d7da; border-color: #f5c6cb; }
.stat-number { font-size: 2.0em; font-weight: bold; color: #007bff; margin-bottom: 5px; }
.stat-label { color: #666; font-size: 14px; text-transform: uppercase; letter-spacing: 0.5px; margin-bottom: 10px; }
.stat-suffix { color: #007bff; font-size: 1.0em; font-weight: normal; }
.component-success { position: relative; }
.component-error { text-align: center; }
.error-message {
  color: #721c24;
  font-size: 12px;
  margin-bottom: 10px;
  word-break: break-word;
}
.edit-icon-btn {
  position: absolute;
  top: 8px;
  right: 8px;
  width: 20px;
  height: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(255, 255, 255, 0.9);
  border: 1px solid #e9ecef;
  border-radius: 4px;
  color: #6c757d;
  text-decoration: none;
  opacity: 0;
  transition: all 0.2s ease;
  cursor: pointer;
}
.edit-icon-btn:hover {
  background: #007bff;
  color: white;
  border-color: #007bff;
  text-decoration: none;
}
.stat-card:hover .edit-icon-btn {
  opacity: 1;
}
.error {
  background: #f8d7da;
  color: #721c24;
  padding: 0.75rem;
  border: 1px solid #f5c6cb;
  border-radius: 4px;
}
</style>
