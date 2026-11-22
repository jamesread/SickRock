<script setup lang="ts">
// Sends a notification to other users (NotifyUsers): those who share a group with the sender.
// Each recipient gets it on the channels they subscribed to "Notified by another user".
import { computed, ref } from 'vue'
import Dialog from 'picocrank/vue/components/Dialog.vue'
import FormLayout from 'picocrank/vue/components/FormLayout.vue'
import FormField from 'picocrank/vue/components/FormField.vue'
import CheckGroup from 'picocrank/vue/components/CheckGroup.vue'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'
import { createApiClient } from '../stores/api'
import type { NotifiableUser, NotifyUserResult } from '../gen/sickrock_pb'
import type { DialogApi } from '../utils/dialog'
import { notifyResultText, NOTIFY_MAX_MESSAGE_LENGTH, NOTIFY_MAX_TITLE_LENGTH } from '../utils/notifyUsers'

const props = defineProps<{
  /** Pre-filled title, e.g. "Meal plan for 2 Nov – 8 Nov is ready". */
  title: string
  /** Pre-filled message. */
  message?: string
  /** Path on this site the notification links to. */
  linkPath?: string
  /** Users ticked when the dialog opens. */
  defaultRecipientIds?: number[]
}>()

const client = createApiClient()
const dialogEl = ref<DialogApi | null>(null)
const users = ref<NotifiableUser[]>([])
const recipients = ref<number[]>([])
const titleText = ref('')
const messageText = ref('')
const loading = ref(false)
const sending = ref(false)
const errorMessage = ref('')
const results = ref<NotifyUserResult[] | null>(null)
const formId = `notify-users-form-${Math.random().toString(36).slice(2)}`

const userOptions = computed(() => users.value.map(user => ({ value: user.id, label: user.username })))
const canSend = computed(() => !sending.value && recipients.value.length > 0 && titleText.value.trim() !== '')

async function open() {
  titleText.value = props.title
  messageText.value = props.message ?? ''
  errorMessage.value = ''
  results.value = null
  dialogEl.value?.show()
  loading.value = true
  try {
    const res = await client.listNotifiableUsers({})
    users.value = res.users
    const known = new Set(res.users.map(user => user.id))
    recipients.value = (props.defaultRecipientIds ?? []).filter(id => known.has(id))
  } catch (e: unknown) {
    users.value = []
    errorMessage.value = e instanceof Error ? e.message : 'Could not load users.'
  } finally {
    loading.value = false
  }
}

async function send() {
  if (!canSend.value) return
  sending.value = true
  errorMessage.value = ''
  try {
    const res = await client.notifyUsers({
      recipientUserIds: recipients.value,
      title: titleText.value.trim(),
      message: messageText.value.trim(),
      linkPath: props.linkPath ?? '',
    })
    results.value = res.results
  } catch (e: unknown) {
    errorMessage.value = e instanceof Error ? e.message : 'Could not send the notification.'
  } finally {
    sending.value = false
  }
}

defineExpose({ open })
</script>

<template>
  <Dialog ref="dialogEl" title="Notify">
    <template v-if="results">
      <ul class="notify-results">
        <li v-for="result in results" :key="result.userId" :class="`notify-${result.status}`">
          {{ notifyResultText(result) }}
        </li>
      </ul>
    </template>
    <FormLayout v-else :id="formId" @submit.prevent="send">
      <FormField label="Notify" :disabled="sending">
        <p v-if="loading" class="subtle">Loading…</p>
        <p v-else-if="users.length === 0 && !errorMessage" class="subtle">
          There is nobody to notify: you can notify people who share a user group with you.
        </p>
        <CheckGroup v-else v-model="recipients" :options="userOptions" aria-label="People to notify" :disabled="sending" />
        <small class="subtle">They get it on the channels they chose for <em>Notified by another user</em> in their notification settings.</small>
      </FormField>
      <FormField label="Title" for="notify-title" :disabled="sending">
        <input id="notify-title" v-model="titleText" type="text" :maxlength="NOTIFY_MAX_TITLE_LENGTH" required :disabled="sending" />
      </FormField>
      <FormField label="Message" for="notify-message" :disabled="sending">
        <textarea id="notify-message" v-model="messageText" rows="3" :maxlength="NOTIFY_MAX_MESSAGE_LENGTH" placeholder="Optional" :disabled="sending"></textarea>
      </FormField>
      <NotificationBlock v-if="errorMessage" type="error" :message="errorMessage" />
    </FormLayout>

    <template v-if="!results" #actions>
      <button type="submit" :form="formId" class="good" :disabled="!canSend">
        {{ sending ? 'Sending…' : 'Send' }}
      </button>
    </template>
  </Dialog>
</template>

<style scoped>
.notify-results {
  margin: 0;
  padding-left: 1.2em;
}

.notify-not_subscribed,
.notify-failed {
  color: var(--karma-warning, inherit);
}
</style>
