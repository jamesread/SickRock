<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { RefreshIcon, ShieldKeyIcon } from '@hugeicons/core-free-icons'
import Section from 'picocrank/vue/components/Section.vue'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'
import Table from 'picocrank/vue/components/Table.vue'
import { createApiClient } from '../../stores/api'
import type { RbacPermission, RbacRole } from '../../gen/sickrock_pb'

const iconStrokeWidth = 2.5
const client = createApiClient()

const permissions = ref<RbacPermission[]>([])
const roles = ref<RbacRole[]>([])
const loading = ref(true)
const errorMessage = ref('')

const tableHeaders = [
  { key: 'name', label: 'Permission', sortable: true },
  { key: 'description', label: 'Description', sortable: true },
  { key: 'rolesLabel', label: 'Granted by roles', sortable: true },
]

type PermissionTableRow = RbacPermission & {
  description: string
  rolesLabel: string
  roles: RbacRole[]
}

const tableRows = computed((): PermissionTableRow[] =>
  permissions.value.map((p) => {
    const grantingRoles = roles.value
      .filter((r) => (r.permissionIds ?? []).map(Number).includes(Number(p.id)))
      .sort((a, b) => String(a.name).localeCompare(String(b.name)))
    return {
      ...p,
      description: p.description || '—',
      rolesLabel: grantingRoles.map((r) => r.name).join(', '),
      roles: grantingRoles,
    }
  }),
)

async function loadAll() {
  loading.value = true
  errorMessage.value = ''
  try {
    const [pr, rr] = await Promise.all([
      client.listRbacPermissions({}),
      client.listRbacRoles({}),
    ])
    permissions.value = pr.permissions ?? []
    roles.value = rr.roles ?? []
  } catch (e: unknown) {
    errorMessage.value = e instanceof Error ? e.message : 'Failed to load permissions.'
  } finally {
    loading.value = false
  }
}

onMounted(loadAll)
</script>

<template>
  <Section
    title="Permission Catalog"
    subtitle="All RBAC permissions. Permissions reach users through roles assigned to their groups."
    :icon="ShieldKeyIcon"
    :padding="false"
  >
    <template #toolbar>
      <router-link :to="{ name: 'iam-rbac' }" class="button inline-icon neutral">Back to Roles</router-link>
      <button type="button" class="inline-icon neutral" aria-label="Refresh" :disabled="loading" @click="loadAll">
        <HugeiconsIcon :icon="RefreshIcon" width="1em" height="1em" :strokeWidth="iconStrokeWidth" />
      </button>
    </template>

    <div v-if="errorMessage" class="list-banner-pad">
      <NotificationBlock type="error" :message="errorMessage" />
    </div>
    <div v-if="loading && !permissions.length" class="list-banner-pad muted">Loading…</div>

    <template v-else>
      <p v-if="!permissions.length" class="list-banner-pad inline-notification note">No permissions defined.</p>
      <Table v-else class="permissions-table-wrap" :data="tableRows" :headers="tableHeaders">
        <template #cell-name="{ value }">
          <code>{{ value }}</code>
        </template>
        <template #cell-rolesLabel="{ row }">
          <span v-if="!(row as PermissionTableRow).roles.length" class="muted">—</span>
          <span v-else class="inline-tags">
            <span v-for="role in (row as PermissionTableRow).roles" :key="role.id" class="tag">{{ role.name }}</span>
          </span>
        </template>
      </Table>
    </template>
  </Section>
</template>

<style scoped>
.list-banner-pad {
  padding-left: 1em;
  padding-right: 1em;
}
.permissions-table-wrap {
  margin-top: 0.5rem;
  margin-bottom: 1.5rem;
}
.inline-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 0.25rem;
}
.muted {
  opacity: 0.75;
}
</style>
