import * as Hugeicons from '@hugeicons/core-free-icons'
import { DatabaseIcon } from '@hugeicons/core-free-icons'

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

export function resolveNavigationIcon(ctx: NavigationIconContext) {
  const iconName = ctx.icon?.trim() || defaultNavigationIconName(ctx)
  const fallbackName = defaultNavigationIconName(ctx)
  return (Hugeicons as Record<string, typeof DatabaseIcon>)[iconName]
    ?? (Hugeicons as Record<string, typeof DatabaseIcon>)[fallbackName]
    ?? DatabaseIcon
}
