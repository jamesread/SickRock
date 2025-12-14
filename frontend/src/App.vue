<script setup lang="ts">
import Sidebar from 'picocrank/vue/components/Sidebar.vue'
import { ref, onMounted, onUnmounted, computed, watch, provide, nextTick } from 'vue'
import { DatabaseIcon, DatabaseSettingIcon, PhoneArrowDownFreeIcons, LogoutIcon, HomeIcon, UserIcon, CheckmarkSquare03Icon, SearchIcon, BookmarkIcon, QuestionIcon } from '@hugeicons/core-free-icons'
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
import { useKeyboardShortcuts, type KeyboardShortcut } from './composables/useKeyboardShortcuts'
import KeyboardShortcutsHelp from './components/KeyboardShortcutsHelp.vue'

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
  title: string;
  navigationItem?: {
    id: number;
    ordinal: number;
    tableConfiguration: number;
    tableName: string;
    icon: string;
    tableView: string;
    dashboardId: number;
    dashboardName: string;
  };
}>>([])

// Toggle state for header toolbar
const showBookmarks = ref(true) // Default to bookmarks view

// Keyboard shortcuts state
const showShortcutsHelp = ref(false)
const gKeyPressed = ref(false)
let gKeyTimeout: ReturnType<typeof setTimeout> | null = null

// G key overlay state
const showGKeyOverlay = ref(false)
const gKeySecondKey = ref<string | null>(null)
const gKeyAction = ref<string | null>(null)
const gKeyError = ref(false)
let gKeyOverlayTimeout: ReturnType<typeof setTimeout> | null = null

// Provide a way for child components to request focus on table filter
const tableFilterFocusRequest = ref<(() => void) | null>(null)
provide('tableFilterFocusRequest', tableFilterFocusRequest)

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
          title: bookmark.title,
          navigationItem: bookmark.navigationItem ? {
            id: bookmark.navigationItem.id,
            ordinal: bookmark.navigationItem.ordinal,
            tableConfiguration: bookmark.navigationItem.tableConfiguration,
            tableName: bookmark.navigationItem.tableName,
            icon: bookmark.navigationItem.icon,
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
                const icon = item.icon || 'DatabaseIcon'
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

        // Add keyboard shortcuts help item
        if (quickSearch.value) {
            quickSearch.value.addItem({
                id: 'keyboard-shortcuts',
                name: 'Keyboard Shortcuts',
                title: 'Keyboard Shortcuts',
                description: 'View all available keyboard shortcuts (g then ?)',
                category: 'Help',
                path: '#keyboard-shortcuts',
                type: 'action',
                icon: QuestionIcon
            })
        }

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

// Watch for keyboard shortcuts navigation from QuickSearch
watch(() => router.currentRoute.value.hash, (hash) => {
    if (hash === '#keyboard-shortcuts') {
        showShortcutsHelp.value = true
        // Remove the hash from URL
        router.replace({ ...router.currentRoute.value, hash: '' })
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

// Keyboard shortcuts handlers
function focusQuickSearch() {
    // Switch to search view if showing bookmarks
    if (showBookmarks.value) {
        showBookmarks.value = false
        nextTick(() => {
            // Try to focus the QuickSearch input
            const searchInput = document.querySelector('.quick-search input, [class*="quick-search"] input') as HTMLInputElement
            if (searchInput) {
                searchInput.focus()
            } else if (quickSearch.value && typeof (quickSearch.value as any).focus === 'function') {
                (quickSearch.value as any).focus()
            }
        })
    } else {
        nextTick(() => {
            const searchInput = document.querySelector('.quick-search input, [class*="quick-search"] input') as HTMLInputElement
            if (searchInput) {
                searchInput.focus()
            } else if (quickSearch.value && typeof (quickSearch.value as any).focus === 'function') {
                (quickSearch.value as any).focus()
            }
        })
    }
}

function openQuickAdd() {
    // Emit event that TableComponent can listen to
    window.dispatchEvent(new CustomEvent('open-quick-add'))
}

function focusTableFilter() {
    // Request focus on table filter if available
    if (tableFilterFocusRequest.value) {
        tableFilterFocusRequest.value()
    } else {
        // Fallback: try to find any filter input in the current view
        nextTick(() => {
            const filterInput = document.querySelector('input[type="search"], input[placeholder*="filter" i], input[placeholder*="search" i]') as HTMLInputElement
            if (filterInput) {
                filterInput.focus()
            }
        })
    }
}

function saveCurrentEdit() {
    // Dispatch event for table component to handle
    window.dispatchEvent(new CustomEvent('save-current-edit'))
}

function handleGKey() {
    if (gKeyTimeout) {
        clearTimeout(gKeyTimeout)
    }
    if (gKeyOverlayTimeout) {
        clearTimeout(gKeyOverlayTimeout)
    }

    gKeyPressed.value = true
    showGKeyOverlay.value = true
    gKeySecondKey.value = null
    gKeyAction.value = null
    gKeyError.value = false

    gKeyTimeout = setTimeout(() => {
        gKeyPressed.value = false
        if (!gKeySecondKey.value) {
            // Only hide overlay if no second key was pressed
            showGKeyOverlay.value = false
        }
    }, 1000) // 1 second timeout for the second key
}

function getActionDescription(key: string): string {
    switch (key) {
        case 'h':
            return 'Go to Home'
        case 't':
            return 'Go to Tables'
        case 'c':
            return 'Go to Control Panel'
        case '?':
            return 'Show Keyboard Shortcuts'
        default:
            return ''
    }
}

function isValidNavigationKey(key: string): boolean {
    return ['h', 't', 'c', '?'].includes(key)
}

function handleNavigationKey(key: string) {
    if (!gKeyPressed.value) {
        // If 'g' wasn't pressed, handle as regular key
        if (key === 'b') {
            toggleToolbarView()
        }
        return
    }

    // Show the second key in overlay
    gKeySecondKey.value = key

    // Check if it's a valid navigation key
    const isValid = isValidNavigationKey(key)

    if (isValid) {
        // Valid key - show action description
        gKeyAction.value = getActionDescription(key)
        gKeyError.value = false

        // Clear the g key state
        if (gKeyTimeout) {
            clearTimeout(gKeyTimeout)
            gKeyTimeout = null
        }
        gKeyPressed.value = false

        // Hide overlay after 0.5 seconds
        if (gKeyOverlayTimeout) {
            clearTimeout(gKeyOverlayTimeout)
        }
        gKeyOverlayTimeout = setTimeout(() => {
            showGKeyOverlay.value = false
            gKeySecondKey.value = null
            gKeyAction.value = null
            gKeyError.value = false
        }, 500)

        // Handle navigation
        switch (key) {
            case 'h':
                router.push('/')
                break
            case 't':
                // Navigate to first table or home (which shows tables)
                router.push('/')
                break
            case 'c':
                router.push('/admin/control-panel')
                break
            case '?':
                showShortcutsHelp.value = true
                break
        }
    } else {
        // Invalid key - show error
        gKeyAction.value = null
        gKeyError.value = true

        // Clear the g key state
        if (gKeyTimeout) {
            clearTimeout(gKeyTimeout)
            gKeyTimeout = null
        }
        gKeyPressed.value = false

        // Hide overlay after 0.5 seconds
        if (gKeyOverlayTimeout) {
            clearTimeout(gKeyOverlayTimeout)
        }
        gKeyOverlayTimeout = setTimeout(() => {
            showGKeyOverlay.value = false
            gKeySecondKey.value = null
            gKeyAction.value = null
            gKeyError.value = false
        }, 500)
    }
}

// Set up keyboard shortcuts
const shortcuts = ref<KeyboardShortcut[]>([
    {
        key: 'k',
        ctrl: true,
        handler: () => focusQuickSearch(),
        description: 'Focus QuickSearch'
    },
    {
        key: '/',
        ctrl: true,
        handler: () => focusQuickSearch(),
        description: 'Focus QuickSearch'
    },
    {
        key: 'i',
        ctrl: true,
        handler: () => openQuickAdd(),
        description: 'Insert new row (open quick add)'
    },
    {
        key: 'f',
        ctrl: true,
        handler: () => focusTableFilter(),
        description: 'Focus table filter/search'
    },
    {
        key: 's',
        ctrl: true,
        handler: () => saveCurrentEdit(),
        description: 'Save current edit'
    },
    {
        key: '/',
        ctrl: true,
        shift: true,
        handler: () => { showShortcutsHelp.value = true },
        description: 'Show keyboard shortcuts help'
    },
    {
        key: 'g',
        handler: () => handleGKey(),
        description: 'Navigation prefix'
    },
    {
        key: 'h',
        handler: (e: KeyboardEvent) => {
            // Don't intercept if user is typing in an input field, textarea, or contenteditable
            const target = e.target as HTMLElement
            if (target && (
                target.tagName === 'INPUT' ||
                target.tagName === 'TEXTAREA' ||
                target.isContentEditable ||
                (target.tagName === 'SELECT')
            )) {
                return
            }
            handleNavigationKey('h')
        },
        description: 'Go to Home (after g)'
    },
    {
        key: 't',
        handler: (e: KeyboardEvent) => {
            // Don't intercept if user is typing in an input field, textarea, or contenteditable
            const target = e.target as HTMLElement
            if (target && (
                target.tagName === 'INPUT' ||
                target.tagName === 'TEXTAREA' ||
                target.isContentEditable ||
                (target.tagName === 'SELECT')
            )) {
                return
            }
            handleNavigationKey('t')
        },
        description: 'Go to Tables (after g)'
    },
    {
        key: 'c',
        handler: (e: KeyboardEvent) => {
            // Don't intercept if user is typing in an input field, textarea, or contenteditable
            const target = e.target as HTMLElement
            if (target && (
                target.tagName === 'INPUT' ||
                target.tagName === 'TEXTAREA' ||
                target.isContentEditable ||
                (target.tagName === 'SELECT')
            )) {
                return
            }
            handleNavigationKey('c')
        },
        description: 'Go to Control Panel (after g)'
    },
    {
        key: 'b',
        handler: () => handleNavigationKey('b'),
        description: 'Toggle bookmarks toolbar'
    },
    {
        key: '?',
        handler: (e: KeyboardEvent) => {
            // Don't intercept if user is typing in an input field, textarea, or contenteditable
            const target = e.target as HTMLElement
            if (target && (
                target.tagName === 'INPUT' ||
                target.tagName === 'TEXTAREA' ||
                target.isContentEditable ||
                (target.tagName === 'SELECT')
            )) {
                return
            }
            handleNavigationKey('?')
        },
        description: 'Show keyboard shortcuts help (after g)'
    },
])

// Catch-all handler for any key pressed after 'g'
const handleAnyKeyAfterG = (e: KeyboardEvent) => {
    if (gKeyPressed.value) {
        // Don't intercept if user is typing in an input field, textarea, or contenteditable
        const target = e.target as HTMLElement
        if (target && (
            target.tagName === 'INPUT' ||
            target.tagName === 'TEXTAREA' ||
            target.isContentEditable ||
            (target.tagName === 'SELECT')
        )) {
            return
        }

        // Only handle if it's not already handled by a specific shortcut
        // Check if it's a letter, number, or special character
        const key = e.key.toLowerCase()
        if (key.length === 1 && !['h', 't', 'c', 'b', '?'].includes(key)) {
            handleNavigationKey(key)
        }
    }
}

// Add global keydown listener for catch-all
onMounted(() => {
    window.addEventListener('keydown', handleAnyKeyAfterG)
})

onUnmounted(() => {
    window.removeEventListener('keydown', handleAnyKeyAfterG)
})

useKeyboardShortcuts(shortcuts)

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
                            :title="bookmark.title || bookmark.navigationItem?.tableName"
                        >
                            <HugeiconsIcon
                                :icon="(bookmark.navigationItem?.icon && (Hugeicons as any)[bookmark.navigationItem.icon])
                                    ? (Hugeicons as any)[bookmark.navigationItem.icon]
                                    : DatabaseIcon"
                                class="bookmark-toolbar-icon"
                            />
                            <span class="bookmark-toolbar-text">
                                {{ bookmark.title || bookmark.navigationItem?.tableName }}
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
                @click="showShortcutsHelp = true"
                class="help-button"
                title="Keyboard Shortcuts (g then ?)"
            >
                <HugeiconsIcon :icon="Hugeicons.QuestionIcon" />
            </button>
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

    <!-- Keyboard Shortcuts Help Modal -->
    <KeyboardShortcutsHelp v-model:visible="showShortcutsHelp" />

    <!-- G Key Overlay -->
    <div v-if="showGKeyOverlay" class="g-key-overlay">
        <div class="g-key-content" :class="{ 'g-key-error': gKeyError }">
            <span class="g-key-prefix">g</span>
            <span v-if="gKeySecondKey" class="g-key-separator">+</span>
            <span v-if="gKeySecondKey" class="g-key-second" :class="{ 'g-key-second-error': gKeyError }">{{ gKeySecondKey }}</span>
            <span v-if="gKeyError" class="g-key-error-message">no shortcut matched</span>
            <span v-else-if="gKeyAction" class="g-key-action">{{ gKeyAction }}</span>
            <span v-else class="g-key-waiting">Waiting for second key...</span>
        </div>
    </div>
</template>

<style scoped>
.help-button {
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

.help-button:hover {
    background: rgba(255, 255, 255, 0.1);
    border-color: rgba(255, 255, 255, 0.2);
    color: white;
    transform: translateY(-1px);
}

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

/* G Key Overlay */
.g-key-overlay {
    position: fixed;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    z-index: 10000;
    pointer-events: none;
    animation: fadeIn 0.15s ease-out;
}

.g-key-content {
    background: rgba(0, 0, 0, 0.85);
    color: white;
    padding: 1rem 1.5rem;
    border-radius: 8px;
    font-size: 1.1rem;
    font-weight: 500;
    display: flex;
    align-items: center;
    gap: 0.5rem;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
    min-width: 200px;
    justify-content: center;
    transition: background-color 0.2s ease;
}

.g-key-content.g-key-error {
    background: rgba(220, 53, 69, 0.9);
}

.g-key-prefix {
    background: rgba(255, 255, 255, 0.2);
    padding: 0.25rem 0.5rem;
    border-radius: 4px;
    font-family: monospace;
    font-weight: 600;
}

.g-key-separator {
    color: rgba(255, 255, 255, 0.6);
    margin: 0 0.25rem;
}

.g-key-second {
    background: rgba(255, 255, 255, 0.2);
    padding: 0.25rem 0.5rem;
    border-radius: 4px;
    font-family: monospace;
    font-weight: 600;
    animation: popIn 0.2s ease-out;
}

.g-key-second-error {
    background: rgba(255, 255, 255, 0.3);
}

.g-key-action {
    margin-left: 0.75rem;
    color: rgba(255, 255, 255, 0.9);
    font-size: 0.95rem;
    animation: fadeIn 0.2s ease-out;
}

.g-key-waiting {
    margin-left: 0.75rem;
    color: rgba(255, 255, 255, 0.7);
    font-size: 0.9rem;
    font-style: italic;
}

.g-key-error-message {
    margin-left: 0.75rem;
    color: rgba(255, 255, 255, 0.95);
    font-size: 0.9rem;
    font-weight: 500;
    animation: fadeIn 0.2s ease-out;
}

@keyframes fadeIn {
    from {
        opacity: 0;
        transform: translate(-50%, -60%) scale(0.95);
    }
    to {
        opacity: 1;
        transform: translate(-50%, -50%) scale(1);
    }
}

@keyframes popIn {
    from {
        opacity: 0;
        transform: scale(0.8);
    }
    to {
        opacity: 1;
        transform: scale(1);
    }
}

/* Mobile responsive styles */
@media (max-width: 768px) {
    .toolbar-content {
        display: none;
    }

    .bookmark-button {
        display: none;
    }

    .g-key-content {
        font-size: 0.95rem;
        padding: 0.75rem 1.25rem;
    }
}
</style>
