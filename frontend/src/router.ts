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
import ExportView from './views/ExportView.vue'
import TableCreate from './views/TableCreate.vue'
import ControlPanel from './views/ControlPanel.vue'
import LoginView from './views/LoginView.vue'
import NotFoundView from './views/NotFoundView.vue'
import DeviceCodeClaimerView from './views/DeviceCodeClaimerView.vue'
import DatabaseBrowser from './views/DatabaseBrowser.vue'
import { DatabaseAddIcon } from '@hugeicons/core-free-icons'
import DashboardListView from './views/DashboardListView.vue'
import Dashboard from './views/Dashboard.vue'
import UserPreferences from './views/UserPreferences.vue'
import { useAuthStore } from './stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: LoginView,
      meta: { requiresAuth: false }
    },
    {
      path: '/table/:tableName/export',
      name: 'export',
      component: ExportView,
      props: true,
      meta: {
        requiresAuth: true,
        breadcrumbs: (route: any) => [
          { name: String(route.params.tableName), href: { name: 'table', params: { tableName: route.params.tableName } } },
          { name: 'Export CSV' },
        ],
      },
    },
    {
      path: '/',
      name: 'home',
      component: HomeView,
      meta: { requiresAuth: true }
    },
    {
      path: '/dashboards',
      name: 'dashboards',
      component: DashboardListView,
      meta: { requiresAuth: true }
    },
    {
      path: '/dashboard/:dashboardName',
      name: 'dashboard',
      component: Dashboard,
      props: true,
      meta: {
        requiresAuth: true,
        breadcrumbs: (route: any) => [
          { name: 'Dashboards', href: { name: 'dashboards' } },
          { name: String(route.params.dashboardName) },
        ],
      },
    },
    {
      path: '/about',
      name: 'about',
      component: AboutView,
      meta: { requiresAuth: true }
    },
    {
      path: '/user-preferences',
      name: 'user-preferences',
      component: UserPreferences,
      meta: { requiresAuth: true }
    },
    {
      path: '/table/:tableName',
      name: 'table',
      component: TableView,
      props: true,
      meta: {
        requiresAuth: true,
        breadcrumbs: (route: any) => [
          { name: 'Table: ' + String(route.params.tableName), to: { name: 'table', params: { tableName: route.params.tableName } } },
        ],
      },
    },
    {
      path: '/table/:tableName/:rowId',
      name: 'row',
      component: RowView,
      props: true,
      meta: {
        requiresAuth: true,
        breadcrumbs: (route: any) => [
          { name: String(route.params.tableName), href: { name: 'table', params: { tableName: route.params.tableName } } },
          { name: `Row ${String(route.params.rowId)}` },
        ],
      },
    },
    {
      path: '/table/:tableName/:rowId/edit',
      name: 'row-edit',
      component: RowEditView,
      props: true,
      meta: {
        requiresAuth: true,
        breadcrumbs: (route: any) => [
          { name: String(route.params.tableName), href: { name: 'table', params: { tableName: route.params.tableName } } },
          { name: `Row ${String(route.params.rowId)}`, href: { name: 'row', params: { tableName: route.params.tableName, rowId: route.params.rowId } } },
          { name: 'Edit' },
        ],
      },
    },
    {
      path: '/table/:tableName/add-column',
      name: 'add-column',
      component: AddColumnView,
      props: true,
      meta: {
        requiresAuth: true,
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
        requiresAuth: true,
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
        requiresAuth: true,
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
        requiresAuth: true,
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
        requiresAuth: true,
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
        requiresAuth: true,
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
        requiresAuth: true,
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
        requiresAuth: true,
        title: 'Create Table',
        icon: DatabaseAddIcon
      },
    },
    {
      path: '/admin/database-browser',
      name: 'database-browser',
      component: DatabaseBrowser,
      meta: {
        requiresAuth: true,
        title: 'Database Browser',
        icon: DatabaseAddIcon
      },
    },
    {
      path: '/admin/control-panel',
      name: 'control-panel',
      component: ControlPanel,
      meta: {
        requiresAuth: true,
        title: 'Control Panel',
        icon: DatabaseAddIcon
      },
    },
    {
      path: '/device-code-claimer',
      name: 'device-code-claimer',
      component: DeviceCodeClaimerView,
      meta: {
        requiresAuth: true,
        title: 'Device Code Claimer',
        icon: DatabaseAddIcon
      },
    },
    // Logout handled by sidebar link with programmatic action
    {
      path: '/:pathMatch(.*)*',
      name: 'not-found',
      component: NotFoundView,
      meta: { requiresAuth: false }
    },
  ],
})

// Navigation guard
router.beforeEach(async (to, from, next) => {
  const authStore = useAuthStore()

  // If navigating to login but already authenticated, redirect to home
  if (to.name === 'login' && authStore.isAuthenticated) {
    next('/')
    return
  }

  // If route doesn't require auth, allow access
  if (to.meta.requiresAuth === false) {
    next()
    return
  }

  // If user is authenticated, allow access
  if (authStore.isAuthenticated) {
    next()
    return
  }

  // Attempt to restore authentication from localStorage before redirecting
  try {
    const ok = await authStore.validateToken()
    if (ok) {
      next()
      return
    }
  } catch {
    // fall through to login
  }

  // If not authenticated, redirect to login
  next({ path: '/login', query: { redirect: to.fullPath } })
})

export default router
