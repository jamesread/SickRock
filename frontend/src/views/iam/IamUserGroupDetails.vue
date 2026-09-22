<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { HugeiconsIcon } from '@hugeicons/vue'
import { ArrowLeft01Icon, RefreshIcon, UserGroupIcon, WebSecurityIcon } from '@hugeicons/core-free-icons'
import Section from 'picocrank/vue/components/Section.vue'
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
  <Section :subtitle="group ? 'Group overview and membership.' : ''">
    <template #title>
      <span class="section-title-with-icon">
        <HugeiconsIcon :icon="UserGroupIcon" width="22" height="22" aria-hidden="true" />
        {{ group?.name || 'User group' }}
      </span>
    </template>
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

    <div v-if="errorMessage" class="inline-notification error">{{ errorMessage }}</div>
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

  <Section v-if="group" subtitle="Users in this group inherit its roles." :padding="false">
    <template #title>Members</template>
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

  <Section v-if="group && canViewRoles" subtitle="Members inherit permissions from these roles.">
    <template #title>
      <span class="section-title-with-icon">
        <HugeiconsIcon :icon="WebSecurityIcon" width="22" height="22" aria-hidden="true" />
        RBAC roles
      </span>
    </template>
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
</template>

<style scoped>
.section-title-with-icon {
  display: inline-flex;
  align-items: center;
  gap: 0.45em;
}
.group-meta {
  display: grid;
  grid-template-columns: max-content 1fr;
  gap: 0.35rem 1.5rem;
}
.list-banner-pad {
  padding-left: 1em;
  padding-right: 1em;
}
</style>
