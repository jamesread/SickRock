<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { ArrowLeft01Icon, RefreshIcon, ShieldKeyIcon } from '@hugeicons/core-free-icons'
import Section from 'picocrank/vue/components/Section.vue'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'
import { createApiClient } from '../../stores/api'

const iconStrokeWidth = 2.5
const client = createApiClient()

const groupNames = ref<string[]>([])
const roleNames = ref<string[]>([])
const isSuperuser = ref(false)
const permissionRows = ref<{ permission: string; granted: boolean; grantingGroups: string[] }[]>([])
const loading = ref(false)
const errorMessage = ref('')

const auditRows = computed(() =>
  permissionRows.value.map((row) => ({
    ...row,
    name: row.permission,
  })),
)

async function load() {
  loading.value = true
  errorMessage.value = ''
  try {
    const res = await client.getMyPermissionsAudit({})
    groupNames.value = res.groupNames ?? []
    roleNames.value = res.roleNames ?? []
    isSuperuser.value = Boolean(res.isSuperuser)
    permissionRows.value = (res.permissions ?? []).map((p) => ({
      permission: p.permission,
      granted: p.granted,
      grantingGroups: p.grantingGroups ?? [],
    }))
  } catch (e: unknown) {
    errorMessage.value = e instanceof Error ? e.message : 'Failed to load permissions audit.'
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <Section
    title="My Permissions"
    subtitle="Review your group membership and effective permissions"
    :icon="ShieldKeyIcon"
  >
    <template #toolbar>
      <router-link :to="{ name: 'user-control-panel' }" class="button inline-icon neutral">
        <HugeiconsIcon :icon="ArrowLeft01Icon" width="1em" height="1em" :strokeWidth="iconStrokeWidth" />
        <span>Back to User Control Panel</span>
      </router-link>
      <button type="button" class="inline-icon neutral" aria-label="Refresh" :disabled="loading" @click="load">
        <HugeiconsIcon :icon="RefreshIcon" width="1em" height="1em" :strokeWidth="iconStrokeWidth" />
      </button>
    </template>

    <NotificationBlock v-if="errorMessage" type="error" :message="errorMessage" />
    <div v-else-if="loading" class="muted">Loading…</div>
    <template v-else>
      <h3 class="subsection-title">Group membership</h3>
      <p v-if="!groupNames.length" class="inline-notification note">You are not a member of any user groups.</p>
      <p v-else class="inline-tags">
        <span v-for="name in groupNames" :key="name" class="tag note">{{ name }}</span>
      </p>

      <h3 class="subsection-title">Effective roles</h3>
      <p v-if="!roleNames.length" class="inline-notification note">No roles via group membership.</p>
      <p v-else class="inline-tags">
        <span v-for="name in roleNames" :key="name" class="tag">{{ name }}</span>
      </p>

      <h3 class="subsection-title">Effective permissions</h3>
      <p v-if="isSuperuser" class="inline-notification note">
        You have the <strong>superuser</strong> role — all permissions are granted.
      </p>

      <table v-if="auditRows.length" class="perm-audit-table data-table">
        <thead>
          <tr>
            <th class="perm-status-col">Status</th>
            <th>Permission</th>
            <th>Granted via groups</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in auditRows" :key="row.name">
            <td class="perm-status-col">
              <span v-if="row.granted" class="tag good">granted</span>
              <span v-else class="tag bad">denied</span>
            </td>
            <td><code>{{ row.name }}</code></td>
            <td>
              <span v-if="!(isSuperuser && !row.grantingGroups.length) && !row.grantingGroups.length" class="muted">—</span>
              <span v-else class="inline-tags">
                <span v-if="isSuperuser && !row.grantingGroups.length" class="tag good">superuser</span>
                <span v-for="gn in row.grantingGroups" :key="gn" class="tag note">{{ gn }}</span>
              </span>
            </td>
          </tr>
        </tbody>
      </table>
    </template>
  </Section>
</template>

<style scoped>
.subsection-title {
  margin: 1.25rem 0 0.35rem;
}
.inline-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 0.25rem;
}
.perm-audit-table {
  width: 100%;
  margin-top: 0.75rem;
}
.perm-status-col {
  width: 6.5rem;
}
.muted {
  opacity: 0.75;
}
</style>
