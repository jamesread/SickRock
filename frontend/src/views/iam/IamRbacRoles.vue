<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { HugeiconsIcon } from '@hugeicons/vue'
import { Add01Icon, RefreshIcon, WebSecurityIcon } from '@hugeicons/core-free-icons'
import Section from 'picocrank/vue/components/Section.vue'
import TableWithRowClick from '../../components/TableWithRowClick.vue'
import IamCreateRoleDialog from '../../components/IamCreateRoleDialog.vue'
import { createApiClient } from '../../stores/api'
import { useRbac } from '../../composables/useRbac'
import { isSystemRole } from '../../utils/rbacAccess'
import type { RbacPermission, RbacRole } from '../../gen/sickrock_pb'

const iconStrokeWidth = 2.5
const router = useRouter()
const { hasPermission } = useRbac()
const client = createApiClient()

const permissions = ref<RbacPermission[]>([])
const roles = ref<RbacRole[]>([])
const loading = ref(true)
const saving = ref(false)
const errorMessage = ref('')
const createDialog = ref<InstanceType<typeof IamCreateRoleDialog> | null>(null)

const canManage = computed(() => hasPermission('rbac.manage'))

const permById = computed(() => {
  const m = new Map<number, RbacPermission>()
  for (const p of permissions.value) m.set(Number(p.id), p)
  return m
})

const tableHeaders = computed(() => {
  const headers = [
    { key: 'name', label: 'Name', sortable: true },
    { key: 'description', label: 'Description', sortable: true },
    { key: 'permissionsLabel', label: 'Permissions', sortable: true },
    { key: 'tags', label: 'Tags', sortable: false, width: '11rem' },
  ]
  if (canManage.value) {
    headers.push({ key: 'actions', label: 'Actions', sortable: false, width: '6rem' })
  }
  return headers
})

function permissionLabels(ids: number[] | undefined) {
  if (!ids?.length) return '—'
  return ids
    .map((id) => permById.value.get(id)?.name ?? String(id))
    .sort()
    .join(', ')
}

const tableRows = computed(() =>
  roles.value.map((r) => ({
    ...r,
    description: r.description || '—',
    permissionsLabel: permissionLabels(r.permissionIds?.map(Number)),
  })),
)

function canDeleteRole(r: RbacRole) {
  return canManage.value && !isSystemRole(r.name)
}

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
    errorMessage.value = e instanceof Error ? e.message : 'Failed to load RBAC data.'
  } finally {
    loading.value = false
  }
}

function onRoleRowClick(row: Record<string, unknown>) {
  router.push({ name: 'iam-rbac-role-details', params: { id: String(row.id) } })
}

function openCreate() {
  createDialog.value?.open()
}

function onRoleCreated(roleId: number) {
  if (roleId > 0) {
    router.push({ name: 'iam-rbac-role-details', params: { id: String(roleId) } })
    return
  }
  loadAll()
}

async function deleteRole(r: RbacRole) {
  if (!canDeleteRole(r) || !confirm(`Delete role "${r.name}"? Groups lose this role assignment.`)) return
  saving.value = true
  try {
    await client.deleteRbacRole({ roleId: r.id })
    await loadAll()
  } catch (e: unknown) {
    errorMessage.value = e instanceof Error ? e.message : 'Failed to delete role.'
  } finally {
    saving.value = false
  }
}

onMounted(loadAll)
</script>

<template>
  <IamCreateRoleDialog ref="createDialog" :permissions="permissions" @created="onRoleCreated" />

  <Section
    subtitle="Manage RBAC roles and link them to permissions. Assign roles to groups — not users directly."
    :padding="false"
  >
    <template #title>
      <span class="section-title-with-icon">
        <HugeiconsIcon :icon="WebSecurityIcon" width="22" height="22" aria-hidden="true" />
        Roles &amp; Permissions
      </span>
    </template>

    <template #toolbar>
      <button type="button" class="inline-icon neutral" aria-label="Refresh" :disabled="loading" @click="loadAll">
        <HugeiconsIcon :icon="RefreshIcon" width="1em" height="1em" :strokeWidth="iconStrokeWidth" />
      </button>
      <button
        v-if="canManage"
        type="button"
        class="inline-icon good"
        aria-label="Create role"
        :disabled="loading"
        @click="openCreate"
      >
        <HugeiconsIcon :icon="Add01Icon" width="1em" height="1em" :strokeWidth="iconStrokeWidth" />
      </button>
    </template>

    <div v-if="errorMessage" class="list-banner-pad inline-notification error">{{ errorMessage }}</div>
    <div v-if="loading && !roles.length" class="list-banner-pad muted">Loading…</div>

    <template v-else>
      <p v-if="!canManage" class="list-banner-pad inline-notification note">
        You can view roles. Editing requires rbac.manage.
      </p>
      <p v-if="!roles.length" class="list-banner-pad inline-notification note">No roles yet.</p>

      <TableWithRowClick
        v-else
        class="roles-table-wrap"
        row-clickable
        :data="tableRows"
        :headers="tableHeaders"
        @row-click="onRoleRowClick"
      >
        <template #cell-name="{ value }">
          <strong>{{ value }}</strong>
        </template>
        <template #cell-tags="{ row }">
          <span class="role-tags">
            <span v-if="isSystemRole(String(row.name))" class="tag fg-note">system role</span>
            <span v-if="row.name === 'superuser'" class="tag fg-good">all access</span>
          </span>
        </template>
        <template #cell-actions="{ row }">
          <div v-if="canDeleteRole(row as RbacRole)" class="actions-cell">
            <button type="button" class="bad small" :disabled="saving" @click.stop="deleteRole(row as RbacRole)">
              Delete
            </button>
          </div>
        </template>
      </TableWithRowClick>
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
.roles-table-wrap {
  margin-top: 0.5rem;
  margin-bottom: 1.5rem;
}
.actions-cell {
  text-align: right;
}
.role-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 0.25rem;
}
</style>
