<script setup lang="ts">
import Sidebar from 'picocrank/vue/components/Sidebar.vue'
import { ref, onMounted } from 'vue'
import { Menu01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/vue'
import Table from './components/Table.vue'
import { createConnectTransport } from '@connectrpc/connect-web'
import { createClient } from '@connectrpc/connect'
import { SickRock } from './gen/sickrock_pb'
import Header from 'picocrank/vue/components/Header.vue'
import logo from './resources/images/logo.png'

const sidebar = ref(null)

function toggleSidebar() {
    sidebar.value.toggle()
}

const transport = createConnectTransport({ baseUrl: '/api' })
const client = createClient(SickRock, transport)
const pages = ref<Array<{ id: string; title: string; slug: string }>>([])

onMounted(async () => {
    const p = await client.getPages({})
    pages.value = p.pages.map(pg => ({ id: pg.id, title: pg.title, slug: pg.slug }))
    pages.value.forEach(pg => {
        sidebar.value && sidebar.value.addNavigationLink({ id: pg.id, title: pg.title, path: `/table/${pg.slug}`, icon: Menu01Icon })
    })
    sidebar.value && sidebar.value.addNavigationLink({ id: 'table-configurations', title: 'Table Configurations', path: '/table/table_configurations', icon: Menu01Icon })
    sidebar.value && sidebar.value.addNavigationLink({ id: 'admin-create-table', title: 'Create Table', path: '/admin/table/create', icon: Menu01Icon })
})
</script>

<template>
	<Header title = "SickRock" :logoUrl = "logo" breadcrumbs @toggleSidebar = "toggleSidebar" />

    <div id="layout">

        <Sidebar ref="sidebar" />
        <div id="content">
            <main>
                <router-view :key="$route.path" />
            </main>
            <footer>
                <span>SickRock</span>
            </footer>
        </div>
    </div>

</template>
