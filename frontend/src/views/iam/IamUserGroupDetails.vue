<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { HugeiconsIcon } from '@hugeicons/vue'
import { ArrowLeft01Icon, DatabaseIcon, RefreshIcon, UserGroupIcon, UserMultiple02Icon, WebSecurityIcon } from '@hugeicons/core-free-icons'
import Section from 'picocrank/vue/components/Section.vue'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'
import CheckGroup from 'picocrank/vue/components/CheckGroup.vue'
import FormLayout from 'picocrank/vue/components/FormLayout.vue'
import FormField from 'picocrank/vue/components/FormField.vue'
import { createApiClient } from '../../stores/api'
import { useRbac } from '../../composables/useRbac'
import { isSystemGroup } from '../../utils/rbacAccess'
import type { UserGroup, IamUser, RbacRole } from '../../gen/sickrock_pb'

const iconStrokeWidth = 2.5
const route = useRoute()
const router = useRouter()
const { hasPermission } = useRbac()
const client = createApiClient()

const group = ref<UserGroup | null>(null)
const allUsers = ref<IamUser[]>([])
const allRoles = ref<RbacRole[]>([])
const memberIds = ref<number[]>([])
const roleIds = ref<number[]>([])
type TableAclRow = { tableKey: string; title: string; roleId: number }
const tableAclRows = ref<TableAclRow[]>([])
const loading = ref(true)
const saving = ref(false)
const errorMessage = ref('')
const saveMessage = ref('')

const groupId = computed(() => Number(route.params.id))
const canManageMembers = computed(() => hasPermission('usergroups.manage'))
const canManageRoles = computed(() => hasPermission('rbac.manage'))
const canViewRoles = computed(() => hasPermission('rbac.view'))
const isSystem = computed(() => group.value ? isSystemGroup(group.value.name) : false)

const userOptions = computed(() =>
  allUsers.value.map((u) => ({ label: u.username, value: Number(u.id) })),
)

const roleOptions = computed(() =>
  allRoles.value.map((r) => ({ label: r.name, value: Number(r.id) })),
)

async function load() {
  loading.value = true
  errorMessage.value = ''
  saveMessage.value = ''
  try {
    const [groupsRes, usersRes] = await Promise.all([
      client.listUserGroups({}),
      client.listUsers({}),
    ])
    allUsers.value = usersRes.users ?? []
    group.value = (groupsRes.groups ?? []).find((g) => Number(g.id) === groupId.value) ?? null

    if (!group.value) return

    const membersRes = await client.getUserGroupMembers({ groupId: groupId.value })
    memberIds.value = (membersRes.userIds ?? []).map(Number)

    if (canViewRoles.value) {
      const [rolesRes, groupRolesRes] = await Promise.all([
        client.listRbacRoles({}),
        client.getUserGroupRbacRoles({ groupId: groupId.value }),
      ])
      allRoles.value = rolesRes.roles ?? []
      roleIds.value = (groupRolesRes.roleIds ?? []).map(Number)

      const [tablesRes, tableGrantsRes] = await Promise.all([
        client.getTableConfigurations({}),
        client.listGroupTableRoleGrants({ groupId: groupId.value }),
      ])
      const grantByTable = new Map(
        (tableGrantsRes.grants ?? []).map((g) => [g.tableKey, Number(g.roleId)]),
      )
      tableAclRows.value = (tablesRes.pages ?? [])
        .filter((p) => p.id !== 'table_logs')
        .map((p) => ({
          tableKey: p.id,
          title: p.title || p.id,
          roleId: grantByTable.get(p.id) ?? 0,
        }))
        .sort((a, b) => a.title.localeCompare(b.title))
    }
  } catch (e: unknown) {
    errorMessage.value = e instanceof Error ? e.message : 'Failed to load group.'
    group.value = null
  } finally {
    loading.value = false
  }
}

async function saveMembers() {
  if (!canManageMembers.value || !group.value) return
  saving.value = true
  saveMessage.value = ''
  errorMessage.value = ''
  try {
    await client.setUserGroupMembers({
      groupId: groupId.value,
      userIds: memberIds.value,
    })
    saveMessage.value = 'Group members updated.'
    await load()
  } catch (e: unknown) {
    errorMessage.value = e instanceof Error ? e.message : 'Failed to save members.'
  } finally {
    saving.value = false
  }
}

async function saveTableAcl() {
  if (!canManageRoles.value || !group.value) return
  saving.value = true
  saveMessage.value = ''
  errorMessage.value = ''
  try {
    const grants = tableAclRows.value
      .filter((row) => row.roleId > 0)
      .map((row) => ({ tableKey: row.tableKey, roleId: row.roleId }))
    await client.setGroupTableRoleGrants({
      groupId: groupId.value,
      grants,
    })
    saveMessage.value = 'Table access updated.'
    await load()
  } catch (e: unknown) {
    errorMessage.value = e instanceof Error ? e.message : 'Failed to save table access.'
  } finally {
    saving.value = false
  }
}

async function saveRoles() {
  if (!canManageRoles.value || !group.value) return
  saving.value = true
  saveMessage.value = ''
  errorMessage.value = ''
  try {
    await client.setUserGroupRbacRoles({
      groupId: groupId.value,
      roleIds: roleIds.value,
    })
    saveMessage.value = 'Group roles updated.'
    await load()
  } catch (e: unknown) {
    errorMessage.value = e instanceof Error ? e.message : 'Failed to save roles.'
  } finally {
    saving.value = false
  }
}

async function deleteGroup() {
  if (!canManageMembers.value || !group.value || isSystem.value) return
  if (!confirm(`Delete group "${group.value.name}"?`)) return
  saving.value = true
  try {
    await client.deleteUserGroup({ groupId: groupId.value })
    router.push({ name: 'iam-groups' })
  } catch (e: unknown) {
    errorMessage.value = e instanceof Error ? e.message : 'Failed to delete group.'
  } finally {
    saving.value = false
  }
}

watch(() => route.params.id, load)
onMounted(load)
</script>

<template>
  <Section
    :title="group?.name || 'User group'"
    :subtitle="group ? 'Group overview and membership.' : ''"
    :icon="UserGroupIcon"
  >
    <template #toolbar>
      <router-link :to="{ name: 'iam-groups' }" class="button inline-icon neutral">
        <HugeiconsIcon :icon="ArrowLeft01Icon" width="1em" height="1em" :strokeWidth="iconStrokeWidth" />
        <span>Back to Groups</span>
      </router-link>
      <button type="button" class="inline-icon neutral" aria-label="Refresh" :disabled="loading" @click="load">
        <HugeiconsIcon :icon="RefreshIcon" width="1em" height="1em" :strokeWidth="iconStrokeWidth" />
      </button>
      <button
        v-if="canManageMembers && group && !isSystem"
        type="button"
        class="inline-icon bad"
        :disabled="saving"
        @click="deleteGroup"
      >
        Delete group
      </button>
    </template>

    <NotificationBlock v-if="errorMessage" type="error" :message="errorMessage" />
    <div v-if="saveMessage" class="inline-notification note">{{ saveMessage }}</div>
    <div v-if="loading" class="muted">Loading…</div>
    <div v-else-if="!group" class="inline-notification note">User group not found.</div>
    <template v-else>
      <dl class="group-meta">
        <dt>Group ID</dt>
        <dd>{{ group.id }}</dd>
        <dt>Members</dt>
        <dd>{{ memberIds.length }}</dd>
        <dt>Roles</dt>
        <dd>{{ roleIds.length }}</dd>
      </dl>
      <p v-if="isSystem" class="inline-notification note">
        <strong>{{ group.name }}</strong> is a system group. Changes may affect access for all users.
      </p>
    </template>
  </Section>

  <Section
    v-if="group"
    title="Members"
    subtitle="Users in this group inherit its roles."
    :icon="UserMultiple02Icon"
  >
    <FormLayout @submit.prevent="saveMembers">
      <FormField label="Users" component-has-label :disabled="!canManageMembers || saving">
        <CheckGroup
          v-model="memberIds"
          name="group-members"
          :options="userOptions"
          :disabled="!canManageMembers || saving"
        />
      </FormField>
      <p v-if="!canManageMembers" class="list-banner-pad inline-notification note">
        You can view members. Editing requires usergroups.manage.
      </p>
      <template v-if="canManageMembers" #actions>
        <button type="submit" class="good" :disabled="saving">{{ saving ? 'Saving…' : 'Save members' }}</button>
      </template>
    </FormLayout>
  </Section>

  <Section
    v-if="group && canViewRoles"
    title="RBAC roles"
    subtitle="App-wide permissions (IAM, settings, etc.). Table data access is configured separately below."
    :icon="WebSecurityIcon"
  >
    <FormLayout @submit.prevent="saveRoles">
      <FormField label="Roles" component-has-label :disabled="!canManageRoles || saving">
        <CheckGroup
          v-model="roleIds"
          name="group-roles"
          :options="roleOptions"
          :disabled="!canManageRoles || saving"
        />
      </FormField>
      <p v-if="!canManageRoles" class="inline-notification note">
        You can view group roles. Editing requires rbac.manage.
      </p>
      <template v-if="canManageRoles" #actions>
        <button type="submit" class="good" :disabled="saving">{{ saving ? 'Saving…' : 'Save roles' }}</button>
      </template>
    </FormLayout>
  </Section>

  <Section
    v-if="group && canViewRoles"
    title="Table access"
    subtitle="Assign a role per table. Members get that role's table permissions only on the listed tables."
    :icon="DatabaseIcon"
  >
    <FormLayout @submit.prevent="saveTableAcl">
      <p v-if="tableAclRows.length === 0" class="muted">No table configurations found.</p>
      <div v-else class="table-acl-grid">
        <div class="table-acl-head">Table</div>
        <div class="table-acl-head">Role</div>
        <template v-for="row in tableAclRows" :key="row.tableKey">
          <div class="table-acl-cell">{{ row.title }}</div>
          <div class="table-acl-cell">
            <select v-model.number="row.roleId" :disabled="!canManageRoles || saving">
              <option :value="0">No access</option>
              <option v-for="opt in roleOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
            </select>
          </div>
        </template>
      </div>
      <p v-if="!canManageRoles" class="inline-notification note">
        You can view table access. Editing requires rbac.manage.
      </p>
      <template v-if="canManageRoles" #actions>
        <button type="submit" class="good" :disabled="saving">{{ saving ? 'Saving…' : 'Save table access' }}</button>
      </template>
    </FormLayout>
  </Section>
</template>

<style scoped>
.group-meta {
  display: grid;
  grid-template-columns: max-content 1fr;
  gap: 0.35rem 1.5rem;
}
.list-banner-pad {
  padding-left: 1em;
  padding-right: 1em;
}
.table-acl-grid {
  display: grid;
  grid-template-columns: 1fr min(16rem, 40vw);
  gap: 0.5rem 1rem;
  align-items: center;
}
.table-acl-head {
  font-weight: 600;
}
.table-acl-cell select {
  width: 100%;
}
</style>
