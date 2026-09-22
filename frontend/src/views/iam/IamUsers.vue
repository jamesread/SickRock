<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { HugeiconsIcon } from '@hugeicons/vue'
import { Add01Icon, RefreshIcon, UserMultiple02Icon } from '@hugeicons/core-free-icons'
import Section from 'picocrank/vue/components/Section.vue'
import TableWithRowClick from '../../components/TableWithRowClick.vue'
import IamCreateUserDialog from '../../components/IamCreateUserDialog.vue'
import { createApiClient } from '../../stores/api'
import { useAuthStore } from '../../stores/auth'
import { useRbac } from '../../composables/useRbac'
import type { IamUser } from '../../gen/sickrock_pb'

const iconStrokeWidth = 2.5
const router = useRouter()
const authStore = useAuthStore()
const { hasPermission } = useRbac()
const client = createApiClient()

const users = ref<IamUser[]>([])
const loading = ref(true)
const saving = ref(false)
const errorMessage = ref('')
const createDialog = ref<InstanceType<typeof IamCreateUserDialog> | null>(null)

const canCreate = computed(() => hasPermission('users.create'))
const canDelete = computed(() => hasPermission('users.delete'))

const tableHeaders = computed(() => {
  const headers = [
    { key: 'username', label: 'Username', sortable: true },
    { key: 'createdByLabel', label: 'Created by', sortable: true },
  ]
  if (canDelete.value) {
    headers.push({ key: 'actions', label: 'Actions', sortable: false, width: '6rem' })
  }
  return headers
})

const tableRows = computed(() =>
  users.value.map((u) => ({
    ...u,
    createdByLabel: u.createdBy || '—',
  })),
)

function isCurrentUser(user: IamUser) {
  return Boolean(authStore.user?.username && user.username === authStore.user.username)
}

async function loadAll() {
  loading.value = true
  errorMessage.value = ''
  try {
    const res = await client.listUsers({})
    users.value = res.users ?? []
  } catch (e: unknown) {
    errorMessage.value = e instanceof Error ? e.message : 'Failed to load users'
  } finally {
    loading.value = false
  }
}

function openUserDetails(row: Record<string, unknown>) {
  router.push({ name: 'iam-user-details', params: { id: String(row.id) } })
}

function openCreate() {
  createDialog.value?.open()
}

function onUserCreated(userId: number) {
  if (userId > 0) {
    router.push({ name: 'iam-user-details', params: { id: String(userId) } })
    return
  }
  loadAll()
}

async function deleteUserAccount(user: IamUser) {
  if (!canDelete.value || isCurrentUser(user)) return
  if (!confirm(`Delete user "${user.username}"? This cannot be undone.`)) return

  saving.value = true
  errorMessage.value = ''
  try {
    await client.deleteUser({ userId: user.id })
    await loadAll()
  } catch (e: unknown) {
    errorMessage.value = e instanceof Error ? e.message : 'Failed to delete user.'
  } finally {
    saving.value = false
  }
}

onMounted(loadAll)
</script>

<template>
  <IamCreateUserDialog ref="createDialog" @created="onUserCreated" />

  <Section
    subtitle="View users, create new accounts, or remove users you no longer need."
    classes="settings-users"
    :padding="false"
  >
    <template #title>
      <span class="section-title-with-icon">
        <HugeiconsIcon :icon="UserMultiple02Icon" width="22" height="22" aria-hidden="true" />
        Users
      </span>
    </template>

    <template #toolbar>
      <button type="button" class="inline-icon neutral" aria-label="Refresh" :disabled="loading" @click="loadAll">
        <HugeiconsIcon :icon="RefreshIcon" width="1em" height="1em" :strokeWidth="iconStrokeWidth" />
      </button>
      <button
        v-if="canCreate"
        type="button"
        class="inline-icon good"
        aria-label="Create user"
        :disabled="loading"
        @click="openCreate"
      >
        <HugeiconsIcon :icon="Add01Icon" width="1em" height="1em" :strokeWidth="iconStrokeWidth" />
      </button>
    </template>

    <div v-if="errorMessage" class="list-banner-pad inline-notification error">{{ errorMessage }}</div>
    <div v-if="loading && !users.length" class="list-banner-pad muted">Loading…</div>

    <template v-else>
      <p v-if="!users.length" class="list-banner-pad inline-notification note">No users available.</p>

      <TableWithRowClick
        v-else
        class="user-table-wrap"
        row-clickable
        :data="tableRows"
        :headers="tableHeaders"
        @row-click="openUserDetails"
      >
        <template #cell-username="{ value }">
          <strong>{{ value }}</strong>
        </template>
        <template #cell-actions="{ row }">
          <div v-if="canDelete" class="actions-cell">
            <button
              type="button"
              class="bad small"
              :disabled="saving || isCurrentUser(row as IamUser)"
              :title="isCurrentUser(row as IamUser) ? 'You cannot delete your own account' : 'Delete user'"
              @click.stop="deleteUserAccount(row as IamUser)"
            >
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
.user-table-wrap {
  margin-top: 0.5rem;
  margin-bottom: 1.5rem;
}
.actions-cell {
  text-align: right;
}
</style>
