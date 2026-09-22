<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { ArrowLeft01Icon, RefreshIcon, ShieldKeyIcon } from '@hugeicons/core-free-icons'
import Section from 'picocrank/vue/components/Section.vue'
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
  <Section title="My Permissions" subtitle="Review your group membership and effective permissions">
    <template #toolbar>
      <router-link :to="{ name: 'user-control-panel' }" class="button inline-icon neutral">
        <HugeiconsIcon :icon="ArrowLeft01Icon" width="1em" height="1em" :strokeWidth="iconStrokeWidth" />
        <span>Back to User Control Panel</span>
      </router-link>
      <button type="button" class="inline-icon neutral" aria-label="Refresh" :disabled="loading" @click="load">
        <HugeiconsIcon :icon="RefreshIcon" width="1em" height="1em" :strokeWidth="iconStrokeWidth" />
      </button>
    </template>

    <div v-if="errorMessage" class="inline-notification error">{{ errorMessage }}</div>
    <div v-else-if="loading" class="muted">Loading…</div>
    <template v-else>
      <h3 class="subsection-title">Group membership</h3>
      <p v-if="!groupNames.length" class="inline-notification note">You are not a member of any user groups.</p>
      <p v-else>
        <span v-for="name in groupNames" :key="name" class="role-tag">{{ name }}</span>
      </p>

      <h3 class="subsection-title">Effective roles</h3>
      <p v-if="!roleNames.length" class="inline-notification note">No roles via group membership.</p>
      <p v-else>
        <span v-for="name in roleNames" :key="name" class="role-tag">{{ name }}</span>
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
            <td class="perm-status-col">{{ row.granted ? '✓' : '✗' }}</td>
            <td><code>{{ row.name }}</code></td>
            <td>
              <span v-if="isSuperuser && !row.grantingGroups.length" class="role-tag">superuser</span>
              <template v-else>
                <span v-for="gn in row.grantingGroups" :key="gn" class="role-tag">{{ gn }}</span>
                <span v-if="!row.grantingGroups.length" class="muted">—</span>
              </template>
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
.role-tag {
  display: inline-block;
  margin: 0 0.35rem 0.35rem 0;
  padding: 0.2rem 0.55rem;
  border-radius: 4px;
  background: #e2e8f0;
  font-size: 0.85rem;
}
.perm-audit-table {
  width: 100%;
  margin-top: 0.75rem;
}
.perm-status-col {
  width: 4rem;
  text-align: center;
}
.muted {
  opacity: 0.75;
}
</style>
