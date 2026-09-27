<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { Share08Icon } from '@hugeicons/core-free-icons'
import Section from 'picocrank/vue/components/Section.vue'
import FormLayout from 'picocrank/vue/components/FormLayout.vue'
import { createApiClient } from '../stores/api'
import { useRbac } from '../composables/useRbac'
import { isSystemGroup } from '../utils/rbacAccess'
import type { RbacRole } from '../gen/sickrock_pb'

const props = defineProps<{
  tableKey: string
}>()

const client = createApiClient()
const { hasPermission } = useRbac()

const canView = computed(() => hasPermission('rbac.view'))
const canManage = computed(() => hasPermission('rbac.manage'))

type ShareRow = { groupId: number; groupName: string; roleId: number }
const shareRows = ref<ShareRow[]>([])
const allRoles = ref<RbacRole[]>([])
const loading = ref(false)
const saving = ref(false)
const errorMessage = ref('')
const saveMessage = ref('')

const roleOptions = computed(() =>
  allRoles.value.map((r) => ({ label: r.name, value: Number(r.id) })),
)

async function load() {
  if (!canView.value || !props.tableKey) {
    shareRows.value = []
    return
  }
  loading.value = true
  errorMessage.value = ''
  saveMessage.value = ''
  try {
    const [grantsRes, rolesRes] = await Promise.all([
      client.listTableShareGrants({ tableKey: props.tableKey }),
      client.listRbacRoles({}),
    ])
    allRoles.value = rolesRes.roles ?? []
    shareRows.value = (grantsRes.grants ?? []).map((g) => ({
      groupId: Number(g.groupId),
      groupName: g.groupName,
      roleId: Number(g.roleId) || 0,
    }))
  } catch (e: unknown) {
    errorMessage.value = e instanceof Error ? e.message : 'Failed to load sharing settings.'
    shareRows.value = []
  } finally {
    loading.value = false
  }
}

async function save() {
  if (!canManage.value || !props.tableKey) return
  saving.value = true
  saveMessage.value = ''
  errorMessage.value = ''
  try {
    const grants = shareRows.value
      .filter((row) => row.roleId > 0)
      .map((row) => ({
        groupId: row.groupId,
        groupName: row.groupName,
        roleId: row.roleId,
      }))
    await client.setTableShareGrants({
      tableKey: props.tableKey,
      grants,
    })
    saveMessage.value = 'Table access updated.'
    await load()
  } catch (e: unknown) {
    errorMessage.value = e instanceof Error ? e.message : 'Failed to save sharing settings.'
  } finally {
    saving.value = false
  }
}

watch(() => props.tableKey, load)
onMounted(load)
</script>

<template>
  <Section
    v-if="canView"
    title="Share"
    subtitle="Choose which user groups can access this table and with which role."
    :icon="Share08Icon"
  >
    <div v-if="errorMessage" class="inline-notification error">{{ errorMessage }}</div>
    <div v-if="saveMessage" class="inline-notification note">{{ saveMessage }}</div>
    <div v-if="loading" class="muted">Loading…</div>
    <FormLayout v-else @submit.prevent="save">
      <p class="share-hint muted">
        Members of each group inherit the selected role for this table only.
        Use <strong>No access</strong> to revoke. Changes mirror IAM → User groups → Table access.
      </p>
      <div class="share-grid">
        <div class="share-head">Group</div>
        <div class="share-head">Role</div>
        <template v-for="row in shareRows" :key="row.groupId">
          <div class="share-cell">
            {{ row.groupName }}
            <span v-if="isSystemGroup(row.groupName)" class="tag">system</span>
          </div>
          <div class="share-cell">
            <select v-model.number="row.roleId" :disabled="!canManage || saving">
              <option :value="0">No access</option>
              <option v-for="opt in roleOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
            </select>
          </div>
        </template>
      </div>
      <p v-if="!canManage" class="inline-notification note">
        You can view sharing settings. Editing requires rbac.manage.
      </p>
      <template v-if="canManage" #actions>
        <button type="submit" class="good" :disabled="saving">{{ saving ? 'Saving…' : 'Save access' }}</button>
      </template>
    </FormLayout>
  </Section>
</template>

<style scoped>
.share-hint {
  margin: 0 0 1rem;
}
.share-grid {
  display: grid;
  grid-template-columns: 1fr min(16rem, 40vw);
  gap: 0.5rem 1rem;
  align-items: center;
}
.share-head {
  font-weight: 600;
}
.share-cell select {
  width: 100%;
}
.share-cell .tag {
  margin-left: 0.35rem;
}
</style>
