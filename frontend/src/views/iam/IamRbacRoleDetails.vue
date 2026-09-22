<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { HugeiconsIcon } from '@hugeicons/vue'
import { ArrowLeft01Icon, RefreshIcon, WebSecurityIcon } from '@hugeicons/core-free-icons'
import Section from 'picocrank/vue/components/Section.vue'
import FormLayout from 'picocrank/vue/components/FormLayout.vue'
import FormField from 'picocrank/vue/components/FormField.vue'
import { createApiClient } from '../../stores/api'
import { useRbac } from '../../composables/useRbac'
import { isSystemRole } from '../../utils/rbacAccess'
import type { RbacPermission, RbacRole } from '../../gen/sickrock_pb'

const iconStrokeWidth = 2.5
const route = useRoute()
const { hasPermission } = useRbac()
const client = createApiClient()

const role = ref<RbacRole | null>(null)
const permissions = ref<RbacPermission[]>([])
const groupNames = ref<string[]>([])
const loading = ref(true)
const saving = ref(false)
const errorMessage = ref('')
const saveMessage = ref('')

const form = ref({ name: '', description: '', permissionIds: [] as number[] })

const roleId = computed(() => Number(route.params.id))
const canManage = computed(() => hasPermission('rbac.manage'))
const isSuperuserRole = computed(() => role.value?.name === 'superuser')
const readOnly = computed(() => !canManage.value || isSuperuserRole.value)

const permissionsSorted = computed(() =>
  [...permissions.value].sort((a, b) => String(a.name).localeCompare(String(b.name))),
)

async function load() {
  loading.value = true
  errorMessage.value = ''
  saveMessage.value = ''
  try {
    const [pr, rr, groupsRes] = await Promise.all([
      client.listRbacPermissions({}),
      client.listRbacRoles({}),
      client.getRbacRoleGroups({ roleId: roleId.value }),
    ])
    permissions.value = pr.permissions ?? []
    role.value = (rr.roles ?? []).find((r) => Number(r.id) === roleId.value) ?? null
    groupNames.value = groupsRes.groupNames ?? []

    if (role.value) {
      form.value = {
        name: role.value.name,
        description: role.value.description ?? '',
        permissionIds: (role.value.permissionIds ?? []).map(Number),
      }
    }
  } catch (e: unknown) {
    errorMessage.value = e instanceof Error ? e.message : 'Failed to load role.'
    role.value = null
  } finally {
    loading.value = false
  }
}

async function save() {
  if (!role.value || readOnly.value) return
  if (!form.value.name.trim()) {
    errorMessage.value = 'Name is required.'
    return
  }
  saving.value = true
  errorMessage.value = ''
  saveMessage.value = ''
  try {
    await client.updateRbacRole({
      roleId: roleId.value,
      name: form.value.name.trim(),
      description: form.value.description.trim(),
      permissionIds: form.value.permissionIds,
    })
    saveMessage.value = 'Role updated.'
    await load()
  } catch (e: unknown) {
    errorMessage.value = e instanceof Error ? e.message : 'Failed to update role.'
  } finally {
    saving.value = false
  }
}

watch(() => route.params.id, load)
onMounted(load)
</script>

<template>
  <Section :subtitle="role ? 'Edit role permissions and metadata.' : ''">
    <template #title>
      <span class="section-title-with-icon">
        <HugeiconsIcon :icon="WebSecurityIcon" width="22" height="22" aria-hidden="true" />
        {{ role?.name || 'Role' }}
      </span>
    </template>
    <template #toolbar>
      <router-link :to="{ name: 'iam-rbac' }" class="button inline-icon neutral">
        <HugeiconsIcon :icon="ArrowLeft01Icon" width="1em" height="1em" :strokeWidth="iconStrokeWidth" />
        <span>Back to Roles</span>
      </router-link>
      <button type="button" class="inline-icon neutral" aria-label="Refresh" :disabled="loading" @click="load">
        <HugeiconsIcon :icon="RefreshIcon" width="1em" height="1em" :strokeWidth="iconStrokeWidth" />
      </button>
    </template>

    <div v-if="errorMessage" class="inline-notification error">{{ errorMessage }}</div>
    <div v-if="saveMessage" class="inline-notification note">{{ saveMessage }}</div>
    <div v-if="loading" class="muted">Loading…</div>
    <div v-else-if="!role" class="inline-notification note">Role not found.</div>
    <template v-else>
      <p v-if="isSuperuserRole" class="inline-notification note">
        The <strong>superuser</strong> role grants all permissions and cannot be edited.
      </p>
      <p v-else-if="isSystemRole(role.name)" class="inline-notification note">
        <strong>{{ role.name }}</strong> is a system role with limited editing.
      </p>

      <FormLayout @submit.prevent="save">
        <FormField label="Name" for="role-name" :disabled="readOnly || saving">
          <input id="role-name" v-model="form.name" type="text" :disabled="readOnly || saving || role.name === 'member'" />
        </FormField>
        <FormField label="Description" for="role-desc" :disabled="readOnly || saving">
          <input id="role-desc" v-model="form.description" type="text" :disabled="readOnly || saving" />
        </FormField>

        <FormField label="Permissions" component-has-label :disabled="readOnly || saving">
          <table class="perm-table data-table">
            <thead>
              <tr>
                <th class="perm-col-check" scope="col"><span class="a11yhidden">Grant</span></th>
                <th scope="col">Permission</th>
                <th scope="col">Description</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="p in permissionsSorted" :key="p.id">
                <td class="perm-col-check">
                  <input
                    v-model="form.permissionIds"
                    type="checkbox"
                    :value="Number(p.id)"
                    :disabled="readOnly || saving"
                  />
                </td>
                <td><code>{{ p.name }}</code></td>
                <td>{{ p.description || '—' }}</td>
              </tr>
            </tbody>
          </table>
        </FormField>

        <template v-if="canManage && !isSuperuserRole" #actions>
          <button type="submit" class="good" :disabled="saving">{{ saving ? 'Saving…' : 'Save role' }}</button>
        </template>
      </FormLayout>

      <h3 class="subsection-title">Groups with this role</h3>
      <p v-if="!groupNames.length" class="inline-notification note">No groups have this role.</p>
      <p v-else>
        <span v-for="name in groupNames" :key="name" class="role-tag">{{ name }}</span>
      </p>
    </template>
  </Section>
</template>

<style scoped>
.section-title-with-icon {
  display: inline-flex;
  align-items: center;
  gap: 0.45em;
}
.perm-table {
  width: 100%;
}
.perm-col-check {
  width: 2.5rem;
  text-align: center;
}
.subsection-title {
  margin: 1.5rem 0 0.5rem;
}
.role-tag {
  display: inline-block;
  margin: 0 0.35rem 0.35rem 0;
  padding: 0.2rem 0.55rem;
  border-radius: 4px;
  background: #e2e8f0;
  font-size: 0.85rem;
}
</style>
