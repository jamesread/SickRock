// PicoCrank Table `viewSettings` callbacks (viewControlMode="callbackControlled") backed by
// SickRock server table views, so views saved from PicoCrank's view menu are stored on
// the server (and listed under the Views button) instead of in browser localStorage.
import { computed, type Ref } from 'vue'
import type { createApiClient } from '../stores/api'
import type { TableView } from './useTableViewManager'
import { presetStateFromView, viewPayloadFromState, type LayoutState } from '../utils/serverTableViews'

type ApiClient = ReturnType<typeof createApiClient>
type Result = { ok: boolean; error?: string; preset?: Preset | null }
type Preset = { id: string; name: string; state: LayoutState }

export type ServerTableViewSettingsOptions = {
  client: ApiClient
  tableId: () => string
  /** The table's server views, as loaded by the owning component. */
  views: Ref<TableView[]>
  /** The calling user's own default view id (0 when unset). */
  userDefaultViewId: Ref<number>
  /** Real field names of the table (excludes the actions column and synthetic columns). */
  fieldNames: () => string[]
  /** Whether the user may create, overwrite and delete views (personal defaults are always allowed). */
  canEdit: () => boolean
  /** Reloads `views` and `userDefaultViewId` from the server. */
  reload: () => Promise<void>
  /** Shows a view in the owning component (same path as the Views button). */
  select: (viewId: number) => void
}

function errorMessage(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message) return error.message
  return fallback
}

export function useServerTableViewSettings(options: ServerTableViewSettingsOptions) {
  const { client, views, userDefaultViewId } = options

  const tableLayoutViews = () => views.value.filter(view => view.viewType === 'table')
  const toPreset = (view: TableView): Preset => ({
    id: String(view.id),
    name: view.viewName,
    state: presetStateFromView(view, options.fieldNames()),
  })
  const findView = (presetId: string) => views.value.find(view => String(view.id) === presetId) ?? null

  async function setDefault(presetId: string | null): Promise<Result> {
    try {
      await client.setUserDefaultTableView({ tableName: options.tableId(), viewId: presetId ? Number(presetId) : 0 })
      await options.reload()
      return { ok: true }
    } catch (error) {
      return { ok: false, error: errorMessage(error, 'Unable to update default view.') }
    }
  }

  async function save({ name, state, setAsDefault }: { name: string; state: LayoutState; setAsDefault?: boolean }): Promise<Result> {
    try {
      const payload = viewPayloadFromState(state, options.fieldNames())
      const res = await client.createTableView({
        tableName: options.tableId(),
        viewName: name,
        viewType: 'table',
        columns: payload.columns,
        layoutJson: payload.layoutJson,
      })
      if (!res.success) return { ok: false, error: res.message || 'Unable to save view.' }
      if (setAsDefault) {
        await client.setUserDefaultTableView({ tableName: options.tableId(), viewId: res.viewId })
      }
      await options.reload()
      options.select(res.viewId)
      const view = findView(String(res.viewId))
      return { ok: true, preset: view ? toPreset(view) : null }
    } catch (error) {
      return { ok: false, error: errorMessage(error, 'Unable to save view.') }
    }
  }

  async function overwrite({ presetId, state }: { presetId: string; state: LayoutState }): Promise<Result> {
    const view = findView(presetId)
    if (!view) return { ok: false, error: 'View not found.' }
    try {
      const payload = viewPayloadFromState(state, options.fieldNames())
      const res = await client.updateTableView({
        viewId: view.id,
        tableName: options.tableId(),
        viewName: view.viewName,
        viewType: view.viewType,
        columns: payload.columns,
        layoutJson: payload.layoutJson,
      })
      if (!res.success) return { ok: false, error: res.message || 'Unable to overwrite view.' }
      await options.reload()
      const updated = findView(presetId)
      return { ok: true, preset: updated ? toPreset(updated) : null }
    } catch (error) {
      return { ok: false, error: errorMessage(error, 'Unable to overwrite view.') }
    }
  }

  async function remove(presetId: string): Promise<Result> {
    try {
      const res = await client.deleteTableView({ viewId: Number(presetId) })
      if (!res.success) return { ok: false, error: res.message || 'Unable to delete view.' }
      await options.reload()
      return { ok: true }
    } catch (error) {
      return { ok: false, error: errorMessage(error, 'Unable to delete view.') }
    }
  }

  // Recomputed whenever the views change, which makes PicoCrank refresh its view list.
  return computed(() => {
    const presets = tableLayoutViews().map(toPreset)
    const defaultPresetId = presets.some(preset => preset.id === String(userDefaultViewId.value))
      ? String(userDefaultViewId.value)
      : null

    return {
      list: () => ({ presets, defaultPresetId }),
      load: (presetId: string): Result => {
        const preset = presets.find(candidate => candidate.id === presetId)
        if (!preset) return { ok: false, error: 'View not found.' }
        options.select(Number(presetId))
        return { ok: true, preset }
      },
      setDefault,
      ...(options.canEdit() ? { save, overwrite, delete: remove } : {}),
    }
  })
}
