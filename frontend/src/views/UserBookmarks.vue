<script setup lang="ts">
import { ref, onMounted, inject } from 'vue'
import { useRouter } from 'vue-router'
import { HugeiconsIcon } from '@hugeicons/vue'
import * as Hugeicons from '@hugeicons/core-free-icons'
import { DatabaseIcon, Delete01Icon } from '@hugeicons/core-free-icons'
import type { createApiClient } from '../stores/api'
import Section from 'picocrank/vue/components/Section.vue'

// Use global API client
const client = inject<ReturnType<typeof createApiClient>>('apiClient')
const router = useRouter()

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
    icon: string;
    tableView: string;
    dashboardId: number;
    dashboardName: string;
    workflowId: number;
  };
}>>([])

const bookmarksLoading = ref(false)
const error = ref<string | null>(null)

async function loadBookmarks() {
  bookmarksLoading.value = true
  error.value = null
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
        icon: bookmark.navigationItem.icon,
        tableView: bookmark.navigationItem.tableView,
        dashboardId: bookmark.navigationItem.dashboardId,
        dashboardName: bookmark.navigationItem.dashboardName,
        workflowId: bookmark.navigationItem.workflowId || 0
      } : undefined
    }))
  } catch (e: any) {
    console.error('Failed to load bookmarks:', e)
    error.value = String(e?.message || e)
  } finally {
    bookmarksLoading.value = false
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

function navigateToBookmark(bookmark: typeof bookmarks.value[0]) {
  if (!bookmark.navigationItem) return

  let path = ''
  if (bookmark.navigationItem.dashboardId > 0) {
    path = `/dashboard/${bookmark.navigationItem.dashboardName}`
  } else if (bookmark.navigationItem.workflowId && bookmark.navigationItem.workflowId > 0) {
    path = `/workflow/${bookmark.navigationItem.workflowId}`
  } else {
    path = `/table/${bookmark.navigationItem.tableName}`
  }

  router.push(path)
}

onMounted(async () => {
  await loadBookmarks()
})
</script>

<template>
  <Section title="Bookmarks">
    <div v-if="error" class="error">{{ error }}</div>

    <div v-if="bookmarksLoading" class="loading">Loading bookmarks...</div>
    <div v-else-if="bookmarks.length === 0" class="no-bookmarks">
      <p>No bookmarks yet. Use the bookmark button in the header to add bookmarks to pages you visit.</p>
    </div>
    <div v-else class="bookmarks-list">
      <div
        v-for="bookmark in bookmarks"
        :key="bookmark.id"
        class="bookmark-item"
      >
        <div class="bookmark-content" @click="navigateToBookmark(bookmark)">
          <HugeiconsIcon
            :icon="(bookmark.navigationItem?.icon && (Hugeicons as any)[bookmark.navigationItem.icon])
                ? (Hugeicons as any)[bookmark.navigationItem.icon]
                : DatabaseIcon"
            class="bookmark-icon"
          />
          <div class="bookmark-info">
            <span class="bookmark-title">{{ bookmark.navigationItem?.tableName || bookmark.navigationItem?.dashboardName || bookmark.navigationItem?.workflowId || 'Unknown' }}</span>
            <span v-if="bookmark.navigationItem?.tableTitle" class="bookmark-subtitle">{{ bookmark.navigationItem.tableTitle }}</span>
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
  </Section>
</template>

<style scoped>
.error {
  background: #f8d7da;
  color: #721c24;
  padding: 0.75rem;
  border: 1px solid #f5c6cb;
  border-radius: 4px;
  margin-bottom: 1rem;
}

.loading {
  color: #666;
  font-style: italic;
  padding: 1rem;
}

.no-bookmarks {
  color: #666;
  font-style: italic;
  padding: 2rem;
  text-align: center;
  background: #f8f9fa;
  border: 1px solid #e9ecef;
  border-radius: 6px;
}

.bookmarks-list {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.bookmark-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1rem;
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
  cursor: pointer;
}

.bookmark-icon {
  width: 24px;
  height: 24px;
  color: #666;
  flex-shrink: 0;
}

.bookmark-info {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.bookmark-title {
  font-weight: 500;
  color: #333;
  font-size: 1rem;
}

.bookmark-subtitle {
  font-size: 0.875rem;
  color: #6c757d;
}

.remove-bookmark-btn {
  padding: 0.5rem;
  background: #dc3545;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  transition: background-color 0.2s ease;
  flex-shrink: 0;
}

.remove-bookmark-btn:hover {
  background: #c82333;
}

.remove-bookmark-btn :deep(svg) {
  width: 16px;
  height: 16px;
}
</style>
