<script setup lang="ts">
import Sidebar from 'picocrank/vue/components/Sidebar.vue'
import { ref, onMounted, computed, watch } from 'vue'
import { DatabaseIcon, DatabaseSettingIcon, PhoneArrowDownFreeIcons, LogoutIcon } from '@hugeicons/core-free-icons'
import { createApiClient } from './stores/api'
import Header from 'picocrank/vue/components/Header.vue'
import logo from './resources/images/logo.png'
import QuickSearch from 'picocrank/vue/components/QuickSearch.vue'
import * as Hugeicons from '@hugeicons/core-free-icons'
import { useAuthStore } from './stores/auth'
import { useRouter } from 'vue-router'

const sidebar = ref(null)
const router = useRouter()
const authStore = useAuthStore()

const isAuthenticated = computed(() => authStore.isAuthenticated)
const user = computed(() => authStore.user)

function toggleSidebar() {
    sidebar.value.toggle()
}

async function handleLogout() {
    await authStore.logout()
    router.push('/login')
}

const pages = ref<Array<{ id: string; title: string; slug: string; view: string; icon: string }>>([])
const version = ref<string>('')
const quickSearch = ref(null)

async function loadAppData() {
    // Only load data if authenticated
    if (!authStore.isAuthenticated) {
        return
    }

    try {
        const client = createApiClient()
        
        // Get build info
        const initResponse = await client.init({})
        version.value = initResponse.version

        const p = await client.getPages({})
        pages.value = p.pages.map(pg => ({ id: pg.id, name: pg.id, title: pg.title, slug: pg.slug, icon: pg.icon, view: pg.view }))
        
        sidebar.value.clearNavigationLinks()
        quickSearch.value.clearItems()
    
        pages.value.forEach(pg => {
            const icon = Hugeicons[pg.icon] || DatabaseIcon
            const path = `/table/${pg.slug}`

            sidebar.value?.addNavigationLink({
                id: pg.id,
                name: pg.id,
                title: pg.title,
                path: path,
                icon: icon
            })

            quickSearch.value?.addItem({
                id: pg.id,
                name: pg.title,
                title: pg.title,
                description: 'Table: ' + pg.title,
                category: 'Navigation',
                path: path,
                type: 'route',
                icon: icon
            })
        })
        
        if (sidebar.value) {
            sidebar.value.addSeparator()
            sidebar.value.addNavigationLink({
                id: 'table-configurations',
                name: 'Table Configurations', title: 'Table Configurations', path: '/table/table_configurations', icon: DatabaseSettingIcon })
            sidebar.value.addRouterLink('table-create')
            sidebar.value.addRouterLink('control-panel')
            sidebar.value.stick()
            sidebar.value.open()
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
    <div v-if="!isAuthenticated" class="login-container">
        <router-view />
    </div>
    
    <div v-else>
        <Header title = "SickRock" :logoUrl = "logo" @toggleSidebar = "toggleSidebar" :username = "user?.username">
            <template #toolbar>
                <QuickSearch
                    ref="quickSearch"
                    :search-fields="['title']"
                />
            </template>
        </Header>

        <div id="layout">
            <Sidebar ref="sidebar" />
            <div id="content">
                <main>
                    <router-view :key="$route.path" />
                </main>
                <footer>
                    <span>SickRock</span>
                    <span>{{ version }}</span>
                </footer>
            </div>
        </div>
    </div>
</template>

<style scoped>
.login-container {
    min-height: 100vh;
}

.user-info {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-right: 16px;
}

.username {
    font-weight: 500;
    color: #333;
}

.logout-button {
    background: none;
    border: none;
    cursor: pointer;
    padding: 8px;
    border-radius: 4px;
    display: flex;
    align-items: center;
    justify-content: center;
    color: #666;
    transition: background-color 0.2s ease;
}

.logout-button:hover {
    background-color: #f0f0f0;
    color: #333;
}

.logout-button svg {
    width: 16px;
    height: 16px;
}
</style>
