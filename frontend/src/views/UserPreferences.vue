<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useAuthStore } from '../stores/auth'
import { HugeiconsIcon } from '@hugeicons/vue'
import * as Hugeicons from '@hugeicons/core-free-icons'
import Section from 'picocrank/vue/components/Section.vue'
import FormLayout from 'picocrank/vue/components/FormLayout.vue'
import FormField from 'picocrank/vue/components/FormField.vue'

const authStore = useAuthStore()
const user = computed(() => authStore.user)

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
    console.log('Loading user preferences...')
  } catch (e: any) {
    error.value = String(e?.message || e)
  } finally {
    loading.value = false
  }
}

async function savePreferences() {
  try {
    // TODO: Implement actual preferences saving to API
    console.log('Saving preferences:', preferences.value)
  } catch (e: any) {
    error.value = String(e?.message || e)
  }
}

onMounted(async () => {
  await loadPreferences()
})
</script>

<template>
  <Section title="User Preferences" :icon="Hugeicons.SettingsIcon">
    <div v-if="loading" class="muted">Loading preferences…</div>
    <template v-else>
      <p v-if="error" class="inline-notification error">{{ error }}</p>

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

        <FormField label="Enable notifications" for="notifications">
          <input id="notifications" type="checkbox" v-model="preferences.notifications" />
        </FormField>

        <FormField label="Email updates" for="email-updates">
          <input id="email-updates" type="checkbox" v-model="preferences.emailUpdates" />
        </FormField>

        <template #actions>
          <button type="submit" class="good">Save Preferences</button>
        </template>
      </FormLayout>

      <h3 class="subsection-title">Account Information</h3>
      <dl class="user-meta">
        <dt>Username</dt>
        <dd>{{ user?.username }}</dd>
        <dt>Status</dt>
        <dd>Active</dd>
      </dl>

      <h3 class="subsection-title">PWA & Service Worker</h3>
      <p class="section-hint">Manage Progressive Web App installation and service worker status.</p>
      <router-link to="/admin/pwa-installation" class="button inline-icon good">
        <HugeiconsIcon :icon="Hugeicons.Download01Icon" width="1em" height="1em" />
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
