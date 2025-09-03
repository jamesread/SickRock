import { createRouter, createWebHistory } from 'vue-router'
import HomeView from './views/HomeView.vue'
import AboutView from './views/AboutView.vue'
import TableView from './views/TableView.vue'
import RowView from './views/RowView.vue'
import RowEdit from './views/RowEdit.vue'
import AddColumnView from './views/AddColumnView.vue'
import InsertRowView from './views/InsertRowView.vue'
import TableCreate from './views/TableCreate.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'home', component: HomeView },
    { path: '/about', name: 'about', component: AboutView },
    { path: '/table/:tableName', name: 'table', component: TableView, props: true,
      meta: {
        breadcrumbs: (route: any) => [
          { name: 'Table: ' + String(route.params.tableName), to: { name: 'table', params: { tableName: route.params.tableName } } },
        ],
      },
    },
    { path: '/table/:tableName/:rowId', name: 'row', component: RowView, props: true,
      meta: {
        breadcrumbs: (route: any) => [
          { name: String(route.params.tableName), href: { name: 'table', params: { tableName: route.params.tableName } } },
          { name: `Row ${String(route.params.rowId)}` },
        ],
      },
    },
    { path: '/table/:tableName/:rowId/edit', name: 'row-edit', component: RowEdit, props: true,
      meta: {
        breadcrumbs: (route: any) => [
          { name: String(route.params.tableName), href: { name: 'table', params: { tableName: route.params.tableName } } },
          { name: `Row ${String(route.params.rowId)}`, href: { name: 'row', params: { tableName: route.params.tableName, rowId: route.params.rowId } } },
          { name: 'Edit' },
        ],
      },
    },
    { path: '/table/:tableName/add-column', name: 'add-column', component: AddColumnView, props: true,
      meta: {
        breadcrumbs: (route: any) => [
          { name: String(route.params.tableName), href: { name: 'table', params: { tableName: route.params.tableName } } },
          { name: 'Add Column' },
        ],
      },
    },
    { 
      path: '/table/:tableName/insert-row', 
      name: 'insert-row', 
      component: InsertRowView, 
      props: true,
      meta: {
        breadcrumbs: (route: any) => [
          { name: String(route.params.tableName), href: { name: 'table', params: { tableName: route.params.tableName } } },
          { name: 'Insert Row' },
        ],
      },
    },
    { path: '/admin/table/create', name: 'table-create', component: TableCreate },
  ],
})

export default router


