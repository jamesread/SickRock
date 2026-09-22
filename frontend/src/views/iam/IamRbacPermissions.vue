<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { RefreshIcon, ShieldKeyIcon } from '@hugeicons/core-free-icons'
import Section from 'picocrank/vue/components/Section.vue'
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

const tableRows = computed(() =>
  permissions.value.map((p) => {
    const grantingRoles = roles.value.filter((r) =>
      (r.permissionIds ?? []).map(Number).includes(Number(p.id)),
    )
    return {
      ...p,
      description: p.description || '—',
      rolesLabel: grantingRoles.length
        ? grantingRoles.map((r) => r.name).sort().join(', ')
        : '—',
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
    subtitle="All RBAC permissions. Permissions reach users through roles assigned to their groups."
    :padding="false"
  >
    <template #title>
      <span class="section-title-with-icon">
        <HugeiconsIcon :icon="ShieldKeyIcon" width="22" height="22" aria-hidden="true" />
        Permission Catalog
      </span>
    </template>

    <template #toolbar>
      <router-link :to="{ name: 'iam-rbac' }" class="button inline-icon neutral">Back to Roles</router-link>
      <button type="button" class="inline-icon neutral" aria-label="Refresh" :disabled="loading" @click="loadAll">
        <HugeiconsIcon :icon="RefreshIcon" width="1em" height="1em" :strokeWidth="iconStrokeWidth" />
      </button>
    </template>

    <div v-if="errorMessage" class="list-banner-pad inline-notification error">{{ errorMessage }}</div>
    <div v-if="loading && !permissions.length" class="list-banner-pad muted">Loading…</div>

    <template v-else>
      <p v-if="!permissions.length" class="list-banner-pad inline-notification note">No permissions defined.</p>
      <Table v-else class="permissions-table-wrap" :data="tableRows" :headers="tableHeaders">
        <template #cell-name="{ value }">
          <code>{{ value }}</code>
        </template>
      </Table>
    </template>
  </Section>
</template>

<style scoped>
.section-title-with-icon {
  display: inline-flex;
  align-items: center;
  gap: 0.45em;
}
.list-banner-pad {
  padding-left: 1em;
  padding-right: 1em;
}
.permissions-table-wrap {
  margin-top: 0.5rem;
  margin-bottom: 1.5rem;
}
</style>
