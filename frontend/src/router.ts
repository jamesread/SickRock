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
import ConditionalFormattingRules from './components/ConditionalFormattingRules.vue'
import ExportView from './views/ExportView.vue'
import TableCreate from './views/TableCreate.vue'
import ControlPanel from './views/ControlPanel.vue'
import PWAInstallation from './views/PWAInstallation.vue'
import LoginView from './views/LoginView.vue'
import NotFoundView from './views/NotFoundView.vue'
import DeviceCodeClaimerView from './views/DeviceCodeClaimerView.vue'
import DatabaseBrowser from './views/DatabaseBrowser.vue'
import { DatabaseAddIcon } from '@hugeicons/core-free-icons'
import DashboardListView from './views/DashboardListView.vue'
import Dashboard from './views/Dashboard.vue'
import WorkflowView from './views/WorkflowView.vue'
import UserPreferences from './views/UserPreferences.vue'
import UserControlPanel from './views/UserControlPanel.vue'
import UserBookmarks from './views/UserBookmarks.vue'
import UserAPIKeys from './views/UserAPIKeys.vue'
import UserNotifications from './views/UserNotifications.vue'
import UserManagement from './views/UserManagement.vue'
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
      path: '/workflow/:workflowId',
      name: 'workflow',
      component: WorkflowView,
      props: true,
      meta: {
        requiresAuth: true,
        breadcrumbs: () => [
          { name: 'Workflow' },
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
      path: '/user-control-panel',
      name: 'user-control-panel',
      component: UserControlPanel,
      meta: { requiresAuth: true }
    },
    {
      path: '/user-preferences',
      name: 'user-preferences',
      component: UserPreferences,
      meta: { requiresAuth: true }
    },
    {
      path: '/user-bookmarks',
      name: 'user-bookmarks',
      component: UserBookmarks,
      meta: { requiresAuth: true }
    },
    {
      path: '/user-api-keys',
      name: 'user-api-keys',
      component: UserAPIKeys,
      meta: { requiresAuth: true }
    },
    {
      path: '/user-notifications',
      name: 'user-notifications',
      component: UserNotifications,
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
          { name: String(route.params.tableName), to: { name: 'table', params: { tableName: route.params.tableName } } },
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
      path: '/table/:tableName/conditional-formatting',
      name: 'conditional-formatting',
      component: ConditionalFormattingRules,
      props: true,
      meta: {
        requiresAuth: true,
        breadcrumbs: (route: any) => [
          { name: String(route.params.tableName), href: { name: 'table', params: { tableName: route.params.tableName } } },
          { name: 'Conditional Formatting' },
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
      path: '/admin/user-management',
      name: 'user-management',
      component: UserManagement,
      meta: {
        requiresAuth: true,
        title: 'User Management',
        icon: DatabaseAddIcon
      },
    },
    {
      path: '/admin/pwa-installation',
      name: 'pwa-installation',
      component: PWAInstallation,
      meta: {
        requiresAuth: true,
        title: 'PWA & Service Worker',
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

// Cache for appTitle to avoid loading it on every route change
let cachedAppTitle: string | null = null
let appTitleLoadPromise: Promise<string> | null = null

async function getAppTitle(): Promise<string> {
  // Return cached value if available
  if (cachedAppTitle !== null) {
    return cachedAppTitle
  }

  // If already loading, return the existing promise
  if (appTitleLoadPromise) {
    return appTitleLoadPromise
  }

  // Load appTitle from settings
  appTitleLoadPromise = (async () => {
    try {
      const { createApiClient } = await import('./stores/api')
      const client = createApiClient()
      const settingsResponse = await client.listItems({ tcName: 'table_settings', where: { setting_key: 'appTitle' } })
      if (settingsResponse.items && settingsResponse.items.length > 0) {
        const appTitleItem = settingsResponse.items[0]
        const stringVal = appTitleItem.additionalFields?.string_val
        if (stringVal) {
          cachedAppTitle = stringVal
          return stringVal
        }
      }
    } catch (e) {
      console.warn('Failed to load appTitle setting, using default:', e)
    }
    // Default fallback
    cachedAppTitle = 'SickRock'
    return 'SickRock'
  })()

  return appTitleLoadPromise
}

// Set document title based on route
router.afterEach(async (to) => {
  const appTitle = await getAppTitle()
  let title = appTitle

  // Special handling for table route - load table configuration title
  if (to.name === 'table' && to.params.tableName) {
    try {
      const { createApiClient } = await import('./stores/api')
      const client = createApiClient()
      const configs = await client.getTableConfigurations({})
      const config = configs.pages?.find(p => p.id === String(to.params.tableName))
      if (config && config.title) {
        title = `${config.title} - ${appTitle}`
        document.title = title
        return
      }
    } catch (e) {
      console.warn('Failed to load table configuration for document title:', e)
      // Fall through to breadcrumb handling
    }
  }

  // If route has a title in meta, use it
  if (to.meta.title) {
    title = `${to.meta.title} - ${appTitle}`
  }
  // Otherwise, try to get title from breadcrumbs
  else if (to.meta.breadcrumbs && typeof to.meta.breadcrumbs === 'function') {
    try {
      const breadcrumbs = to.meta.breadcrumbs(to)
      if (breadcrumbs && breadcrumbs.length > 0) {
        // Use the last breadcrumb as the title
        const lastBreadcrumb = breadcrumbs[breadcrumbs.length - 1]
        if (lastBreadcrumb && lastBreadcrumb.name) {
          title = `${lastBreadcrumb.name} - ${appTitle}`
        }
      }
    } catch (e) {
      // If breadcrumbs function throws an error, fall through to next option
      console.warn('Error generating breadcrumbs for title:', e)
    }
  }
  // Fallback to route name formatted nicely
  else if (to.name && to.name !== 'not-found') {
    const routeName = String(to.name)
      .split('-')
      .map(word => word.charAt(0).toUpperCase() + word.slice(1))
      .join(' ')
    title = `${routeName} - ${appTitle}`
  }

  document.title = title
})

// Set initial title (will be updated when appTitle loads)
getAppTitle().then(title => {
  document.title = title
})

export default router
