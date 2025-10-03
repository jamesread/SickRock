<script setup lang="ts">
import Sidebar from 'picocrank/vue/components/Sidebar.vue'
import { ref, onMounted, computed, watch, provide } from 'vue'
import { DatabaseIcon, DatabaseSettingIcon, PhoneArrowDownFreeIcons, LogoutIcon, HomeIcon, UserIcon, CheckmarkSquare03Icon, SearchIcon, BookmarkIcon } from '@hugeicons/core-free-icons'
import { createApiClient } from './stores/api'
import Header from 'picocrank/vue/components/Header.vue'
import logo from './resources/images/logo.png'
import QuickSearch from 'picocrank/vue/components/QuickSearch.vue'
import * as Hugeicons from '@hugeicons/core-free-icons'
import { useAuthStore } from './stores/auth'
import { useRouter } from 'vue-router'
import { create } from '@bufbuild/protobuf'
import { InitRequestSchema, ItemSchema } from './gen/sickrock_pb'
import { HugeiconsIcon } from '@hugeicons/vue'

const sidebar = ref(null)
const isSidebarOpen = ref(true)
const SIDEBAR_STATE_KEY = 'sickrock_sidebar_open'
const router = useRouter()
const authStore = useAuthStore()

const isAuthenticated = computed(() => authStore.isAuthenticated)
const user = computed(() => authStore.user)

// Create global API client once and provide it to all components
const apiClient = createApiClient()
provide('apiClient', apiClient)

function persistSidebarState() {
    try { localStorage.setItem(SIDEBAR_STATE_KEY, isSidebarOpen.value ? '1' : '0') } catch {}
}

function toggleSidebar() {
    isSidebarOpen.value = !isSidebarOpen.value
    if (isSidebarOpen.value) sidebar.value.open()
    else sidebar.value.close()
    persistSidebarState()
}

async function handleLogout() {
    await authStore.logout()
    router.push('/login')
}

const pages = ref<Array<{ id: string; title: string; slug: string; view: string; icon: string, path: string }>>([])
const version = ref<string>('')
const quickSearch = ref(null)

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

// Toggle state for header toolbar
const showBookmarks = ref(true) // Default to bookmarks view

async function loadAppData() {
    // Only load data if authenticated
    if (!authStore.isAuthenticated) {
        if (sidebar.value) sidebar.value.close();
        isSidebarOpen.value = false
        persistSidebarState()
        return
    }

    try {
        // Get build info
        const initResponse = await apiClient.init(create(InitRequestSchema , {}))
        version.value = initResponse.version

        const navResponse = await apiClient.getNavigation({})

        // Load bookmarks from navigation response
        bookmarks.value = (navResponse.bookmarks || []).map(bookmark => ({
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

        const sortedItems = [...(navResponse.items || [])].sort((a, b) => (a.ordinal ?? 0) - (b.ordinal ?? 0))
        pages.value = sortedItems
            .map(item => {
                const title = item.title || String(item.id)
                const slug = item.tableName || item.dashboardName || ''
                const icon = item.tableIcon || 'DatabaseIcon'
                const view = item.tableView || ''
                const id = title
                const name = title
                const path = item.dashboardId > 0 ? `/dashboard/${item.dashboardName}` : `/table/${item.tableName}`
                return { id, name, title, slug, icon, view, path }
            })
            .filter(pg => !!pg.slug)

        sidebar.value.clearNavigationLinks()
        if (quickSearch.value) {
            quickSearch.value.clearItems()
        }

        // Add home link
        sidebar.value.addNavigationLink({
            id: 'home',
            name: 'Home',
            title: 'Home',
            path: '/',
            icon: HomeIcon
        })

        if (quickSearch.value) {
            quickSearch.value.addItem({
                id: 'home',
                name: 'Home',
                title: 'Home',
                description: 'Dashboard with recently viewed items',
                category: 'Navigation',
                path: '/',
                type: 'route',
                icon: HomeIcon
            })
        }

        pages.value.forEach(pg => {
            const icon = Hugeicons[pg.icon] || DatabaseIcon

            sidebar.value?.addNavigationLink({
                id: pg.id,
                name: pg.id,
                title: pg.title,
                path: pg.path,
                icon: icon
            })

            quickSearch.value?.addItem({
                id: pg.id,
                name: pg.title,
                title: pg.title,
                description: 'Table: ' + pg.title,
                category: 'Navigation',
                path: `/table/${pg.id}`,
                type: 'route',
                icon: icon
            })
        })

        if (sidebar.value) {
            sidebar.value.addSeparator()
            sidebar.value.addNavigationLink({
                id: 'table-configurations',
                name: 'Table Configurations', title: 'Table Configurations', path: '/table/table_configurations', icon: DatabaseSettingIcon })
            sidebar.value.addNavigationLink({
                id: 'nav-items',
                name: 'Navigation', title: 'Navigation', path: '/table/table_navigation', icon: DatabaseSettingIcon })
            sidebar.value.addRouterLink('table-create')
            sidebar.value.addRouterLink('control-panel')
            sidebar.value.addRouterLink('device-code-claimer')
            sidebar.value.addCallback('Logout', async () => { await handleLogout() }, LogoutIcon)
            sidebar.value.stick()
            // Restore sidebar state from localStorage (default open)
            try {
                const stored = localStorage.getItem(SIDEBAR_STATE_KEY)
                isSidebarOpen.value = (stored == null ? true : stored === '1')
            } catch { isSidebarOpen.value = true }
            if (isSidebarOpen.value) sidebar.value.open()
            else sidebar.value.close()
        }
    } catch (error) {
        console.error('Failed to load data:', error)
        // If we get an auth error, redirect to login
        if (error.code === 'unauthenticated') {
            router.push('/login')
        }
    }
}

// Watch for authentication changes and reload data
watch(isAuthenticated, (newValue) => {
    if (newValue) {
        loadAppData()
    } else {
        // Clear data when logged out
        pages.value = []
        version.value = ''
        if (sidebar.value && typeof sidebar.value.clearNavigationLinks === 'function') {
            sidebar.value.clearNavigationLinks()
        }
        if (quickSearch.value && typeof quickSearch.value.clearItems === 'function') {
            quickSearch.value.clearItems()
        }
    }
})

// Bookmark management
const currentRoute = computed(() => router.currentRoute.value)

const isCurrentPageBookmarked = computed(() => {
    if (!currentRoute.value.name) return false

    // Check if current route matches any bookmarked navigation item
    const currentPath = currentRoute.value.path
    return bookmarks.value.some(bookmark => {
        if (!bookmark.navigationItem) return false

        const bookmarkPath = bookmark.navigationItem.dashboardId > 0
            ? `/dashboard/${bookmark.navigationItem.dashboardName}`
            : `/table/${bookmark.navigationItem.tableName}`

        return bookmarkPath === currentPath
    })
})

async function toggleCurrentPageBookmark() {
    if (!currentRoute.value.name) return

    const currentPath = currentRoute.value.path

    // Find matching navigation item
    const navResponse = await apiClient.getNavigation({})
    const matchingItem = navResponse.items?.find(item => {
        const itemPath = item.dashboardId > 0
            ? `/dashboard/${item.dashboardName}`
            : `/table/${item.tableName}`
        return itemPath === currentPath
    })

    if (!matchingItem) {
        console.warn('No matching navigation item found for current page')
        return
    }

    try {
        if (isCurrentPageBookmarked.value) {
            // Remove bookmark
            const bookmark = bookmarks.value.find(b => b.navigationItemId === matchingItem.id)
            if (bookmark) {
                await apiClient.deleteUserBookmark({ bookmarkId: bookmark.id })
                // Reload bookmarks
                await loadAppData()
            }
        } else {
            // Add bookmark
            await apiClient.createUserBookmark({ navigationItemId: matchingItem.id })
            // Reload bookmarks
            await loadAppData()
        }
    } catch (error) {
        console.error('Failed to toggle bookmark:', error)
    }
}

function toggleToolbarView() {
    showBookmarks.value = !showBookmarks.value
}

onMounted(async () => {
    await loadAppData()
})
</script>

<template>
    <Header
        title = "SickRock"
        :logoUrl = "logo"
        :username = "user?.username"
        @toggleSidebar = "toggleSidebar"
        v-if="isAuthenticated">

        <template #toolbar>
            <div class="toolbar-content">
                <!-- Toggle Button -->
                <button
                    @click="toggleToolbarView"
                    class="toolbar-toggle-button"
                    :title="showBookmarks ? 'Switch to Search' : 'Switch to Bookmarks'"
                >
                    <HugeiconsIcon :icon="showBookmarks ? Hugeicons.SearchIcon : Hugeicons.BookmarkIcon" />
                </button>

                <!-- Bookmarks List -->
                <div v-if="showBookmarks" class="bookmarks-toolbar">
                    <div v-if="bookmarks.length === 0" class="no-bookmarks-toolbar">
                        No bookmarks yet
                    </div>
                    <div v-else class="bookmarks-list-toolbar">
                        <router-link
                            v-for="bookmark in bookmarks.slice(0, 5)"
                            :key="bookmark.id"
                            :to="bookmark.navigationItem?.dashboardId > 0
                                ? `/dashboard/${bookmark.navigationItem.dashboardName}`
                                : `/table/${bookmark.navigationItem?.tableName}`"
                            class="bookmark-toolbar-item"
                            :title="bookmark.navigationItem?.dashboardName || bookmark.navigationItem?.tableTitle || bookmark.navigationItem?.tableName"
                        >
                            <HugeiconsIcon
                                v-if="bookmark.navigationItem?.tableIcon && (Hugeicons as any)[bookmark.navigationItem.tableIcon]"
                                :icon="(Hugeicons as any)[bookmark.navigationItem.tableIcon]"
                                class="bookmark-toolbar-icon"
                            />
                            <span class="bookmark-toolbar-text">
                                {{ bookmark.navigationItem?.dashboardName || bookmark.navigationItem?.tableTitle || bookmark.navigationItem?.tableName }}
                            </span>
                        </router-link>
                    </div>
                </div>

                <!-- Search Component -->
                <div v-if="!showBookmarks" class="search-toolbar">
                    <QuickSearch
                        ref="quickSearch"
                        :search-fields="['title']"
                    />
                </div>

                <!-- Hidden QuickSearch for initialization -->
                <div v-else style="display: none;">
                    <QuickSearch
                        ref="quickSearch"
                        :search-fields="['title']"
                    />
                </div>
            </div>
        </template>

        <template #user-info>
            <button
                @click="toggleCurrentPageBookmark"
                class="bookmark-button"
                :class="{ 'bookmarked': isCurrentPageBookmarked }"
                :title="isCurrentPageBookmarked ? 'Remove bookmark' : 'Add bookmark'"
            >
                <HugeiconsIcon :icon="Hugeicons.CheckmarkSquare03Icon" />
            </button>
            <router-link
                to="/user-preferences"
                class="user-preferences-button"
                title="User Preferences"
            >
                <HugeiconsIcon :icon="Hugeicons.UserIcon" />
                {{ user?.username }}
            </router-link>
        </template>
    </Header>

    <div id="layout">
        <Sidebar v-if="isAuthenticated" ref="sidebar" />
        <div id="content">
            <main>
                <router-view :key="$route.path" />
            </main>
            <footer v-if="version">
                <span>SickRock</span>
                <span>{{ version }}</span>
            </footer>
        </div>
    </div>
</template>

<style scoped>
.bookmark-button {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 32px;
    height: 32px;
    padding: 0;
    background: transparent;
    color: white;
    border: 1px solid transparent;
    border-radius: 4px;
    cursor: pointer;
    transition: all 0.2s ease;
    margin-right: 0.5rem;
}

.bookmark-button:hover {
    background: rgba(255, 255, 255, 0.1);
    border-color: rgba(255, 255, 255, 0.2);
    color: white;
    transform: translateY(-1px);
}

.bookmark-button.bookmarked {
    background: rgba(40, 167, 69, 0.2);
    color: #28a745;
    border-color: rgba(40, 167, 69, 0.3);
}

.bookmark-button.bookmarked:hover {
    background: rgba(40, 167, 69, 0.3);
    border-color: rgba(40, 167, 69, 0.4);
    transform: translateY(-1px);
}

.user-preferences-button {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.5rem 0.75rem;
    background: transparent;
    color: white;
    text-decoration: none;
    border: 1px solid transparent;
    border-radius: 4px;
    font-size: 0.875rem;
    font-weight: 500;
    transition: all 0.2s ease;
}

.user-preferences-button:hover {
    background: rgba(255, 255, 255, 0.1);
    border-color: rgba(255, 255, 255, 0.2);
    color: white;
    text-decoration: none;
    transform: translateY(-1px);
}

.user-preferences-button:focus {
    outline: 2px solid #007bff;
    outline-offset: 2px;
}

/* Toolbar Styles */
.toolbar-content {
    display: flex;
    align-items: center;
    gap: 0.75rem;
}

.toolbar-toggle-button {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 32px;
    height: 32px;
    padding: 0;
    background: transparent;
    color: white;
    border: 1px solid transparent;
    border-radius: 4px;
    cursor: pointer;
    transition: all 0.2s ease;
}

.toolbar-toggle-button:hover {
    background: rgba(255, 255, 255, 0.1);
    border-color: rgba(255, 255, 255, 0.2);
    color: white;
    transform: translateY(-1px);
}

.bookmarks-toolbar {
    display: flex;
    align-items: center;
    gap: 0.5rem;
}

.no-bookmarks-toolbar {
    color: rgba(255, 255, 255, 0.6);
    font-size: 0.875rem;
    font-style: italic;
}

.bookmarks-list-toolbar {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    max-width: 400px;
    overflow-x: auto;
}

.bookmark-toolbar-item {
    display: flex;
    align-items: center;
    gap: 0.25rem;
    padding: 0.25rem 0.5rem;
    background: rgba(255, 255, 255, 0.1);
    color: white;
    text-decoration: none;
    border: 1px solid rgba(255, 255, 255, 0.2);
    border-radius: 4px;
    font-size: 0.8rem;
    white-space: nowrap;
    transition: all 0.2s ease;
}

.bookmark-toolbar-item:hover {
    background: rgba(255, 255, 255, 0.2);
    border-color: rgba(255, 255, 255, 0.3);
    color: white;
    text-decoration: none;
    transform: translateY(-1px);
}

.bookmark-toolbar-icon {
    width: 14px;
    height: 14px;
}

.bookmark-toolbar-text {
    max-width: 120px;
    overflow: hidden;
    text-overflow: ellipsis;
}

.search-toolbar {
    flex: 1;
}
</style>
