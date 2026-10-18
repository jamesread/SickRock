import { watch } from 'vue'
import { createRouter, createWebHistory, type RouteLocationNormalizedLoaded } from 'vue-router'
import { appTitle, formatDocumentTitle } from './appTitle'
import { rowViewPageLabel } from './utils/rowViewPageTitle'
// The entry points (login, home, table and row pages) load up front; table and row pages
// also work offline from IndexedDB, and the service worker does not cache /assets chunks.
// Every other page is a lazily loaded chunk, keeping the initial bundle small.
import HomeView from './views/HomeView.vue'
const AboutView = () => import('./views/AboutView.vue')
import TableView from './views/TableView.vue'
import RowView from './views/RowView.vue'
const RowEditView = () => import('./views/RowEditView.vue')
const AddColumnView = () => import('./views/AddColumnView.vue')
const InsertRowView = () => import('./views/InsertRowView.vue')
const AfterInsertView = () => import('./views/AfterInsertView.vue')
const CreateTableView = () => import('./views/CreateTableView.vue')
const ForeignKeyManagement = () => import('./views/ForeignKeyManagement.vue')
const ItemViewLayoutEditor = () => import('./views/ItemViewLayoutEditor.vue')
const ColumnTypeManagement = () => import('./views/ColumnTypeManagement.vue')
const ConditionalFormattingRules = () => import('./components/ConditionalFormattingRules.vue')
const ExportView = () => import('./views/ExportView.vue')
const ReadOnlyCalendarExportView = () => import('./views/ReadOnlyCalendarExportView.vue')
const TableCreate = () => import('./views/TableCreate.vue')
const DashboardCreate = () => import('./views/DashboardCreate.vue')
const ControlPanel = () => import('./views/ControlPanel.vue')
const RssCalendarFeedsView = () => import('./views/RssCalendarFeedsView.vue')
const RssCalendarFeedEditView = () => import('./views/RssCalendarFeedEditView.vue')
const PWAInstallation = () => import('./views/PWAInstallation.vue')
import LoginView from './views/LoginView.vue'
const NotFoundView = () => import('./views/NotFoundView.vue')
const DeviceCodeClaimerView = () => import('./views/DeviceCodeClaimerView.vue')
const DatabaseBrowser = () => import('./views/DatabaseBrowser.vue')
import { DatabaseAddIcon } from '@hugeicons/core-free-icons'
const DashboardListView = () => import('./views/DashboardListView.vue')
const Dashboard = () => import('./views/Dashboard.vue')
const WorkflowView = () => import('./views/WorkflowView.vue')
const HealthView = () => import('./views/HealthView.vue')
const UserPreferences = () => import('./views/UserPreferences.vue')
const UserControlPanel = () => import('./views/UserControlPanel.vue')
const UserBookmarks = () => import('./views/UserBookmarks.vue')
const UserAPIKeys = () => import('./views/UserAPIKeys.vue')
const UserNotifications = () => import('./views/UserNotifications.vue')
const DayView = () => import('./views/DayView.vue')
const IamHub = () => import('./views/iam/IamHub.vue')
const IamUsers = () => import('./views/iam/IamUsers.vue')
const IamUserDetails = () => import('./views/iam/IamUserDetails.vue')
const IamUserGroups = () => import('./views/iam/IamUserGroups.vue')
const IamUserGroupDetails = () => import('./views/iam/IamUserGroupDetails.vue')
const IamRbacRoles = () => import('./views/iam/IamRbacRoles.vue')
const IamRbacRoleDetails = () => import('./views/iam/IamRbacRoleDetails.vue')
const IamRbacPermissions = () => import('./views/iam/IamRbacPermissions.vue')
const IamMyPermissions = () => import('./views/iam/IamMyPermissions.vue')
import { useAuthStore } from './stores/auth'
import {
  canAccessAdminTableConfiguration,
  canAccessControlPanel,
  canAccessIam,
} from './utils/rbacAccess'

function isAuthenticatedRouteAllowed(
  to: { meta: Record<string, unknown>; params: Record<string, unknown> },
  authStore: ReturnType<typeof useAuthStore>,
): boolean {
  const perms = authStore.user?.rbacPermissions ?? authStore.initResponse?.rbacPermissions ?? []
  const superuser = authStore.user?.rbacIsSuperuser ?? authStore.initResponse?.rbacIsSuperuser ?? false

  if (to.meta.requiresControlPanel && !canAccessControlPanel(perms, superuser)) {
    return false
  }
  if (to.meta.requiresIam && !canAccessIam(perms, superuser)) {
    return false
  }
  const requiredPerm = to.meta.requiresPermission as string | undefined
  if (requiredPerm && !authStore.hasPermission(requiredPerm)) {
    return false
  }
  const tableName = to.params.tableName
  if (typeof tableName === 'string' && !canAccessAdminTableConfiguration(tableName, perms, superuser)) {
    return false
  }
  return true
}

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
      path: '/exports/:slug',
      name: 'read-only-calendar-export',
      component: ReadOnlyCalendarExportView,
      props: true,
      meta: {
        requiresAuth: true,
        breadcrumbs: (route: { params: { slug?: string } }) => [
          { name: 'Calendar export' },
          { name: String(route.params.slug ?? '') },
        ],
      },
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
      meta: { requiresAuth: true, requiresPermission: 'apikeys.use' }
    },
    {
      path: '/user-notifications',
      name: 'user-notifications',
      component: UserNotifications,
      meta: { requiresAuth: true }
    },
    {
      path: '/my-permissions',
      name: 'my-permissions',
      component: IamMyPermissions,
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
      path: '/table/:tableName/day/:date',
      name: 'day-view',
      component: DayView,
      props: true,
      meta: {
        requiresAuth: true,
        breadcrumbs: (route: any) => {
          const date = new Date(String(route.params.date))
          const formattedDate = date.toLocaleDateString('en-US', { month: 'long', day: 'numeric', year: 'numeric' })
          return [
            { name: String(route.params.tableName), href: { name: 'table', params: { tableName: route.params.tableName } } },
            { name: formattedDate },
          ]
        },
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
          { name: rowViewPageLabel.value?.trim() || `Row ${String(route.params.rowId)}` },
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
          {
            name: rowViewPageLabel.value?.trim() || `Row ${String(route.params.rowId)}`,
            href: { name: 'row', params: { tableName: route.params.tableName, rowId: route.params.rowId } },
          },
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
      path: '/table/:tableName/item-view-layout',
      name: 'item-view-layout',
      component: ItemViewLayoutEditor,
      props: true,
      meta: {
        requiresAuth: true,
        breadcrumbs: (route: RouteLocationNormalizedLoaded) => [
          { name: String(route.params.tableName), href: { name: 'table', params: { tableName: route.params.tableName } } },
          { name: 'Item view layout' },
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
      path: '/admin/health',
      name: 'health',
      component: HealthView,
      meta: {
        requiresAuth: true,
        requiresPermission: 'system.settings',
        title: 'Health',
        icon: DatabaseAddIcon,
      },
    },
    {
      path: '/admin/table/create',
      name: 'table-create',
      component: TableCreate,
      meta: {
        requiresAuth: true,
        requiresPermission: 'system.settings',
        title: 'Create Table',
        icon: DatabaseAddIcon
      },
    },
    {
      path: '/admin/dashboard/create',
      name: 'dashboard-create',
      component: DashboardCreate,
      meta: {
        requiresAuth: true,
        requiresPermission: 'system.settings',
        title: 'Create Dashboard',
        icon: DatabaseAddIcon
      },
    },
    {
      path: '/admin/database-browser',
      name: 'database-browser',
      component: DatabaseBrowser,
      meta: {
        requiresAuth: true,
        requiresPermission: 'system.settings',
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
        requiresControlPanel: true,
        title: 'Control Panel',
        icon: DatabaseAddIcon
      },
    },
    {
      path: '/admin/rss-calendar-feeds',
      name: 'rss-calendar-feeds',
      component: RssCalendarFeedsView,
      meta: {
        requiresAuth: true,
        requiresPermission: 'exports.manage',
        title: 'Calendar feeds',
        icon: DatabaseAddIcon,
      },
    },
    {
      path: '/admin/rss-calendar-feeds/:feedId',
      name: 'rss-calendar-feed-edit',
      component: RssCalendarFeedEditView,
      meta: {
        requiresAuth: true,
        requiresPermission: 'exports.manage',
        title: 'Calendar feed',
        icon: DatabaseAddIcon,
      },
    },
    {
      path: '/admin/iam',
      name: 'iam-hub',
      component: IamHub,
      meta: { requiresAuth: true, requiresIam: true, title: 'IAM' }
    },
    {
      path: '/admin/iam/users',
      name: 'iam-users',
      component: IamUsers,
      meta: { requiresAuth: true, requiresPermission: 'users.view' }
    },
    {
      path: '/admin/iam/users/:id',
      name: 'iam-user-details',
      component: IamUserDetails,
      props: true,
      meta: { requiresAuth: true, requiresPermission: 'users.view' }
    },
    {
      path: '/admin/iam/groups',
      name: 'iam-groups',
      component: IamUserGroups,
      meta: { requiresAuth: true, requiresPermission: 'usergroups.view' }
    },
    {
      path: '/admin/iam/groups/:id',
      name: 'iam-group-details',
      component: IamUserGroupDetails,
      props: true,
      meta: { requiresAuth: true, requiresPermission: 'usergroups.view' }
    },
    {
      path: '/admin/iam/rbac',
      name: 'iam-rbac',
      component: IamRbacRoles,
      meta: { requiresAuth: true, requiresPermission: 'rbac.view' }
    },
    {
      path: '/admin/iam/rbac/permissions',
      name: 'iam-rbac-permissions',
      component: IamRbacPermissions,
      meta: { requiresAuth: true, requiresPermission: 'rbac.view' }
    },
    {
      path: '/admin/iam/rbac/:id',
      name: 'iam-rbac-role-details',
      component: IamRbacRoleDetails,
      props: true,
      meta: { requiresAuth: true, requiresPermission: 'rbac.view' }
    },
    {
      path: '/admin/user-management',
      redirect: { name: 'iam-users' },
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
        requiresPermission: 'devicecode.claim',
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

  // If user is authenticated, check route permissions
  if (authStore.isAuthenticated) {
    if (!isAuthenticatedRouteAllowed(to, authStore)) {
      next('/')
      return
    }
    next()
    return
  }

  // Only attempt to restore authentication if there's a token in localStorage
  // This avoids unnecessary API calls if init already determined user is not authenticated
  const SESSION_TOKEN_KEY = 'session-token'
  const LEGACY_TOKEN_KEY = 'auth_token'
  let token: string | null = null
  try {
    token = localStorage.getItem(SESSION_TOKEN_KEY) || localStorage.getItem(LEGACY_TOKEN_KEY)
  } catch {
    // localStorage might not be available
  }

  // Only call validateToken if we have a token and user is not already set
  // This prevents unnecessary API calls when init already determined user is not authenticated
  if (token && !authStore.user) {
    try {
      const ok = await authStore.validateToken()
      if (ok) {
        if (!isAuthenticatedRouteAllowed(to, authStore)) {
          next('/')
          return
        }
        next()
        return
      }
    } catch {
      // fall through to login
    }
  }

  // If not authenticated, redirect to login
  next({ path: '/login', query: { redirect: to.fullPath } })
})

async function updateDocumentTitle(to: RouteLocationNormalizedLoaded) {
  const baseTitle = appTitle.value
  let title = baseTitle

  if (to.name === 'row' || to.name === 'row-edit') {
    const label = rowViewPageLabel.value?.trim()
    if (label) {
      document.title = formatDocumentTitle(label, baseTitle)
      return
    }
    document.title = formatDocumentTitle(String(to.params.tableName ?? ''), baseTitle)
    return
  }

  // Special handling for table route - load table configuration title
  if (to.name === 'table' && to.params.tableName) {
    try {
      const { useAuthStore } = await import('./stores/auth')
      const authStore = useAuthStore()

      if (!authStore.isAuthenticated) {
        document.title = `${to.params.tableName} - ${baseTitle}`
        return
      }

      const { createApiClient } = await import('./stores/api')
      const client = createApiClient()
      const configs = await client.getTableConfigurations({})
      const config = configs.pages?.find(p => p.id === String(to.params.tableName))
      if (config && config.title) {
        document.title = `${config.title} - ${baseTitle}`
        return
      }
    } catch (e) {
      console.warn('Failed to load table configuration for document title:', e)
    }
  }

  if (to.meta.title) {
    title = `${to.meta.title} - ${baseTitle}`
  } else if (to.meta.breadcrumbs && typeof to.meta.breadcrumbs === 'function') {
    try {
      const breadcrumbs = to.meta.breadcrumbs(to)
      if (breadcrumbs && breadcrumbs.length > 0) {
        const lastBreadcrumb = breadcrumbs[breadcrumbs.length - 1]
        if (lastBreadcrumb && lastBreadcrumb.name) {
          title = `${lastBreadcrumb.name} - ${baseTitle}`
        }
      }
    } catch (e) {
      console.warn('Error generating breadcrumbs for title:', e)
    }
  } else if (to.name && to.name !== 'not-found') {
    const routeName = String(to.name)
      .split('-')
      .map(word => word.charAt(0).toUpperCase() + word.slice(1))
      .join(' ')
    title = `${routeName} - ${baseTitle}`
  }

  document.title = title
}

router.afterEach((to) => {
  void updateDocumentTitle(to)
})

watch(appTitle, () => {
  void updateDocumentTitle(router.currentRoute.value)
})

// After a redeploy, an open tab may request a lazily loaded chunk that no longer exists.
// Reload the target page instead of leaving navigation broken.
router.onError((error, to) => {
  const message = error instanceof Error ? error.message : String(error)
  if (/dynamically imported module|Importing a module script failed|error loading dynamically/i.test(message)) {
    window.location.assign(to.fullPath)
  }
})

export default router
