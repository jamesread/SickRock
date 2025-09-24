<script setup lang="ts">
import Sidebar from 'picocrank/vue/components/Sidebar.vue'
import { ref, onMounted, computed, watch, provide } from 'vue'
import { DatabaseIcon, DatabaseSettingIcon, PhoneArrowDownFreeIcons, LogoutIcon, HomeIcon } from '@hugeicons/core-free-icons'
import { createApiClient } from './stores/api'
import Header from 'picocrank/vue/components/Header.vue'
import logo from './resources/images/logo.png'
import QuickSearch from 'picocrank/vue/components/QuickSearch.vue'
import * as Hugeicons from '@hugeicons/core-free-icons'
import { useAuthStore } from './stores/auth'
import { useRouter } from 'vue-router'
import { create } from '@bufbuild/protobuf'
import { InitRequestSchema, ItemSchema } from './gen/sickrock_pb'

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
        const sortedItems = [...(navResponse.items || [])].sort((a, b) => (a.ordinal ?? 0) - (b.ordinal ?? 0))
        pages.value = sortedItems
            .map(item => {
                const title = item.tableTitle || item.tableName || item.dashboardName || String(item.id)
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
        quickSearch.value.clearItems()

        // Add home link
        sidebar.value.addNavigationLink({
            id: 'home',
            name: 'Home',
            title: 'Home',
            path: '/',
            icon: HomeIcon
        })

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
            <QuickSearch
                ref="quickSearch"
                :search-fields="['title']"
            />
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
