// Loads what RowPickerDialog needs to choose a row of one table (a relation's target): its rows as
// options, whether to show tiles or a list (see utils/rowPicker.ts), and whether and how a new row
// can be created from the dialog. The table can change (InsertRow shares one dialog between its
// relation fields); each table's settings are loaded once.
import { computed, ref, watch, type Ref } from 'vue'
import type { Item } from '../gen/sickrock_pb'
import { createApiClient } from '../stores/api'
import { useAuthStore } from '../stores/auth'
import { useTableAccess } from './useTableAccess'
import { fieldDefsForInsertForm, type InsertFieldDef } from '../utils/insertRowViewFields'
import { rowInsertActionLabel } from '../utils/rowInsertLabel'
import {
  pickerColumns,
  pickerImageId,
  pickerLabel,
  pickerOptions,
  recentPicks,
  recentPicksKey,
  rememberPick,
  type RowPickerColumns,
} from '../utils/rowPicker'

type TableSettings = {
  columns: RowPickerColumns
  fieldTypes: Record<string, string>
  fieldLabels: Record<string, string>
  fieldDefs: InsertFieldDef[]
  /** "Create <row name>", or "" when the table has a create_delegate (its own create page). */
  createLabel: string
}

const NO_SETTINGS: TableSettings = {
  columns: { layout: 'list', imageColumn: '', detailColumns: [] },
  fieldTypes: {},
  fieldLabels: {},
  fieldDefs: [],
  createLabel: '',
}

export function useRowPicker(tcName: Ref<string>) {
  const client = createApiClient()
  const auth = useAuthStore()
  const { access } = useTableAccess(tcName)
  const settingsByTable = new Map<string, Promise<TableSettings>>()
  const settings = ref<TableSettings>(NO_SETTINGS)
  const rows = ref<Item[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)
  const recent = ref<string[]>([])

  const recentKey = computed(() => recentPicksKey(auth.user?.username ?? '', tcName.value))
  const layout = computed(() => settings.value.columns.layout)
  const options = computed(() => pickerOptions(rows.value, settings.value.columns, settings.value.fieldTypes, settings.value.fieldLabels))
  const createLabel = computed(() => (access.value.canInsert ? settings.value.createLabel : ''))
  const rowsById = computed(() => new Map(rows.value.map(row => [String(row.id), row])))

  async function loadSettings(table: string): Promise<TableSettings> {
    const [structure, views] = await Promise.all([
      client.getTableStructure({ pageId: table }),
      client.getTableViews({ tableName: table }),
    ])
    const fields = structure.fields ?? []
    return {
      columns: pickerColumns(fields, views.views ?? [], views.userDefaultViewId),
      fieldTypes: Object.fromEntries(fields.map(field => [field.name, field.type])),
      fieldLabels: Object.fromEntries(fields.map(field => [field.name, field.displayLabel ?? ''])),
      fieldDefs: fieldDefsForInsertForm(fields),
      createLabel: structure.createDelegate ? '' : rowInsertActionLabel(structure.rowName),
    }
  }

  /** Fetches the table's rows again (e.g. after one was created). */
  async function reloadRows(): Promise<Item[]> {
    const table = tcName.value
    if (!table) return []
    const res = await client.listItems({ tcName: table })
    if (table === tcName.value) rows.value = res.items ?? []
    return res.items ?? []
  }

  /**
   * Loads the current table's settings and rows. Pass rows the caller already holds to skip
   * fetching them again.
   */
  async function load(knownRows?: Item[]) {
    const table = tcName.value
    if (!table) {
      settings.value = NO_SETTINGS
      rows.value = []
      return
    }
    loading.value = true
    error.value = null
    try {
      let pending = settingsByTable.get(table)
      if (!pending) {
        pending = loadSettings(table)
        settingsByTable.set(table, pending)
        // A failed load is retried next time rather than cached.
        pending.catch(() => settingsByTable.delete(table))
      }
      const [loaded] = await Promise.all([pending, knownRows ? Promise.resolve() : reloadRows()])
      if (table !== tcName.value) return
      settings.value = loaded
      if (knownRows) rows.value = knownRows
    } catch (e) {
      error.value = e instanceof Error ? e.message : String(e)
    } finally {
      loading.value = false
    }
  }

  /** Records a pick so it is offered first next time. */
  function remember(value: string) {
    recent.value = rememberPick(recentKey.value, value)
  }

  watch(recentKey, key => { recent.value = recentPicks(key) }, { immediate: true })

  return {
    access,
    settings,
    rows,
    rowsById,
    options,
    layout,
    createLabel,
    recent,
    loading,
    error,
    load,
    reloadRows,
    remember,
    labelOf: pickerLabel,
    imageIdOf: (row: Item | undefined) => pickerImageId(row, settings.value.columns.imageColumn),
  }
}
