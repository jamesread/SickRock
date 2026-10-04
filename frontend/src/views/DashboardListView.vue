<script setup lang="ts">
import { ref, onMounted, inject } from 'vue'
import type { createApiClient } from '../stores/api'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'

const client = inject<ReturnType<typeof createApiClient>>('apiClient')

type Dashboard = { id: number; name: string }

const loading = ref(true)
const error = ref<string | null>(null)
const dashboards = ref<Dashboard[]>([])

async function load() {
  loading.value = true
  error.value = null
  try {
    const res = await client.getDashboards({})
    dashboards.value = (res.dashboards || []).map(d => ({ id: d.id, name: d.name }))
  } catch (e: any) {
    error.value = String(e?.message || e)
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <section>
    <h2>Dashboards</h2>
    <div v-if="loading">Loading…</div>
    <div v-else>
      <NotificationBlock v-if="error" type="error" :message="error" />
      <div v-else>
        <div v-if="dashboards.length === 0" class="subtle">No dashboards yet.</div>
        <ul v-else class="dashboard-list">
          <li v-for="d in dashboards" :key="d.id" class="dashboard-item">
            <span class="dash-name">{{ d.name }}</span>
            <span class="dash-id">#{{ d.id }}</span>
          </li>
        </ul>
      </div>
    </div>
  </section>

</template>

<style scoped>
.dashboard-list {
  list-style: none;
  padding: 0;
  margin: 0.5rem 0 0 0;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}
.dashboard-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.5rem 0.75rem;
  border: 1px solid #e9ecef;
  border-radius: 6px;
  background: #f8f9fa;
}
.dash-name { font-weight: 500; }
.dash-id { color: #6c757d; font-size: 0.85rem; }
.subtle { color: #888; }
</style>
