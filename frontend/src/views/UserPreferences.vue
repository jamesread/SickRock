<script setup lang="ts">
import RadioGroup from 'picocrank/vue/components/RadioGroup.vue'
import { YES_NO_OPTIONS } from '../utils/fieldTypes'
import { ref, onMounted, computed, inject, watch } from 'vue'
import { useAuthStore } from '../stores/auth'
import type { createApiClient } from '../stores/api'
import { showNotificationPopup } from 'picocrank/vue/composables/useNotificationPopups.js'
import { HugeiconsIcon } from '@hugeicons/vue'
import { Download01Icon, SettingsIcon } from '@hugeicons/core-free-icons'
import Section from 'picocrank/vue/components/Section.vue'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'
import FormLayout from 'picocrank/vue/components/FormLayout.vue'
import FormField from 'picocrank/vue/components/FormField.vue'

const authStore = useAuthStore()
const user = computed(() => authStore.user)
const client = inject<ReturnType<typeof createApiClient>>('apiClient')!

// Profile: the display name shown in the header instead of the username.
const MAX_DISPLAY_NAME_LENGTH = 64
const displayNameDraft = ref(authStore.user?.displayName ?? '')
const savingDisplayName = ref(false)
const displayNameError = ref<string | null>(null)
// Follows the saved name (the user may load after this page) until it is edited.
const displayNameEdited = ref(false)
watch(() => authStore.user?.displayName, (saved) => {
  if (!displayNameEdited.value) displayNameDraft.value = saved ?? ''
})

async function saveDisplayName() {
  if (savingDisplayName.value) return
  savingDisplayName.value = true
  displayNameError.value = null
  try {
    const res = await client.setDisplayName({ displayName: displayNameDraft.value })
    displayNameDraft.value = res.displayName
    displayNameEdited.value = false
    authStore.setDisplayName(res.displayName)
    showNotificationPopup({
      message: res.displayName ? `Display name saved: ${res.displayName}` : 'Display name cleared; your username is shown.',
      class: 'good',
    })
  } catch (e: unknown) {
    displayNameError.value = e instanceof Error ? e.message : String(e)
  } finally {
    savingDisplayName.value = false
  }
}

const loading = ref(true)
const error = ref<string | null>(null)

const preferences = ref({
  theme: 'light',
  language: 'en',
  notifications: true,
  emailUpdates: false
})

async function loadPreferences() {
  loading.value = true
  error.value = null
  try {
    // TODO: Implement actual preferences loading from API
  } catch (e: any) {
    error.value = String(e?.message || e)
  } finally {
    loading.value = false
  }
}

async function savePreferences() {
  try {
    // TODO: Implement actual preferences saving to API
  } catch (e: any) {
    error.value = String(e?.message || e)
  }
}

onMounted(async () => {
  await loadPreferences()
})
</script>

<template>
  <Section title="User Preferences" :icon="SettingsIcon">
    <div v-if="loading" class="muted">Loading preferences…</div>
    <template v-else>
      <NotificationBlock v-if="error" type="error" :message="error" />

      <h3 class="subsection-title">Profile</h3>
      <FormLayout id="profile-form" @submit.prevent="saveDisplayName">
        <FormField
          label="Display name"
          for="display-name"
          :description="`Shown in the header instead of your username (${user?.username ?? ''}). Leave blank to show your username.`"
        >
          <input
            id="display-name"
            v-model="displayNameDraft"
            type="text"
            autocomplete="name"
            :maxlength="MAX_DISPLAY_NAME_LENGTH"
            :placeholder="user?.username ?? ''"
            :disabled="savingDisplayName"
            @input="displayNameEdited = true"
          />
        </FormField>
        <NotificationBlock v-if="displayNameError" type="error" :message="displayNameError" />
        <template #actions>
          <button type="submit" class="good" :disabled="savingDisplayName">
            {{ savingDisplayName ? 'Saving…' : 'Save display name' }}
          </button>
        </template>
      </FormLayout>

      <h3 class="subsection-title">Preferences</h3>
      <FormLayout @submit.prevent="savePreferences">
        <FormField label="Theme" for="theme">
          <select id="theme" v-model="preferences.theme">
            <option value="light">Light</option>
            <option value="dark">Dark</option>
            <option value="auto">Auto</option>
          </select>
        </FormField>

        <FormField label="Language" for="language">
          <select id="language" v-model="preferences.language">
            <option value="en">English</option>
            <option value="es">Spanish</option>
            <option value="fr">French</option>
          </select>
        </FormField>

        <FormField label="Enable notifications" component-has-label>
          <RadioGroup
            variant="boolean"
            :options="YES_NO_OPTIONS"
            v-model="preferences.notifications"
            aria-label="Enable notifications"
          />
        </FormField>

        <FormField label="Email updates" component-has-label>
          <RadioGroup
            variant="boolean"
            :options="YES_NO_OPTIONS"
            v-model="preferences.emailUpdates"
            aria-label="Email updates"
          />
        </FormField>

        <template #actions>
          <button type="submit" class="good">Save Preferences</button>
        </template>
      </FormLayout>

      <h3 class="subsection-title">Account Information</h3>
      <dl class="user-meta">
        <dt>Username</dt>
        <dd>{{ user?.username }}</dd>
        <dt>Display name</dt>
        <dd>{{ user?.displayName || '—' }}</dd>
        <dt>Status</dt>
        <dd>Active</dd>
      </dl>

      <h3 class="subsection-title">PWA & Service Worker</h3>
      <p class="section-hint">Manage Progressive Web App installation and service worker status.</p>
      <router-link to="/admin/pwa-installation" class="button inline-icon good">
        <HugeiconsIcon :icon="Download01Icon" width="1em" height="1em" />
        <span>Open PWA Installation & Service Worker</span>
      </router-link>
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
</style>
