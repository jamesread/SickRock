<script setup lang="ts">
import { ref, onMounted } from 'vue'

import { createApiClient } from '../stores/api'
import Section from 'picocrank/vue/components/Section.vue'
import Navigation from 'picocrank/vue/components/Navigation.vue'
import NavigationGrid from 'picocrank/vue/components/NavigationGrid.vue'
import StatusCard from 'picocrank/vue/components/StatusCard.vue'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'
import {
  AddIcon,
  HomeIcon,
  KeyIcon,
  DatabaseIcon,
  DatabaseSettingIcon,
  WebSecurityIcon,
  ComputerIcon,
  Calendar03Icon,
  LayoutIcon,
} from '@hugeicons/core-free-icons'
import {
  canAccessIam,
  canManageExports,
  canManageSystemSettings,
  canViewAuditLogs,
} from '../utils/rbacAccess'
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

const localNavigation = ref<InstanceType<typeof Navigation> | null>(null)

onMounted(async () => {
  await loadBuildInfo()
  await loadDatabaseStats()

  if (localNavigation.value) {
    const perms = authStore.user?.rbacPermissions ?? authStore.initResponse?.rbacPermissions ?? []
    const superuser = authStore.user?.rbacIsSuperuser ?? authStore.initResponse?.rbacIsSuperuser ?? false

    if (canManageSystemSettings(perms, superuser)) {
      localNavigation.value.addNavigationLink({
        id: 'create-table',
        name: 'create-table',
        title: 'Create New Table',
        path: '/admin/table/create',
        icon: AddIcon,
        type: 'route',
        description: 'Create a new database table',
      })

      localNavigation.value.addNavigationLink({
        id: 'database-browser',
        name: 'database-browser',
        title: 'Database Browser',
        path: '/admin/database-browser',
        icon: DatabaseIcon,
        type: 'route',
        description: 'Browse and explore database structure',
      })

      localNavigation.value.addNavigationLink({
        id: 'table-configurations',
        name: 'table-configurations',
        title: 'Table Configurations',
        path: '/table/table_configurations',
        icon: DatabaseSettingIcon,
        type: 'route',
        description: 'Manage table configurations, titles, and database mapping',
      })

      localNavigation.value.addNavigationLink({
        id: 'workflows',
        name: 'workflows',
        title: 'Workflows',
        path: '/table/table_workflows',
        icon: DatabaseSettingIcon,
        type: 'route',
        description: 'Manage workflow definitions and navigation membership',
      })

      localNavigation.value.addNavigationLink({
        id: 'dashboards',
        name: 'dashboards',
        title: 'Dashboards',
        path: '/table/table_dashboards',
        icon: LayoutIcon,
        type: 'route',
        description: 'Manage dashboard definitions and components.',
      })
    }

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

    if (canManageSystemSettings(perms, superuser)) {
      localNavigation.value.addNavigationLink({
        id: 'view-device-codes',
        name: 'view-device-codes',
        title: 'View Device Codes',
        path: '/table/device_codes',
        icon: KeyIcon,
        type: 'route',
        description: 'Manage device authentication codes',
      })

      localNavigation.value.addNavigationLink({
        id: 'settings',
        name: 'settings',
        title: 'Settings',
        path: '/table/table_settings',
        icon: DatabaseSettingIcon,
        type: 'route',
        description: 'Manage application settings',
      })

      localNavigation.value.addNavigationLink({
        id: 'nav-items',
        name: 'nav-items',
        title: 'Navigation',
        path: '/table/table_navigation',
        icon: DatabaseSettingIcon,
        type: 'route',
        description: 'Manage navigation items',
      })
    }

    if (canManageExports(perms, superuser)) {
      localNavigation.value.addNavigationLink({
        id: 'read-only-exports',
        name: 'read-only-exports',
        title: 'Read-only calendar exports',
        path: '/table/table_read_only_exports',
        icon: Calendar03Icon,
        type: 'route',
        description: 'Configure read-only calendar exports at /exports/{slug}.',
      })

      localNavigation.value.addNavigationLink({
        id: 'rss-calendar-feeds',
        name: 'rss-calendar-feeds',
        title: 'Calendar feeds',
        path: '/admin/rss-calendar-feeds',
        icon: Calendar03Icon,
        type: 'route',
        description: 'Import calendar events from feeds into tables.',
      })
    }

    if (canViewAuditLogs(perms, superuser)) {
      localNavigation.value.addNavigationLink({
        id: 'audit-logs',
        name: 'audit-logs',
        title: 'Audit logs',
        path: '/table/table_logs',
        icon: DatabaseSettingIcon,
        type: 'route',
        description: 'Security and access audit trail (login, table access, exports)',
      })
    }

    localNavigation.value.addNavigationLink({
      id: 'go-home',
      name: 'go-home',
      title: 'Go to Home',
      path: '/',
      icon: HomeIcon,
      type: 'route',
      description: 'Return to the home dashboard',
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
  const perms = authStore.user?.rbacPermissions ?? authStore.initResponse?.rbacPermissions ?? []
  const superuser = authStore.user?.rbacIsSuperuser ?? authStore.initResponse?.rbacIsSuperuser ?? false
  if (!canManageSystemSettings(perms, superuser)) {
    return
  }
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
    <NotificationBlock v-if="error" type="error" :message="error" />

    <div class="control-sections">
      <Section
        title="Control Panel"
        subtitle="Administrative tools and quick actions"
        :icon="DatabaseSettingIcon"
      >
        <Navigation ref="localNavigation">
          <NavigationGrid />
        </Navigation>
      </Section>

      <Section title="System Diagnostics" :icon="ComputerIcon">
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

      <Section
        v-if="authStore.hasPermission('system.settings')"
        title="Database Diagnostics"
        :icon="DatabaseIcon"
      >
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

.control-sections {
  display: grid;
  gap: 30px;
}
</style>
