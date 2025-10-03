<script setup lang="ts">
import { ref, onMounted, computed, inject } from 'vue'
import { useAuthStore } from '../stores/auth'
import { HugeiconsIcon } from '@hugeicons/vue'
import * as Hugeicons from '@hugeicons/core-free-icons'
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

// Bookmarks state
const bookmarks = ref<Array<{
  id: number;
  userId: number;
  navigationItemId: number;
  navigationItem?: {
    id: number;
    ordinal: number;
    tableConfiguration: number;
    tableName: string;
    tableTitle: string;
    tableIcon: string;
    tableView: string;
    dashboardId: number;
    dashboardName: string;
  };
}>>([])

const bookmarksLoading = ref(false)

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

async function loadBookmarks() {
  bookmarksLoading.value = true
  try {
    const response = await client.getUserBookmarks({})
    bookmarks.value = (response.bookmarks || []).map(bookmark => ({
      id: bookmark.id,
      userId: bookmark.userId,
      navigationItemId: bookmark.navigationItemId,
      navigationItem: bookmark.navigationItem ? {
        id: bookmark.navigationItem.id,
        ordinal: bookmark.navigationItem.ordinal,
        tableConfiguration: bookmark.navigationItem.tableConfiguration,
        tableName: bookmark.navigationItem.tableName,
        tableTitle: bookmark.navigationItem.tableTitle,
        tableIcon: bookmark.navigationItem.tableIcon,
        tableView: bookmark.navigationItem.tableView,
        dashboardId: bookmark.navigationItem.dashboardId,
        dashboardName: bookmark.navigationItem.dashboardName
      } : undefined
    }))
  } catch (e: any) {
    console.error('Failed to load bookmarks:', e)
  } finally {
    bookmarksLoading.value = false
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

async function removeBookmark(bookmarkId: number) {
  try {
    await client.deleteUserBookmark({ bookmarkId })
    await loadBookmarks() // Reload bookmarks
  } catch (e: any) {
    console.error('Failed to remove bookmark:', e)
    error.value = String(e?.message || e)
  }
}

onMounted(async () => {
  await Promise.all([
    loadPreferences(),
    loadBookmarks()
  ])
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

        <!-- Bookmarks Section -->
        <div class="bookmarks-section">
          <h3>Bookmarks</h3>

          <!-- Current Bookmarks -->
          <div class="current-bookmarks">
            <h4>Current Bookmarks</h4>
            <div v-if="bookmarksLoading" class="loading">Loading bookmarks...</div>
            <div v-else-if="bookmarks.length === 0" class="no-bookmarks">
              <p>No bookmarks yet. Use the star button in the header to add bookmarks.</p>
            </div>
            <div v-else class="bookmarks-list">
              <div
                v-for="bookmark in bookmarks"
                :key="bookmark.id"
                class="bookmark-item"
              >
                <div class="bookmark-content">
                  <HugeiconsIcon
                    v-if="bookmark.navigationItem?.tableIcon && (Hugeicons as any)[bookmark.navigationItem.tableIcon]"
                    :icon="(Hugeicons as any)[bookmark.navigationItem.tableIcon]"
                    class="bookmark-icon"
                  />
                  <div class="bookmark-info">
                    <span class="bookmark-title">{{ bookmark.navigationItem?.dashboardName || bookmark.navigationItem?.tableTitle || bookmark.navigationItem?.tableName || 'Unknown' }}</span>
                  </div>
                </div>
                <button
                  @click="removeBookmark(bookmark.id)"
                  class="remove-bookmark-btn"
                  title="Remove bookmark"
                >
                  <HugeiconsIcon :icon="Hugeicons.Delete01Icon" />
                </button>
              </div>
            </div>
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

/* Bookmarks Section */
.bookmarks-section {
  margin-top: 2rem;
  padding: 1rem;
  background: #f8f9fa;
  border: 1px solid #e9ecef;
  border-radius: 6px;
}

.bookmarks-section h3 {
  margin: 0 0 1.5rem 0;
  color: #333;
}

.bookmarks-section h4 {
  margin: 0 0 1rem 0;
  color: #555;
  font-size: 1rem;
}

.current-bookmarks {
  margin-bottom: 2rem;
}

.loading {
  color: #666;
  font-style: italic;
}

.no-bookmarks {
  color: #666;
  font-style: italic;
  padding: 1rem;
  text-align: center;
}

/* Bookmarks List */
.bookmarks-list {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.bookmark-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.75rem;
  background: white;
  border: 1px solid #e9ecef;
  border-radius: 6px;
  transition: all 0.2s ease;
}

.bookmark-item:hover {
  border-color: #dee2e6;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

.bookmark-content {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  flex: 1;
}

.bookmark-icon {
  width: 20px;
  height: 20px;
  color: #666;
}

.bookmark-info {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.bookmark-title {
  font-weight: 500;
  color: #333;
}

.remove-bookmark-btn {
  padding: 0.5rem;
  background: #dc3545;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  transition: background-color 0.2s ease;
}

.remove-bookmark-btn:hover {
  background: #c82333;
}
</style>
