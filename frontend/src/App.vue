<script setup lang="ts">
import Sidebar from 'picocrank/vue/components/Sidebar.vue'
import { ref, onMounted } from 'vue'
import { DatabaseIcon, DatabaseSettingIcon, PhoneArrowDownFreeIcons } from '@hugeicons/core-free-icons'
import { createConnectTransport } from '@connectrpc/connect-web'
import { createClient } from '@connectrpc/connect'
import { SickRock } from './gen/sickrock_pb'
import Header from 'picocrank/vue/components/Header.vue'
import logo from './resources/images/logo.png'
import QuickSearch from 'picocrank/vue/components/QuickSearch.vue'
import * as Hugeicons from '@hugeicons/core-free-icons'

const sidebar = ref(null)

function toggleSidebar() {
    sidebar.value.toggle()
}

const transport = createConnectTransport({ baseUrl: '/api' })
const client = createClient(SickRock, transport)
const pages = ref<Array<{ id: string; title: string; slug: string }>>([])
const version = ref<string>('')
const quickSearch = ref(null)

onMounted(async () => {
    // Get build info
    try {
        const initResponse = await client.init({})
        version.value = initResponse.version
    } catch (error) {
        console.error('Failed to get build info:', error)
        version.value = 'unknown'
    }

    const p = await client.getPages({})
    pages.value = p.pages.map(pg => ({ id: pg.id, name: pg.id, title: pg.title, slug: pg.slug, icon: pg.icon }))
    pages.value.forEach(pg => {
        const icon = Hugeicons[pg.icon] || DatabaseIcon

        sidebar.value.addNavigationLink({
            id: pg.id,
            name: pg.id,
            title: pg.title,
            path: `/table/${pg.slug}`,
            icon: icon
        })

        quickSearch.value.addItem({
            id: pg.id,
            name: pg.title,
            title: pg.title,
            description: 'Table: ' + pg.title,
            category: 'Navigation',
            path: `/table/${pg.slug}`,
            type: 'route',
            icon: icon
        })
    })
    sidebar.value.addSeparator()
    sidebar.value.addNavigationLink({
        id: 'table-configurations',
        name: 'Table Configurations', title: 'Table Configurations', path: '/table/table_configurations', icon: DatabaseSettingIcon })
    sidebar.value.addRouterLink('table-create')
    sidebar.value.addRouterLink('control-panel')
})
</script>

<template>
	<Header title = "SickRock" :logoUrl = "logo" @toggleSidebar = "toggleSidebar">
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

</template>
