import { createRouter, createWebHistory } from 'vue-router'
import HomeView from './views/HomeView.vue'
import AboutView from './views/AboutView.vue'
import TableView from './views/TableView.vue'
import RowView from './views/RowView.vue'
import RowEditView from './views/RowEditView.vue'
import AddColumnView from './views/AddColumnView.vue'
import InsertRowView from './views/InsertRowView.vue'
import AfterInsertView from './views/AfterInsertView.vue'
import CreateTableView from './views/CreateTableView.vue'
import ForeignKeyManagement from './views/ForeignKeyManagement.vue'
import ColumnTypeManagement from './views/ColumnTypeManagement.vue'
import TableCreate from './views/TableCreate.vue'
import ControlPanel from './views/ControlPanel.vue'
import NotFoundView from './views/NotFoundView.vue'
import { DatabaseAddIcon } from '@hugeicons/core-free-icons'

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
    { path: '/table/:tableName/:rowId/edit', name: 'row-edit', component: RowEditView, props: true,
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
    {
      path: '/table/:tableName/after-insert',
      name: 'after-insert',
      component: AfterInsertView,
      props: true,
      meta: {
        breadcrumbs: (route: any) => [
          { name: String(route.params.tableName), href: { name: 'table', params: { tableName: route.params.tableName } } },
          { name: 'Row Added' },
        ],
      },
    },
    {
      path: '/table/:tableName/create-view',
      name: 'create-table-view',
      component: CreateTableView,
      props: true,
      meta: {
        breadcrumbs: (route: any) => [
          { name: String(route.params.tableName), href: { name: 'table', params: { tableName: route.params.tableName } } },
          { name: 'Create View' },
        ],
      },
    },
    {
      path: '/table/:tableName/edit-view/:viewId',
      name: 'edit-table-view',
      component: CreateTableView,
      props: true,
      meta: {
        breadcrumbs: (route: any) => [
          { name: String(route.params.tableName), href: { name: 'table', params: { tableName: route.params.tableName } } },
          { name: 'Edit View' },
        ],
      },
    },
        {
          path: '/table/:tableName/foreign-keys',
          name: 'foreign-keys',
          component: ForeignKeyManagement,
          props: true,
          meta: {
            breadcrumbs: (route: any) => [
              { name: String(route.params.tableName), href: { name: 'table', params: { tableName: route.params.tableName } } },
              { name: 'Foreign Keys' },
            ],
          },
        },
        {
          path: '/table/:tableName/column-types',
          name: 'column-types',
          component: ColumnTypeManagement,
          props: true,
          meta: {
            breadcrumbs: (route: any) => [
              { name: String(route.params.tableName), href: { name: 'table', params: { tableName: route.params.tableName } } },
              { name: 'Column Types' },
            ],
          },
        },
    {
      path: '/admin/table/create',
      name: 'table-create',
      component: TableCreate,
      meta: {
        title: 'Create Table',
        icon: DatabaseAddIcon
      },
    },
    {
      path: '/admin/control-panel',
      name: 'control-panel',
      component: ControlPanel,
      meta: {
        title: 'Control Panel',
        icon: DatabaseAddIcon
      },
    },
    {
      path: '/:pathMatch(.*)*',
      name: 'not-found',
      component: NotFoundView,
    },
  ],
})

export default router
