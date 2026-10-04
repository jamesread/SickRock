<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRoute } from 'vue-router'
import { inject } from 'vue'
import type { createApiClient } from '../stores/api'
import { Edit02Icon, LayoutIcon } from '@hugeicons/core-free-icons'
import Section from 'picocrank/vue/components/Section.vue'
import StatusCard from 'picocrank/vue/components/StatusCard.vue'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'
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

const addWidgetTo = computed(() => {
  if (!dashboardId.value) return ''
  return `/table/table_dashboard_components/insert-row/?dashboard=${dashboardId.value}&dashboardName=${encodeURIComponent(dashboardName.value)}`
})

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
  <Section :title="dashboardName" :icon="LayoutIcon">
    <template #toolbar>
      <div class="dashboard-actions">
        <router-link
          v-if="dashboardId"
          :to="`/table/table_dashboards/${dashboardId}`"
          class="button neutral edit-dashboard-btn"
        >
          View Dashboard
        </router-link>
        <router-link
          v-if="dashboardId && addWidgetTo"
          :to="addWidgetTo"
          class="button good add-widget-btn"
        >
          Add Widget
        </router-link>
      </div>
    </template>
    <div v-if="loading">Loading…</div>
    <div v-else>
      <NotificationBlock v-if="error" type="error" :message="error" />
      <div v-else>
        <StatusCard
          v-if="components.length === 0"
          karma="note"
          align="center"
          class="dashboard-empty"
        >
          <p>This dashboard has no widgets yet.</p>
          <p class="dashboard-empty-hint">Add a component to show stats, titles, and summaries here.</p>
          <router-link
            v-if="dashboardId && addWidgetTo"
            :to="addWidgetTo"
            class="button good"
          >
            Add Widget
          </router-link>
        </StatusCard>
        <div v-else class="stats-grid">
          <template v-for="c in components" :key="c.id">
            <div v-if="c.error" class="stat-card dashboard-widget-error">
              <router-link
                :to="`/table/table_dashboard_components/${c.id}`"
                class="edit-icon-btn"
              >
                <HugeiconsIcon :icon="Edit02Icon" />
              </router-link>
              <NotificationBlock type="error" :label="c.name" :message="c.error" />
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
.add-widget-btn,
.edit-dashboard-btn {
  text-decoration: none;
}
.meta { margin: 0.25rem 0 1rem 0; color: #6c757d; }
.title-card {
  grid-column: 1 / -1;
  font-size: 1.2em;
  font-weight: bold;
  margin-bottom: 10px;
}
.dashboard-empty {
  margin-top: 0.5rem;
}
.dashboard-empty p {
  margin: 0 0 0.75rem 0;
}
.dashboard-empty-hint {
  color: var(--femto-muted-fg, #6c757d);
  font-size: 0.95rem;
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
.dashboard-widget-error {
  grid-column: span 1;
  text-align: left;
}
.stat-number { font-size: 2.0em; font-weight: bold; color: #007bff; margin-bottom: 5px; }
.stat-label { color: #666; font-size: 14px; text-transform: uppercase; letter-spacing: 0.5px; margin-bottom: 10px; }
.stat-suffix { color: #007bff; font-size: 1.0em; font-weight: normal; }
.component-success { position: relative; }
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
</style>
