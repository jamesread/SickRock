<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { HugeiconsIcon } from '@hugeicons/vue'
import { ArrowLeft01Icon, RefreshIcon, UserIcon } from '@hugeicons/core-free-icons'
import Section from 'picocrank/vue/components/Section.vue'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'
import { createApiClient } from '../../stores/api'
import { useRbac } from '../../composables/useRbac'
import type { IamUser, UserEffectiveRoleGrant } from '../../gen/sickrock_pb'

const iconStrokeWidth = 2.5
const route = useRoute()
const { hasPermission } = useRbac()
const client = createApiClient()

const user = ref<IamUser | null>(null)
const roleGrants = ref<UserEffectiveRoleGrant[]>([])
const isSuperuser = ref(false)
const loading = ref(true)
const saving = ref(false)
const error = ref('')
const resetStatus = ref('')
const resetPassword = ref('')
const resetError = ref('')

const userId = computed(() => Number(route.params.id))
const canResetPassword = computed(() => hasPermission('users.reset-password'))

async function load() {
  loading.value = true
  error.value = ''
  try {
    const res = await client.getUser({ userId: userId.value })
    user.value = res.user ?? null
    if (user.value) {
      const grantsRes = await client.getUserEffectiveRoleGrants({ userId: userId.value })
      isSuperuser.value = Boolean(grantsRes.isSuperuser)
      roleGrants.value = grantsRes.grants ?? []
    } else {
      isSuperuser.value = false
      roleGrants.value = []
    }
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : 'Failed to load user'
    user.value = null
  } finally {
    loading.value = false
  }
}

async function submitResetPassword() {
  resetStatus.value = ''
  resetError.value = ''
  if (!user.value || !resetPassword.value) {
    resetError.value = 'Enter a new password.'
    return
  }
  if (resetPassword.value.length < 8) {
    resetError.value = 'Password must be at least 8 characters.'
    return
  }
  saving.value = true
  try {
    const res = await client.resetUserPassword({
      username: user.value.username,
      newPassword: resetPassword.value,
    })
    if (res.success) {
      resetStatus.value = 'Password updated successfully.'
      resetPassword.value = ''
    } else {
      resetError.value = res.message || 'Failed to update password.'
    }
  } catch (e: unknown) {
    resetError.value = e instanceof Error ? e.message : 'Failed to update password.'
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<template>
  <Section
    :title="user?.username || 'User'"
    subtitle="Account details and effective access"
    :icon="UserIcon"
  >
    <template #toolbar>
      <router-link :to="{ name: 'iam-users' }" class="button inline-icon neutral">
        <HugeiconsIcon :icon="ArrowLeft01Icon" width="1em" height="1em" :strokeWidth="iconStrokeWidth" />
        <span>Back to Users</span>
      </router-link>
      <button type="button" class="inline-icon neutral" aria-label="Refresh" :disabled="loading" @click="load">
        <HugeiconsIcon :icon="RefreshIcon" width="1em" height="1em" :strokeWidth="iconStrokeWidth" />
      </button>
    </template>

    <div v-if="loading" class="muted">Loading…</div>
    <NotificationBlock v-else-if="error" type="error" :message="error" />
    <div v-else-if="!user" class="inline-notification note">User not found.</div>
    <template v-else>
      <dl class="user-meta">
        <dt>User ID</dt>
        <dd>{{ user.id }}</dd>
        <dt>Username</dt>
        <dd>{{ user.username }}</dd>
        <dt>Created by</dt>
        <dd>{{ user.createdBy || '—' }}</dd>
      </dl>

      <h3 class="subsection-title">Effective roles</h3>
      <p class="section-hint">Each row is a role granted through a user group membership.</p>
      <p v-if="isSuperuser" class="inline-notification note">
        This user has the <strong>superuser</strong> role and effectively has all permissions.
      </p>
      <p v-if="!roleGrants.length && !isSuperuser" class="inline-notification note">No roles via group membership.</p>
      <table v-else-if="roleGrants.length" class="data-table role-grants-table">
        <thead>
          <tr>
            <th scope="col">Role</th>
            <th scope="col">Granted via group</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="grant in roleGrants" :key="`${grant.roleId}-${grant.groupId}`">
            <td>
              <router-link :to="{ name: 'iam-rbac-role-details', params: { id: String(grant.roleId) } }">
                {{ grant.roleName }}
              </router-link>
            </td>
            <td>
              <router-link :to="{ name: 'iam-group-details', params: { id: String(grant.groupId) } }">
                {{ grant.groupName }}
              </router-link>
            </td>
          </tr>
        </tbody>
      </table>

      <div v-if="canResetPassword" class="reset-section">
        <h3 class="subsection-title">Reset password</h3>
        <NotificationBlock v-if="resetError" type="error" :message="resetError" />
        <p v-if="resetStatus" class="inline-notification note">{{ resetStatus }}</p>
        <div class="reset-form">
          <input v-model="resetPassword" type="password" placeholder="New password (min 8 characters)" :disabled="saving" />
          <button type="button" class="good" :disabled="saving" @click="submitResetPassword">
            {{ saving ? 'Saving…' : 'Update password' }}
          </button>
        </div>
      </div>
    </template>
  </Section>
</template>

<style scoped>
.user-meta {
  display: grid;
  grid-template-columns: max-content 1fr;
  gap: 0.35rem 1.5rem;
  margin-bottom: 1.5rem;
}
.user-meta dt {
  font-weight: 600;
  color: #64748b;
}
.subsection-title {
  margin: 1.5rem 0 0.35rem;
}
.section-hint {
  color: #64748b;
  font-size: 0.9rem;
  margin: 0 0 0.75rem;
}
.role-grants-table {
  width: 100%;
  max-width: 36rem;
}
.reset-section {
  margin-top: 2rem;
  padding-top: 1rem;
  border-top: 1px solid #e2e8f0;
}
.reset-form {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
  align-items: center;
}
.reset-form input {
  min-width: 220px;
  padding: 0.5rem 0.75rem;
}
</style>
