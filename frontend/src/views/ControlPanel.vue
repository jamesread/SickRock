<script setup lang="ts">
import { ref, onMounted } from 'vue'

import { createApiClient } from '../stores/api'
import Section from 'picocrank/vue/components/Section.vue'
import Navigation from 'picocrank/vue/components/Navigation.vue'
import NavigationGrid from 'picocrank/vue/components/NavigationGrid.vue'
import StatusCard from 'picocrank/vue/components/StatusCard.vue'
import {
  AddIcon,
  HomeIcon,
  KeyIcon,
  DatabaseIcon,
  DatabaseSettingIcon,
  WebSecurityIcon,
} from '@hugeicons/core-free-icons'
import { canAccessIam } from '../utils/rbacAccess'
import { useAuthStore } from '../stores/auth'

const authStore = useAuthStore()
const client = createApiClient()

const version = ref<string>('')
const commit = ref<string>('')
const buildDate = ref<string>('')
const dbName = ref<string>('')
const error = ref<string | null>(null)

const totalTables = ref<number>(0)
const totalItems = ref<number>(0)

const localNavigation = ref(null)

onMounted(async () => {
  await loadBuildInfo()
  await loadDatabaseStats()

  if (localNavigation.value) {
    localNavigation.value.addNavigationLink({
      id: 'create-table',
      name: 'create-table',
      title: 'Create New Table',
      path: '/admin/table/create',
      icon: AddIcon,
      type: 'route',
      description: 'Create a new database table'
    })

    localNavigation.value.addNavigationLink({
      id: 'database-browser',
      name: 'database-browser',
      title: 'Database Browser',
      path: '/admin/database-browser',
      icon: DatabaseIcon,
      type: 'route',
      description: 'Browse and explore database structure'
    })

    const perms = authStore.user?.rbacPermissions ?? authStore.initResponse?.rbacPermissions ?? []
    const superuser = authStore.user?.rbacIsSuperuser ?? authStore.initResponse?.rbacIsSuperuser ?? false
    if (canAccessIam(perms, superuser)) {
      localNavigation.value.addNavigationLink({
        id: 'iam-hub',
        name: 'iam-hub',
        title: 'Identity & Access',
        path: '/admin/iam',
        icon: WebSecurityIcon,
        type: 'route',
        description: 'Manage users, groups, roles, and permissions',
      })
    }

    localNavigation.value.addNavigationLink({
      id: 'view-device-codes',
      name: 'view-device-codes',
      title: 'View Device Codes',
      path: '/table/device_codes',
      icon: KeyIcon,
      type: 'route',
      description: 'Manage device authentication codes'
    })

    localNavigation.value.addNavigationLink({
      id: 'settings',
      name: 'settings',
      title: 'Settings',
      path: '/table/table_settings',
      icon: DatabaseSettingIcon,
      type: 'route',
      description: 'Manage application settings'
    })

    localNavigation.value.addNavigationLink({
      id: 'nav-items',
      name: 'nav-items',
      title: 'Navigation',
      path: '/table/table_navigation',
      icon: DatabaseSettingIcon,
      type: 'route',
      description: 'Manage navigation items'
    })

    localNavigation.value.addNavigationLink({
      id: 'go-home',
      name: 'go-home',
      title: 'Go to Home',
      path: '/',
      icon: HomeIcon,
      type: 'route',
      description: 'Return to the home dashboard'
    })
  }
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

    const sys = await client.getSystemInfo({})
    totalItems.value = Number(sys.approxTotalRows || 0)
  } catch (err) {
    console.error('Failed to load database stats:', err)
  }
}
</script>

<template>
  <div class="control-panel">
    <div v-if="error" class="error-message">
      {{ error }}
    </div>

    <div class="control-sections">
      <Section title="Control Panel" subtitle="Administrative tools and quick actions">
        <Navigation ref="localNavigation">
          <NavigationGrid />
        </Navigation>
      </Section>

      <Section title="System Diagnostics">
        <div class="grid-boxed">
          <StatusCard karma="info">
            <h4>Version</h4>
            <span class="stat">{{ version || '—' }}</span>
          </StatusCard>
          <StatusCard karma="info">
            <h4>Commit</h4>
            <span class="stat">{{ commit || '—' }}</span>
          </StatusCard>
          <StatusCard karma="info">
            <h4>Build Date</h4>
            <span class="stat">{{ buildDate || '—' }}</span>
          </StatusCard>
          <StatusCard karma="info">
            <h4>Database</h4>
            <span class="stat">{{ dbName || 'Unknown' }}</span>
          </StatusCard>
        </div>
      </Section>

      <Section title="Database Diagnostics">
        <div class="grid-boxed">
          <StatusCard karma="good">
            <h4>Total Tables</h4>
            <span class="stat">{{ totalTables }}</span>
          </StatusCard>
          <StatusCard karma="good">
            <h4>Total Items</h4>
            <span class="stat">{{ totalItems }}</span>
          </StatusCard>
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

.error-message {
  background: var(--karma-bad, #f8d7da);
  color: var(--karma-on-bg-fg, #721c24);
  padding: 15px;
  border-radius: 5px;
  margin-bottom: 20px;
  border: 1px solid var(--karma-bad-border, #f5c6cb);
}

.control-sections {
  display: grid;
  gap: 30px;
}
</style>
