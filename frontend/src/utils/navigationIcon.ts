import { DatabaseIcon, LayoutIcon, WorkflowIcon } from '@hugeicons/core-free-icons'
import { whenHugeiconAvailable, type IconData } from './hugeicons'

export type NavigationIconContext = {
  icon?: string
  workflowId?: number
  dashboardId?: number
  path?: string
}

export function defaultNavigationIconName(ctx: NavigationIconContext): string {
  if (ctx.dashboardId && ctx.dashboardId > 0) return 'LayoutIcon'
  if (ctx.path?.startsWith('/dashboard/')) return 'LayoutIcon'
  if (ctx.workflowId && ctx.workflowId > 0) return 'WorkflowIcon'
  if (ctx.path?.startsWith('/workflow/')) return 'WorkflowIcon'
  return 'DatabaseIcon'
}

const DEFAULT_ICONS: Record<string, IconData> = { DatabaseIcon, LayoutIcon, WorkflowIcon }

/** The bundled default icon for a navigation target (shown until a configured icon loads). */
export function defaultNavigationIcon(ctx: NavigationIconContext): IconData {
  return DEFAULT_ICONS[defaultNavigationIconName(ctx)] ?? DatabaseIcon
}

/**
 * Returns the bundled default icon immediately. When a different icon is configured,
 * it is loaded on demand and passed to `onLoaded` (which may never be called, e.g. for
 * an unknown icon name).
 */
export function resolveNavigationIcon(
  ctx: NavigationIconContext,
  onLoaded?: (icon: IconData) => void,
): IconData {
  const name = ctx.icon?.trim()
  if (onLoaded && name && !DEFAULT_ICONS[name]) {
    whenHugeiconAvailable(name, onLoaded)
  } else if (name && DEFAULT_ICONS[name]) {
    return DEFAULT_ICONS[name]
  }
  return defaultNavigationIcon(ctx)
}
