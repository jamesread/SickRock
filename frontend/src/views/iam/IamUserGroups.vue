<script setup lang="ts">
import { ref, computed, onMounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { HugeiconsIcon } from '@hugeicons/vue'
import { Add01Icon, RefreshIcon, UserGroupIcon } from '@hugeicons/core-free-icons'
import Section from 'picocrank/vue/components/Section.vue'
import FormLayout from 'picocrank/vue/components/FormLayout.vue'
import FormField from 'picocrank/vue/components/FormField.vue'
import TableWithRowClick from '../../components/TableWithRowClick.vue'
import { createApiClient } from '../../stores/api'
import { useRbac } from '../../composables/useRbac'
import { isSystemGroup } from '../../utils/rbacAccess'
import type { UserGroup } from '../../gen/sickrock_pb'

const iconStrokeWidth = 2.5
const router = useRouter()
const { hasPermission } = useRbac()
const client = createApiClient()

const groups = ref<(UserGroup & { roleCount?: number })[]>([])
const loading = ref(true)
const saving = ref(false)
const errorMessage = ref('')
const createGroupError = ref('')
const newGroupName = ref('')
const createGroupDialog = ref<HTMLDialogElement | null>(null)
const createGroupNameInput = ref<HTMLInputElement | null>(null)

const canManage = computed(() => hasPermission('usergroups.manage'))
const canViewRoles = computed(() => hasPermission('rbac.view'))

const tableHeaders = computed(() => {
  const headers = [
    { key: 'name', label: 'Name', sortable: true },
    { key: 'memberCount', label: 'Members', sortable: true, width: '8rem' },
  ]
  if (canViewRoles.value) {
    headers.push({ key: 'roleCount', label: 'Roles', sortable: true, width: '8rem' })
  }
  if (canManage.value) {
    headers.push({ key: 'actions', label: 'Actions', sortable: false, width: '6rem' })
  }
  return headers
})

function canDeleteGroup(group: UserGroup) {
  return canManage.value && !isSystemGroup(group.name)
}

async function loadAll() {
  errorMessage.value = ''
  loading.value = true
  try {
    const gr = await client.listUserGroups({})
    const rawGroups = gr.groups ?? []
    if (rawGroups.length && canViewRoles.value) {
      const roleResults = await Promise.all(
        rawGroups.map((g) => client.getUserGroupRbacRoles({ groupId: g.id })),
      )
      groups.value = rawGroups.map((g, i) => ({
        ...g,
        roleCount: (roleResults[i].roleIds ?? []).length,
      }))
    } else {
      groups.value = rawGroups.map((g) => ({ ...g, roleCount: 0 }))
    }
  } catch (e: unknown) {
    errorMessage.value = e instanceof Error ? e.message : 'Failed to load user groups.'
  } finally {
    loading.value = false
  }
}

function openGroupDetails(row: Record<string, unknown>) {
  router.push({ name: 'iam-group-details', params: { id: String(row.id) } })
}

function openCreateGroupDialog() {
  newGroupName.value = ''
  createGroupError.value = ''
  createGroupDialog.value?.showModal()
  nextTick(() => createGroupNameInput.value?.focus())
}

function closeCreateGroupDialog() {
  createGroupDialog.value?.close()
}

async function createGroup() {
  if (!canManage.value || !newGroupName.value.trim()) return
  saving.value = true
  createGroupError.value = ''
  try {
    const res = await client.createUserGroup({ name: newGroupName.value.trim() })
    closeCreateGroupDialog()
    if (res.group?.id) {
      router.push({ name: 'iam-group-details', params: { id: String(res.group.id) } })
      return
    }
    await loadAll()
  } catch (e: unknown) {
    createGroupError.value = e instanceof Error ? e.message : 'Failed to create group.'
  } finally {
    saving.value = false
  }
}

async function deleteGroup(group: UserGroup) {
  if (!canDeleteGroup(group) || !confirm(`Delete group "${group.name}"? Members will be removed.`)) return
  saving.value = true
  errorMessage.value = ''
  try {
    await client.deleteUserGroup({ groupId: group.id })
    await loadAll()
  } catch (e: unknown) {
    errorMessage.value = e instanceof Error ? e.message : 'Failed to delete group.'
  } finally {
    saving.value = false
  }
}

onMounted(loadAll)
</script>

<template>
  <dialog ref="createGroupDialog" class="dialog" @close="newGroupName = ''">
    <h2>Create group</h2>
    <FormLayout @submit.prevent="createGroup">
      <FormField label="Name" for="new-group-name" :disabled="saving">
        <input
          id="new-group-name"
          ref="createGroupNameInput"
          v-model="newGroupName"
          type="text"
          autocomplete="off"
          :disabled="saving"
          required
        />
      </FormField>
      <p v-if="createGroupError" class="inline-notification error">{{ createGroupError }}</p>
      <template #actions>
        <button type="button" class="neutral" :disabled="saving" @click="closeCreateGroupDialog">Cancel</button>
        <button type="submit" class="good" :disabled="saving || !newGroupName.trim()">Create group</button>
      </template>
    </FormLayout>
  </dialog>

  <Section subtitle="Manage user groups. Groups carry RBAC roles to their members." :padding="false">
    <template #title>
      <span class="section-title-with-icon">
        <HugeiconsIcon :icon="UserGroupIcon" width="22" height="22" aria-hidden="true" />
        User Groups
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
        aria-label="Create group"
        :disabled="loading"
        @click="openCreateGroupDialog"
      >
        <HugeiconsIcon :icon="Add01Icon" width="1em" height="1em" :strokeWidth="iconStrokeWidth" />
      </button>
    </template>

    <div v-if="errorMessage" class="list-banner-pad inline-notification error">{{ errorMessage }}</div>
    <div v-if="loading && !groups.length" class="list-banner-pad muted">Loading…</div>

    <template v-else>
      <p v-if="!groups.length" class="list-banner-pad inline-notification note">No user groups yet.</p>

      <TableWithRowClick
        v-else
        class="user-table-wrap"
        row-clickable
        :data="groups"
        :headers="tableHeaders"
        @row-click="openGroupDetails"
      >
        <template #cell-name="{ row, value }">
          <span class="group-name-cell">
            <strong>{{ value }}</strong>
            <span v-if="isSystemGroup(String(row.name))" class="tag fg-note">system group</span>
          </span>
        </template>
        <template #cell-actions="{ row }">
          <div v-if="canDeleteGroup(row as UserGroup)" class="actions-cell">
            <button type="button" class="bad small" :disabled="saving" @click.stop="deleteGroup(row as UserGroup)">
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
.group-name-cell {
  display: inline-flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.45rem;
}
</style>
