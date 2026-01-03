<script setup lang="ts">
import Sidebar from 'picocrank/vue/components/Sidebar.vue'
import Navigation from 'picocrank/vue/components/Navigation.vue'
import { ref, onMounted, onUnmounted, computed, watch, provide, nextTick } from 'vue'
import { DatabaseIcon, DatabaseSettingIcon, PhoneArrowDownFreeIcons, LogoutIcon, HomeIcon, UserIcon, CheckmarkSquare03Icon, SearchIcon, BookmarkIcon, QuestionIcon, CheckListIcon, Delete01Icon, Download01Icon } from '@hugeicons/core-free-icons'
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
import PWAInstallPrompt from './components/PWAInstallPrompt.vue'
import { usePWAInstall } from './composables/usePWAInstall'
import { isOnline } from './utils/indexedDB'

const sidebar = ref(null)
const navigation = ref(null)
const isSidebarOpen = ref(true)
const SIDEBAR_STATE_KEY = 'sickrock_sidebar_open'
const PINNED_WORKFLOW_KEY = 'sickrock_pinned_workflow'
const router = useRouter()
const authStore = useAuthStore()

const isAuthenticated = computed(() => authStore.isAuthenticated)
const user = computed(() => authStore.user)
const isOffline = ref(false)

// Create global API client once and provide it to all components
const apiClient = createApiClient()
provide('apiClient', apiClient)

function persistSidebarState() {
    try { localStorage.setItem(SIDEBAR_STATE_KEY, isSidebarOpen.value ? '1' : '0') } catch {}
}

// Keep sidebar state in sync with navigation (the Sidebar auto-closes on navigation,
// so we mirror that in our local state to avoid the first toggle being a no-op).
router.afterEach(() => {
    isSidebarOpen.value = false
    if (sidebar.value) sidebar.value.close()
    persistSidebarState()
    // Close QuickSearch dialog when navigation occurs (e.g., item selected)
    showQuickSearchDialog.value = false
    // Check if current page can be bookmarked
    checkCanBookmarkCurrentPage()
})

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
const appTitle = ref<string>('SickRock') // Default to 'SickRock', will be loaded from settings

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
    workflowId: number;
  };
}>>([])

// Pinned workflow in header
const pinnedWorkflowId = ref<number | null>(null)
const pinnedWorkflowName = ref<string | null>(null)
const pinnedWorkflowItems = ref<Array<{ id: number; title: string; path: string; icon: any }>>([])

// Toggle state for bookmark dropdown
const showBookmarks = ref(false)

// QuickSearch dialog visibility
const showQuickSearchDialog = ref(false)

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

// PWA Install
const { isInstallable, isInstalled, promptInstall } = usePWAInstall()
const installingPWA = ref(false)

async function handlePWAInstall() {
  installingPWA.value = true
  try {
    await promptInstall()
  } catch (error) {
    console.error('Failed to install PWA:', error)
  } finally {
    installingPWA.value = false
  }
}

async function loadAppData() {
    // Only load data if authenticated
    if (!authStore.isAuthenticated) {
        if (sidebar.value) sidebar.value.close();
        isSidebarOpen.value = false
        persistSidebarState()
        return
    }

    // Check if we're offline
    const online = isOnline()
    isOffline.value = !online

    // If offline, show a message but don't fail completely
    if (!online) {
        console.warn('App is offline, skipping API calls')
        // Still try to set up basic UI structure
        if (navigation.value) {
            navigation.value.clearNavigationLinks()
            navigation.value.addNavigationLink({
                id: 'home',
                name: 'Home',
                title: 'Home',
                path: '/',
                icon: HomeIcon
            })
        }
        if (sidebar.value) {
            sidebar.value.stick()
            try {
                const stored = localStorage.getItem(SIDEBAR_STATE_KEY)
                isSidebarOpen.value = (stored == null ? true : stored === '1')
            } catch { isSidebarOpen.value = true }
            if (isSidebarOpen.value) sidebar.value.open()
            else sidebar.value.close()
        }
        return
    }

    try {
        // Get build info
        const initResponse = await apiClient.init(create(InitRequestSchema , {}))
        version.value = initResponse.version

        // Load appTitle setting from table_settings
        try {
            const settingsResponse = await apiClient.listItems({ tcName: 'table_settings', where: { setting_key: 'appTitle' } })
            if (settingsResponse.items && settingsResponse.items.length > 0) {
                const appTitleItem = settingsResponse.items[0]
                const stringVal = appTitleItem.additionalFields?.string_val
                if (stringVal) {
                    appTitle.value = stringVal
                }
            }
        } catch (e) {
            console.warn('Failed to load appTitle setting, using default:', e)
            // Keep default 'SickRock' if loading fails
        }

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
            dashboardName: bookmark.navigationItem.dashboardName,
            workflowId: bookmark.navigationItem.workflowId || 0
          } : undefined
        }))

        const sortedItems = [...(navResponse.items || [])].sort((a, b) => (a.ordinal ?? 0) - (b.ordinal ?? 0))
        pages.value = sortedItems
            .map(item => {
                const title = item.title || String(item.id)
                const slug = item.tableName || item.dashboardName || ''
                // Use CheckListIcon as default for workflow items, DatabaseIcon for table configurations
                const defaultIcon = (item.workflowId && item.workflowId > 0) ? 'CheckListIcon' : 'DatabaseIcon'
                const icon = item.icon || defaultIcon
                const view = item.tableView || ''
                const id = title
                const name = title
                const path = item.dashboardId > 0 ? `/dashboard/${item.dashboardName}` : `/table/${item.tableName}`
                return { id, name, title, slug, icon, view, path }
            })
            .filter(pg => !!pg.slug)

        // Build pinned workflow quick links, if any
        pinnedWorkflowId.value = null
        pinnedWorkflowName.value = null
        pinnedWorkflowItems.value = []
        let storedPinned: string | null = null
        try {
            storedPinned = localStorage.getItem(PINNED_WORKFLOW_KEY)
        } catch {
            storedPinned = null
        }
        if (storedPinned) {
            const workflows = (navResponse as any).workflows || []
            const wf = workflows.find((w: any) => (w.name || '') === storedPinned)
            if (wf && wf.items && wf.items.length) {
                pinnedWorkflowId.value = wf.id
                pinnedWorkflowName.value = wf.name || storedPinned
                pinnedWorkflowItems.value = (wf.items as any[]).map((item: any) => {
                    const title = item.title || item.tableTitle || item.tableName || String(item.id)
                    const path = item.dashboardId > 0
                        ? `/dashboard/${item.dashboardName}`
                        : `/table/${item.tableName}`
                    const iconName = item.icon || 'DatabaseIcon'
                    const icon = (Hugeicons as any)[iconName] || DatabaseIcon
                    return { id: item.id, title, path, icon }
                })
            }
        }

        if (navigation.value) {
            navigation.value.clearNavigationLinks()
        }
        if (quickSearch.value) {
            quickSearch.value.clearItems()
        }

        // Add home link
        if (navigation.value) {
            navigation.value.addNavigationLink({
                id: 'home',
                name: 'Home',
                title: 'Home',
                path: '/',
                icon: HomeIcon
            })
        }

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

        // Add workflows to sidebar and quick search (if present)
        const workflows = (navResponse as any).workflows || []
        const sortedWorkflows = [...workflows].sort((a: any, b: any) => (a.ordinal ?? 0) - (b.ordinal ?? 0))

        sortedWorkflows.forEach((workflow: any) => {
            const workflowIconName = workflow.icon || 'DatabaseIcon'
            const workflowIcon = (Hugeicons as any)[workflowIconName] || DatabaseIcon
            const workflowPath = `/workflow/${workflow.id}`

            navigation.value?.addNavigationLink({
                id: `workflow-${workflow.id}`,
                name: workflow.name || '',
                title: workflow.name || '',
                path: workflowPath,
                icon: workflowIcon
            })

            if (quickSearch.value) {
                quickSearch.value.addItem({
                    id: `workflow-${workflow.id}`,
                    name: workflow.name || '',
                    title: workflow.name || '',
                    description: `Workflow with ${workflow.items?.length || 0} items`,
                    category: 'Workflows',
                    path: workflowPath,
                    type: 'route',
                    icon: workflowIcon
                })
            }
        })

        pages.value.forEach(pg => {
            const icon = Hugeicons[pg.icon] || DatabaseIcon

            navigation.value?.addNavigationLink({
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
                path: pg.path,
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

        if (navigation.value) {
            navigation.value.addSeparator()

            // Table configurations
            navigation.value.addNavigationLink({
                id: 'table-configurations',
                name: 'Table Configurations',
                title: 'Table Configurations',
                path: '/table/table_configurations',
                icon: DatabaseSettingIcon
            })
            quickSearch.value?.addItem({
                id: 'table-configurations',
                name: 'Table Configurations',
                title: 'Table Configurations',
                description: 'Manage table configurations',
                category: 'Navigation',
                path: '/table/table_configurations',
                type: 'route',
                icon: DatabaseSettingIcon
            })

            // Workflows
            navigation.value.addNavigationLink({
                id: 'workflows',
                name: 'Workflows',
                title: 'Workflows',
                path: '/table/table_workflows',
                icon: DatabaseSettingIcon
            })
            quickSearch.value?.addItem({
                id: 'workflows',
                name: 'Workflows',
                title: 'Workflows',
                description: 'Manage workflows',
                category: 'Navigation',
                path: '/table/table_workflows',
                type: 'route',
                icon: DatabaseSettingIcon
            })

            // Navigation items - removed from sidebar, now in control panel
            quickSearch.value?.addItem({
                id: 'nav-items',
                name: 'Navigation',
                title: 'Navigation',
                description: 'Manage navigation items',
                category: 'Navigation',
                path: '/table/table_navigation',
                type: 'route',
                icon: DatabaseSettingIcon
            })

            // Admin routes: control-panel, device-code-claimer
            quickSearch.value?.addItem({
                id: 'table-create',
                name: 'Create Table',
                title: 'Create Table',
                description: 'Create a new table',
                category: 'Navigation',
                path: '/admin/table/create',
                type: 'route',
                icon: DatabaseSettingIcon
            })

            navigation.value.addRouterLink('control-panel')
            quickSearch.value?.addItem({
                id: 'control-panel',
                name: 'Control Panel',
                title: 'Control Panel',
                description: 'Administrative control panel',
                category: 'Navigation',
                path: '/admin/control-panel',
                type: 'route',
                icon: DatabaseSettingIcon
            })

            quickSearch.value?.addItem({
                id: 'device-code-claimer',
                name: 'Device Code Claimer',
                title: 'Device Code Claimer',
                description: 'Complete device code authentication',
                category: 'Navigation',
                path: '/device-code-claimer',
                type: 'route',
                icon: DatabaseSettingIcon
            })

            navigation.value.addCallback('Logout', async () => { await handleLogout() }, { icon: LogoutIcon })
        }
        if (sidebar.value) {
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

        // Check if it's a network error (offline)
        const isNetworkError = !isOnline() ||
            (error instanceof Error && (
                error.message.includes('Failed to fetch') ||
                error.message.includes('NetworkError') ||
                error.message.includes('network') ||
                error.name === 'NetworkError'
            ))

        if (isNetworkError) {
            isOffline.value = true
            console.warn('Network error detected, app is offline')
            // Still try to set up basic UI structure
            if (navigation.value) {
                navigation.value.clearNavigationLinks()
                navigation.value.addNavigationLink({
                    id: 'home',
                    name: 'Home',
                    title: 'Home',
                    path: '/',
                    icon: HomeIcon
                })
            }
            if (sidebar.value) {
                sidebar.value.stick()
                try {
                    const stored = localStorage.getItem(SIDEBAR_STATE_KEY)
                    isSidebarOpen.value = (stored == null ? true : stored === '1')
                } catch { isSidebarOpen.value = true }
                if (isSidebarOpen.value) sidebar.value.open()
                else sidebar.value.close()
            }
            return
        }

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
        if (navigation.value && typeof navigation.value.clearNavigationLinks === 'function') {
            navigation.value.clearNavigationLinks()
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

// Watch for bookmarks dialog opening to check if current page can be bookmarked
watch(showBookmarks, async (isOpen) => {
    if (isOpen) {
        await checkCanBookmarkCurrentPage()
    }
})

// Bookmark management
const currentRoute = computed(() => router.currentRoute.value)
const canBookmarkCurrentPage = ref(false)

const isCurrentPageBookmarked = computed(() => {
    if (!currentRoute.value.path) return false

    // Check if current route matches any bookmarked navigation item
    const currentPath = currentRoute.value.path
    return bookmarks.value.some(bookmark => {
        if (!bookmark.navigationItem) return false

        // Check table/dashboard paths
        const bookmarkPath = bookmark.navigationItem.dashboardId > 0
            ? `/dashboard/${bookmark.navigationItem.dashboardName}`
            : `/table/${bookmark.navigationItem.tableName}`

        if (bookmarkPath === currentPath) return true

        // Check workflow path - if navigation item has workflowId, check if it matches
        if (currentPath.startsWith('/workflow/') && bookmark.navigationItem.workflowId && bookmark.navigationItem.workflowId > 0) {
            const workflowIdMatch = currentPath.match(/^\/workflow\/(\d+)$/)
            if (workflowIdMatch) {
                const workflowId = Number(workflowIdMatch[1])
                return bookmark.navigationItem.workflowId === workflowId
            }
        }

        return false
    })
})

// Check if current page can be bookmarked (has a matching navigation item)
async function checkCanBookmarkCurrentPage() {
    if (!currentRoute.value.path) {
        canBookmarkCurrentPage.value = false
        return
    }

    const currentPath = currentRoute.value.path

    try {
        const navResponse = await apiClient.getNavigation({})

        // Check if path matches a navigation item (table/dashboard)
        let matchingItem = navResponse.items?.find(item => {
            const itemPath = item.dashboardId > 0
                ? `/dashboard/${item.dashboardName}`
                : `/table/${item.tableName}`
            return itemPath === currentPath
        })

        // If no direct match, check if it's a workflow route
        if (!matchingItem && currentPath.startsWith('/workflow/')) {
            const workflowIdMatch = currentPath.match(/^\/workflow\/(\d+)$/)
            if (workflowIdMatch) {
                const workflowId = Number(workflowIdMatch[1])
                // Find navigation item that references this workflow
                matchingItem = navResponse.items?.find(item => item.workflowId === workflowId)
            }
        }

        canBookmarkCurrentPage.value = !!matchingItem
        console.log('[Bookmarks] Can bookmark current page:', canBookmarkCurrentPage.value, 'for path:', currentPath, 'matching item:', matchingItem)
    } catch (e) {
        console.warn('Failed to check if current page can be bookmarked:', e)
        canBookmarkCurrentPage.value = false
    }
}

async function toggleCurrentPageBookmark() {
    if (!currentRoute.value.path) return

    const currentPath = currentRoute.value.path

    // Find matching navigation item
    const navResponse = await apiClient.getNavigation({})

    // Check if path matches a navigation item (table/dashboard)
    let matchingItem = navResponse.items?.find(item => {
        const itemPath = item.dashboardId > 0
            ? `/dashboard/${item.dashboardName}`
            : `/table/${item.tableName}`
        return itemPath === currentPath
    })

    // If no direct match, check if it's a workflow route
    if (!matchingItem && currentPath.startsWith('/workflow/')) {
        const workflowIdMatch = currentPath.match(/^\/workflow\/(\d+)$/)
        if (workflowIdMatch) {
            const workflowId = Number(workflowIdMatch[1])
            // Find navigation item that references this workflow
            matchingItem = navResponse.items?.find(item => item.workflowId === workflowId)
        }
    }

    if (!matchingItem) {
        console.warn('No matching navigation item found for current page')
        return
    }

    try {
        if (isCurrentPageBookmarked.value) {
            // Remove bookmark
            const bookmark = bookmarks.value.find(b => b.navigationItemId === matchingItem.id)
            if (bookmark) {
                await removeBookmark(bookmark.id)
            }
        } else {
            // Add bookmark
            await apiClient.createUserBookmark({ navigationItemId: matchingItem.id })
            // Reload bookmarks
            await loadAppData()
            // Update canBookmarkCurrentPage state
            canBookmarkCurrentPage.value = true
        }
    } catch (error) {
        console.error('Failed to toggle bookmark:', error)
    }
}

async function removeBookmark(bookmarkId: number) {
    try {
        await apiClient.deleteUserBookmark({ bookmarkId })
        // Reload bookmarks
        await loadAppData()
    } catch (error) {
        console.error('Failed to remove bookmark:', error)
    }
}

function toggleToolbarView() {
    // Use the toolbar button purely as a search trigger
    focusQuickSearch()
}

// Keyboard shortcuts handlers
function focusQuickSearch() {
    // Ensure the QuickSearch dialog is visible, then focus its input
    showQuickSearchDialog.value = true
    nextTick(() => {
        const searchInput = document.querySelector('.quick-search input, [class*="quick-search"] input') as HTMLInputElement
        if (searchInput) {
            searchInput.focus()
        } else if (quickSearch.value && typeof (quickSearch.value as any).focus === 'function') {
            (quickSearch.value as any).focus()
        }
    })
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

// Global listeners
onMounted(() => {
    window.addEventListener('keydown', handleAnyKeyAfterG)
    // Refresh header when pinned workflow changes (e.g., from HomeView)
    window.addEventListener('pinned-workflow-changed', () => {
        // Re-fetch navigation to rebuild pinnedWorkflow state
        loadAppData()
    })

    // Listen for online/offline events
    const handleOnline = () => {
        isOffline.value = false
        if (authStore.isAuthenticated) {
            loadAppData()
        }
    }
    const handleOffline = () => {
        isOffline.value = true
    }

    window.addEventListener('online', handleOnline)
    window.addEventListener('offline', handleOffline)

    // Check initial online status
    isOffline.value = !isOnline()

    onUnmounted(() => {
        window.removeEventListener('online', handleOnline)
        window.removeEventListener('offline', handleOffline)
    })
})

onUnmounted(() => {
    window.removeEventListener('keydown', handleAnyKeyAfterG)
    window.removeEventListener('pinned-workflow-changed', () => {
        loadAppData()
    })
})

useKeyboardShortcuts(shortcuts)

onMounted(async () => {
    await loadAppData()
    await checkCanBookmarkCurrentPage()
})
</script>

<template>
    <Header
        :title = "appTitle"
        :logoUrl = "logo"
        :username = "user?.username"
        @toggleSidebar = "toggleSidebar"
        v-if="isAuthenticated">

        <template #toolbar>
            <div class="toolbar-content">
                <!-- Search Button -->
                <button
                    @click="toggleToolbarView"
                    class="toolbar-toggle-button"
                    title="Search"
                >
                    <HugeiconsIcon :icon="Hugeicons.SearchIcon" />
                </button>

                <!-- Pinned workflow quick links, styled like bookmarks -->
                <div v-if="pinnedWorkflowItems.length" class="pinned-workflow-toolbar">
                    <router-link
                        v-if="pinnedWorkflowId != null"
                        :to="`/workflow/${pinnedWorkflowId}`"
                        class="pinned-workflow-label"
                        :title="pinnedWorkflowName || 'Workflow'"
                    >
                        <span class="pinned-workflow-label-text">{{ pinnedWorkflowName }}</span>
                    </router-link>
                    <span v-else class="pinned-workflow-label">
                        <span class="pinned-workflow-label-text">{{ pinnedWorkflowName }}</span>
                    </span>
                    <div class="bookmarks-list-toolbar">
                        <router-link
                            v-for="item in pinnedWorkflowItems"
                            :key="item.id"
                            :to="item.path"
                            class="bookmark-toolbar-item"
                            :title="item.title"
                        >
                            <HugeiconsIcon
                                :icon="item.icon"
                                class="bookmark-toolbar-icon"
                            />
                            <span class="bookmark-toolbar-text">
                                {{ item.title }}
                            </span>
                        </router-link>
                    </div>
                </div>

            </div>
        </template>

        <template #user-info>
            <button
                v-if="isInstallable && !isInstalled"
                @click="handlePWAInstall"
                :disabled="installingPWA"
                class="pwa-install-header-button"
                :title="installingPWA ? 'Installing...' : 'Install App'"
            >
                <HugeiconsIcon :icon="Hugeicons.Download01Icon" />
            </button>
            <button
                @click="showShortcutsHelp = true"
                class="help-button"
                title="Keyboard Shortcuts (g then ?)"
            >
                <HugeiconsIcon :icon="Hugeicons.QuestionIcon" />
            </button>
            <button
                @click="showBookmarks = !showBookmarks"
                class="bookmark-button"
                :class="{ 'bookmarked': isCurrentPageBookmarked }"
                :title="isCurrentPageBookmarked ? 'Remove bookmark' : 'Add bookmark'"
            >
                <HugeiconsIcon :icon="Hugeicons.CheckmarkSquare03Icon" />
            </button>
            <router-link
                to="/user-control-panel"
                class="user-preferences-button"
                title="User Control Panel"
            >
                <HugeiconsIcon :icon="Hugeicons.UserIcon" />
                <span class="username-text">{{ user?.username }}</span>
            </router-link>
        </template>
    </Header>

    <Navigation v-if="isAuthenticated" ref="navigation">
        <div id="layout">
            <Sidebar ref="sidebar" />
            <div id="content">
            <!-- Offline Banner -->
            <div v-if="isOffline && isAuthenticated" class="offline-banner">
                <span>📡 You're offline. Some features may be unavailable.</span>
            </div>
            <main>
                <router-view :key="$route.path" />
            </main>
            <footer v-if="version">
                <span>
                    <a
                        href="https://github.com/jamesread/SickRock"
                        target="_blank"
                        rel="noopener noreferrer"
                        class="github-link"
                        title="View on GitHub"
                    >
                        SickRock
                    </a>
                </span>
                <span>{{ version }}</span>
            </footer>
        </div>
    </div>
    </Navigation>

    <!-- QuickSearch Dialog -->
    <div
        v-if="showQuickSearchDialog"
        class="modal-overlay"
        @click="showQuickSearchDialog = false"
        @keydown.escape="showQuickSearchDialog = false"
        tabindex="0"
    >
        <div class="modal-content quicksearch-modal" @click.stop>
            <div class="modal-body">
                <QuickSearch
                    ref="quickSearch"
                    :search-fields="['title']"
                />
            </div>
        </div>
    </div>

    <!-- Bookmarks Dialog -->
    <div
        v-if="showBookmarks"
        class="modal-overlay"
        @click="showBookmarks = false"
        @keydown.escape="showBookmarks = false"
        tabindex="0"
    >
        <div class="modal-content bookmarks-modal" @click.stop>
            <div class="modal-header">
                <div class="modal-header-left">
                    <h3>Bookmarks</h3>
                </div>
                <button @click="showBookmarks = false" class="button neutral" title="Close">
                    ✕
                </button>
            </div>
            <div class="modal-body">
                <div class="bookmarks-toolbar">
                    <!-- Bookmark this page button -->
                    <div v-if="!isCurrentPageBookmarked && canBookmarkCurrentPage" class="bookmark-this-page-section">
                        <button
                            @click="toggleCurrentPageBookmark"
                            class="button primary bookmark-this-page-button"
                        >
                            <HugeiconsIcon :icon="Hugeicons.BookmarkIcon" />
                            Bookmark this page
                        </button>
                    </div>
                    <div v-if="bookmarks.length === 0" class="no-bookmarks-toolbar">
                        <p v-if="isCurrentPageBookmarked">No other bookmarks</p>
                        <p v-else>No bookmarks yet</p>
                    </div>
                    <div v-else class="bookmarks-list-toolbar">
                        <div
                            v-for="bookmark in bookmarks.slice(0, 10)"
                            :key="bookmark.id"
                            class="bookmark-item-wrapper"
                        >
                            <router-link
                                :to="bookmark.navigationItem?.dashboardId > 0
                                    ? `/dashboard/${bookmark.navigationItem.dashboardName}`
                                    : bookmark.navigationItem?.workflowId && bookmark.navigationItem.workflowId > 0
                                    ? `/workflow/${bookmark.navigationItem.workflowId}`
                                    : `/table/${bookmark.navigationItem?.tableName}`"
                                class="bookmark-toolbar-item"
                                :title="bookmark.title || bookmark.navigationItem?.tableName"
                                @click="showBookmarks = false"
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
                            <button
                                @click.stop="removeBookmark(bookmark.id)"
                                class="bookmark-delete-button"
                                :title="'Remove bookmark'"
                            >
                                <HugeiconsIcon :icon="Hugeicons.Delete01Icon" />
                            </button>
                        </div>
                    </div>
                </div>
            </div>
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

    <!-- PWA Install Prompt -->
    <PWAInstallPrompt v-if="isAuthenticated" />
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

.pwa-install-header-button {
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

.pwa-install-header-button:hover:not(:disabled) {
    background: rgba(255, 255, 255, 0.1);
    border-color: rgba(255, 255, 255, 0.2);
    color: white;
    transform: translateY(-1px);
}

.pwa-install-header-button:disabled {
    opacity: 0.5;
    cursor: not-allowed;
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

.user-preferences-button .username-text {
    display: inline;
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
    position: relative;
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
}

.bookmark-toolbar-icon {
    width: 14px;
    height: 14px;
}

.bookmark-toolbar-text {
    max-width: 120px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

/* Pinned workflow label inline style */
.pinned-workflow-toolbar {
    display: flex;
    align-items: center;
    gap: 0.5rem;
}

.pinned-workflow-label {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    padding: 0.2rem 0.5rem;
    border-radius: 999px;
    background: rgba(15, 118, 110, 0.75);
    color: white;
    text-decoration: none;
    border: 1px solid rgba(15, 118, 110, 0.9);
    font-size: 0.8rem;
}

.pinned-workflow-label-text {
    font-weight: 500;
}

.pinned-workflow-label-badge {
    font-size: 0.7rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    padding: 0.1rem 0.4rem;
    border-radius: 999px;
    background: rgba(15, 118, 110, 0.9);
    color: white;
}

.modal-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.5);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 3000;
    padding: 20px;
    box-sizing: border-box;
}

.modal-content {
    background: #ffffff;
    border-radius: 8px;
    box-shadow: 0 4px 20px rgba(0, 0, 0, 0.3);
    max-width: 600px;
    width: 90%;
    display: flex;
    flex-direction: column;
}

.modal-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 16px 20px;
    border-bottom: 1px solid rgba(148, 163, 184, 0.4);
}

.modal-header-left h3 {
    margin: 0;
    font-size: 1rem;
    font-weight: 600;
    color: #212529;
}

.modal-body {
    padding: 16px 20px 20px;
}

.bookmarks-modal {
    max-width: 520px;
}

.bookmarks-modal .modal-body {
    padding-top: 0;
}

.quicksearch-modal {
    max-width: 600px;
}

.quicksearch-modal .modal-body {
    padding: 16px 20px 20px;
}

.quicksearch-modal :deep(.quick-search),
.quicksearch-modal :deep([class*="quick-search"]) {
    width: 100%;
}

.quicksearch-modal :deep(input) {
    width: 100%;
    box-sizing: border-box;
}

.bookmarks-modal .bookmarks-toolbar {
    flex-direction: column;
    align-items: stretch;
    gap: 0.75rem;
}

.bookmarks-modal .bookmarks-list-toolbar {
    flex-direction: column;
    max-height: 320px;
    overflow-y: auto;
}

.bookmarks-modal .bookmark-toolbar-item {
    width: 100%;
    background: transparent;
    border: none;
    color: #111827;
    padding: 8px 10px;
    border-radius: 6px;
    transition: background-color 0.15s ease;
}

.bookmarks-modal .bookmark-toolbar-item:hover {
    background-color: #f3f4f6;
    color: #111827;
}

.bookmarks-modal .bookmark-toolbar-icon {
    color: #4b5563;
}

.bookmarks-modal .bookmark-toolbar-text {
    max-width: none;
    white-space: nowrap;
    text-overflow: ellipsis;
    overflow: hidden;
}

.bookmark-item-wrapper {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
}

.bookmark-item-wrapper .bookmark-toolbar-item {
    flex: 1;
}

.bookmark-delete-button {
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 4px 8px;
    background: transparent;
    border: none;
    color: #6c757d;
    cursor: pointer;
    border-radius: 4px;
    transition: all 0.15s ease;
    flex-shrink: 0;
}

.bookmark-delete-button:hover {
    background: #f3f4f6;
    color: #dc2626;
}

.bookmark-delete-button :deep(svg) {
    width: 16px;
    height: 16px;
}

.bookmarks-modal .no-bookmarks-toolbar {
    color: #6c757d;
    font-style: italic;
    padding: 8px 0;
}

.bookmark-this-page-section {
    margin-bottom: 16px;
    padding-bottom: 16px;
    border-bottom: 1px solid rgba(148, 163, 184, 0.3);
}

.bookmark-this-page-button {
    width: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
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
    .pinned-workflow-toolbar {
        display: none;
    }

    .help-button {
        display: none;
    }

    .user-preferences-button .username-text {
        display: none;
    }

    .user-preferences-button {
        padding: 0.5rem;
    }

    .g-key-content {
        font-size: 0.95rem;
        padding: 0.75rem 1.25rem;
    }
}

/* Constrain main content area to prevent page scrolling */
#layout {
    display: flex;
	flex-direction: column;
}

#content {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
}

#content main {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    overflow-x: hidden;
}

footer span {
    color: #666;
}

.github-link {
    color: #007bff;
    text-decoration: none;
    transition: color 0.2s ease;
}

.github-link:hover {
    color: #0056b3;
    text-decoration: underline;
}

.github-link:visited {
    color: #007bff;
}

@media (max-width: 768px) {
    footer {
        flex-direction: column;
        align-items: flex-start;
        gap: 0.5rem;
        font-size: 0.8rem;
    }
}

/* Offline Banner */
.offline-banner {
    background: #ffc107;
    color: #000;
    padding: 0.75rem 1rem;
    text-align: center;
    font-size: 0.875rem;
    font-weight: 500;
    border-bottom: 1px solid #ffb300;
    z-index: 100;
}

.offline-banner span {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
}
</style>
