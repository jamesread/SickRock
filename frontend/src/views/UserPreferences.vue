<script setup lang="ts">
import { ref, onMounted, computed, inject } from 'vue'
import { useAuthStore } from '../stores/auth'
import { HugeiconsIcon } from '@hugeicons/vue'
import * as Hugeicons from '@hugeicons/core-free-icons'
import { Download01Icon } from '@hugeicons/core-free-icons'
import type { createApiClient } from '../stores/api'
import Section from 'picocrank/vue/components/Section.vue'

// Use global API client
const client = inject<ReturnType<typeof createApiClient>>('apiClient')
const authStore = useAuthStore()
const user = computed(() => authStore.user)

const loading = ref(true)
const error = ref<string | null>(null)

// User preferences state
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
    // For now, using default values
    console.log('Loading user preferences...')
  } catch (e: any) {
    error.value = String(e?.message || e)
  } finally {
    loading.value = false
  }
}

function formatSWState(state: string | null): string {
  if (!state) return 'Unknown'
  const stateMap: Record<string, string> = {
    'installing': 'Installing',
    'installed': 'Installed',
    'activating': 'Activating',
    'activated': 'Active',
    'redundant': 'Redundant'
  }
  return stateMap[state] || state.charAt(0).toUpperCase() + state.slice(1)
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
  <Section title="User Preferences">
    <div v-if="loading">Loading preferences…</div>
    <div v-else>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-else>
        <div class="preferences-form">
          <div class="form-group">
            <label for="theme">Theme</label>
            <select id="theme" v-model="preferences.theme">
              <option value="light">Light</option>
              <option value="dark">Dark</option>
              <option value="auto">Auto</option>
            </select>
          </div>

          <div class="form-group">
            <label for="language">Language</label>
            <select id="language" v-model="preferences.language">
              <option value="en">English</option>
              <option value="es">Spanish</option>
              <option value="fr">French</option>
            </select>
          </div>

          <div class="form-group">
            <label class="checkbox-label">
              <input type="checkbox" v-model="preferences.notifications" />
              Enable notifications
            </label>
          </div>

          <div class="form-group">
            <label class="checkbox-label">
              <input type="checkbox" v-model="preferences.emailUpdates" />
              Email updates
            </label>
          </div>

          <div class="form-actions">
            <button @click="savePreferences" class="save-button">
              Save Preferences
            </button>
          </div>
        </div>

        <div class="user-info">
          <h3>Account Information</h3>
          <p><strong>Username:</strong> {{ user?.username }}</p>
          <p><strong>Status:</strong> Active</p>
        </div>

        <!-- PWA & Service Worker Section -->
        <div class="pwa-section">
          <h3>PWA & Service Worker</h3>
          <p>Manage Progressive Web App installation and service worker status.</p>
          <div class="pwa-actions">
            <router-link to="/admin/pwa-installation" class="pwa-link-button">
              <HugeiconsIcon :icon="Hugeicons.Download01Icon" />
              Open PWA Installation & Service Worker
            </router-link>
          </div>
        </div>
      </div>
    </div>
  </Section>
</template>

<style scoped>
.preferences-form {
  margin-bottom: 2rem;
}

.form-group {
  margin-bottom: 1rem;
}

.form-group label {
  display: block;
  margin-bottom: 0.5rem;
  font-weight: 500;
}

.form-group select {
  width: 100%;
  max-width: 300px;
  padding: 0.5rem;
  border: 1px solid #ccc;
  border-radius: 4px;
  background: white;
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  cursor: pointer;
}

.checkbox-label input[type="checkbox"] {
  margin: 0;
}

.form-actions {
  margin-top: 1.5rem;
}

.save-button {
  padding: 0.75rem 1.5rem;
  background: #007bff;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-weight: 500;
  transition: background-color 0.2s ease;
}

.save-button:hover {
  background: #0056b3;
}

.user-info {
  padding: 1rem;
  background: #f8f9fa;
  border: 1px solid #e9ecef;
  border-radius: 6px;
}

.user-info h3 {
  margin: 0 0 1rem 0;
  color: #333;
}

.user-info p {
  margin: 0.5rem 0;
  color: #666;
}

.error {
  background: #f8d7da;
  color: #721c24;
  padding: 0.75rem;
  border: 1px solid #f5c6cb;
  border-radius: 4px;
  margin-bottom: 1rem;
}


/* PWA Section */
.pwa-section {
  margin-top: 2rem;
  padding: 1rem;
  background: #f8f9fa;
  border: 1px solid #e9ecef;
  border-radius: 6px;
}

.pwa-section h3 {
  margin: 0 0 0.5rem 0;
  color: #333;
}

.pwa-section p {
  margin: 0 0 1rem 0;
  color: #666;
  font-size: 0.9rem;
}

.pwa-actions {
  display: flex;
  gap: 1rem;
}

.pwa-link-button {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.75rem 1.5rem;
  background: #007bff;
  color: white;
  text-decoration: none;
  border-radius: 4px;
  font-weight: 500;
  transition: background-color 0.2s ease;
}

.pwa-link-button:hover {
  background: #0056b3;
  color: white;
  text-decoration: none;
}

.pwa-link-button svg {
  width: 18px;
  height: 18px;
}
</style>
