<script setup lang="ts">
import Sidebar from 'picocrank/vue/components/Sidebar.vue'
import Breadcrumbs from 'picocrank/vue/components/Breadcrumbs.vue'
import { ref, onMounted } from 'vue'
import { Menu01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/vue'
import Table from './components/Table.vue'
import { createConnectTransport } from '@connectrpc/connect-web'
import { createClient } from '@connectrpc/connect'
import { SickRock } from './gen/sickrock_pb'

const sidebar = ref(null)

function toggleSidebar() {
    sidebar.value.toggle()
}

const transport = createConnectTransport({ baseUrl: '/api' })
const client = createClient(SickRock, transport)
const computersFields = ref<Array<{ name: string; type: string }>>([])
const contactsFields = ref<Array<{ name: string; type: string }>>([])
const pages = ref<Array<{ id: string; title: string; slug: string }>>([])

onMounted(async () => {
    const p = await client.getPages({})
    pages.value = p.pages.map(pg => ({ id: pg.id, title: pg.title, slug: pg.slug }))
    pages.value.forEach(pg => {
        sidebar.value && sidebar.value.addNavigationLink({ id: pg.id, title: pg.title, path: `/table/${pg.slug}`, icon: Menu01Icon })
    })
    sidebar.value && sidebar.value.addNavigationLink({ id: 'admin-create-table', title: 'Create Table', path: '/admin/table/create', icon: Menu01Icon })
    const computers = await client.getTableStructure({ pageId: 'computers' })
    computersFields.value = computers.fields.map(f => ({ name: f.name, type: f.type }))
    const contacts = await client.getTableStructure({ pageId: 'contacts' })
    contactsFields.value = contacts.fields.map(f => ({ name: f.name, type: f.type }))
})
</script>

<template>
    <header>
        <div id = "sidebar-button">
            <div class="logo-and-title">
                <img src="/src/resources/images/logo.png" alt="SickRock" class="logo" />
                <h1>SickRock</h1>
            </div>

            <button id="sidebar-toggler-button" aria-label="Toggle sidebar" @click="toggleSidebar" class="neutral">
                <HugeiconsIcon :icon="Menu01Icon" width="1em" height="1em" />
            </button>
        </div>

        <div class="fg1">
            <Breadcrumbs />
        </div>
    </header>
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

<style scoped>
button {
    color: inherit;
}
</style>
