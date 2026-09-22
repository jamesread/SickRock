<script setup lang="ts">
import { ref, reactive, nextTick } from 'vue'
import FormLayout from 'picocrank/vue/components/FormLayout.vue'
import FormField from 'picocrank/vue/components/FormField.vue'
import { createApiClient } from '../stores/api'

const emit = defineEmits<{ created: [userId: number] }>()

const dialogEl = ref<HTMLDialogElement | null>(null)
const nameInput = ref<HTMLInputElement | null>(null)
const saving = ref(false)
const errorMessage = ref('')
const form = reactive({ username: '', password: '' })

function resetForm() {
  form.username = ''
  form.password = ''
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
  const username = form.username.trim()
  if (!username) {
    errorMessage.value = 'Username is required.'
    return
  }
  if (form.password && form.password.length < 8) {
    errorMessage.value = 'Password must be at least 8 characters.'
    return
  }

  saving.value = true
  errorMessage.value = ''
  try {
    const client = createApiClient()
    const res = await client.createUser({
      username,
      password: form.password,
    })
    close()
    emit('created', Number(res.user?.id ?? 0))
  } catch (e: unknown) {
    errorMessage.value = e instanceof Error ? e.message : 'Failed to create user.'
  } finally {
    saving.value = false
  }
}

defineExpose({ open, close })
</script>

<template>
  <dialog ref="dialogEl" class="dialog" @close="resetForm">
    <h2>Create user</h2>
    <p>Create a new user account. Password is optional — omit it to disable interactive login until a password is set.</p>

    <FormLayout @submit.prevent="submit">
      <FormField label="Username" for="new-user-name" :disabled="saving">
        <input
          id="new-user-name"
          ref="nameInput"
          v-model="form.username"
          type="text"
          autocomplete="off"
          :disabled="saving"
          required
        />
      </FormField>

      <FormField label="Password" for="new-user-password" :disabled="saving">
        <input
          id="new-user-password"
          v-model="form.password"
          type="password"
          autocomplete="new-password"
          :disabled="saving"
          placeholder="Optional (min 8 characters)"
        />
      </FormField>

      <p v-if="errorMessage" class="inline-notification error">{{ errorMessage }}</p>

      <template #actions>
        <button type="button" class="neutral" :disabled="saving" @click="close">Cancel</button>
        <button type="submit" class="good" :disabled="saving || !form.username.trim()">
          {{ saving ? 'Creating…' : 'Create user' }}
        </button>
      </template>
    </FormLayout>
  </dialog>
</template>
