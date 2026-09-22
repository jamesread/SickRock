<script setup lang="ts">
import { computed, reactive, ref, nextTick } from 'vue'
import FormLayout from 'picocrank/vue/components/FormLayout.vue'
import FormField from 'picocrank/vue/components/FormField.vue'
import type { RbacPermission } from '../gen/sickrock_pb'
import { createApiClient } from '../stores/api'

const props = defineProps<{ permissions: RbacPermission[] }>()
const emit = defineEmits<{ created: [roleId: number] }>()

const dialogEl = ref<HTMLDialogElement | null>(null)
const nameInput = ref<HTMLInputElement | null>(null)
const saving = ref(false)
const errorMessage = ref('')
const form = reactive({
  name: '',
  description: '',
  permissionIds: [] as number[],
})

const permissionsSorted = computed(() =>
  [...props.permissions].sort((a, b) =>
    String(a.name).localeCompare(String(b.name), undefined, { sensitivity: 'base' }),
  ),
)

function resetForm() {
  form.name = ''
  form.description = ''
  form.permissionIds = []
  errorMessage.value = ''
  saving.value = false
}

function open() {
  resetForm()
  dialogEl.value?.showModal()
  nextTick(() => nameInput.value?.focus())
}

function close() {
  dialogEl.value?.close()
}

async function submit() {
  if (!form.name.trim()) {
    errorMessage.value = 'Enter a role name.'
    return
  }

  saving.value = true
  errorMessage.value = ''
  try {
    const client = createApiClient()
    const res = await client.createRbacRole({
      name: form.name.trim(),
      description: form.description.trim(),
      permissionIds: form.permissionIds,
    })
    close()
    emit('created', Number(res.role?.id ?? 0))
  } catch (e: unknown) {
    errorMessage.value = e instanceof Error ? e.message : 'Failed to create role.'
  } finally {
    saving.value = false
  }
}

defineExpose({ open, close })
</script>

<template>
  <dialog ref="dialogEl" class="dialog" @close="resetForm">
    <h2>Create role</h2>
    <p>Define a new RBAC role and choose which permissions it grants.</p>

    <FormLayout @submit.prevent="submit">
      <FormField label="Name" for="new-role-name" :disabled="saving">
        <input
          id="new-role-name"
          ref="nameInput"
          v-model="form.name"
          type="text"
          autocomplete="off"
          :disabled="saving"
          required
        />
      </FormField>

      <FormField label="Description" for="new-role-desc" :disabled="saving">
        <input id="new-role-desc" v-model="form.description" type="text" :disabled="saving" />
      </FormField>

      <FormField label="Permissions" component-has-label :disabled="saving">
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
                  :value="p.id"
                  :disabled="saving"
                  :aria-label="'Grant ' + p.name"
                />
              </td>
              <td><code>{{ p.name }}</code></td>
              <td>{{ p.description || '—' }}</td>
            </tr>
          </tbody>
        </table>
      </FormField>

      <p v-if="errorMessage" class="inline-notification error">{{ errorMessage }}</p>

      <template #actions>
        <button type="button" class="neutral" :disabled="saving" @click="close">Cancel</button>
        <button type="submit" class="good" :disabled="saving || !form.name.trim()">
          {{ saving ? 'Creating…' : 'Create role' }}
        </button>
      </template>
    </FormLayout>
  </dialog>
</template>

<style scoped>
.perm-table {
  width: 100%;
  margin: 0.25rem 0 0;
}

.perm-col-check {
  width: 2.5rem;
  vertical-align: middle;
  text-align: center;
}
</style>
