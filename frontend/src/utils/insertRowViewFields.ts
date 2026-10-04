export type InsertFieldDef = {
  name: string
  type: string
  required: boolean
}

export type InsertViewColumn = {
  columnName: string
  isVisible: boolean
  columnOrder: number
}

export type InsertViewForFields = {
  id: number
  columns: InsertViewColumn[]
}

export function fieldDefsForInsertForm(
  fields: Array<{ name: string; type: string; required?: boolean }>,
): InsertFieldDef[] {
  return fields
    .filter(field => field.name !== 'sr_created' && field.name !== 'sr_updated')
    .map(field => ({
      name: field.name,
      type: field.type,
      required: !!field.required,
    }))
}

/** Match insert-row page: order by view, hide non-visible in-view columns, append columns not in view. */
export function applyViewToInsertFieldDefs(
  defs: InsertFieldDef[],
  views: InsertViewForFields[],
  selectedViewId: number | null,
): InsertFieldDef[] {
  if (selectedViewId === null || selectedViewId === -1) {
    return defs
  }

  const currentView = views.find(view => view.id === selectedViewId)
  if (!currentView?.columns?.length) {
    return defs
  }

  const orderMap: Record<string, number> = {}
  const visibilityMap: Record<string, boolean> = {}
  for (const col of currentView.columns) {
    orderMap[col.columnName] = col.columnOrder
    visibilityMap[col.columnName] = col.isVisible
  }

  const inView: InsertFieldDef[] = []
  const notInView: InsertFieldDef[] = []
  for (const def of defs) {
    if (orderMap[def.name] != null) {
      inView.push(def)
    } else {
      notInView.push(def)
    }
  }

  inView.sort((a, b) => (orderMap[a.name] ?? 0) - (orderMap[b.name] ?? 0))
  const visibleInView = inView.filter(def => visibilityMap[def.name] !== false)
  return [...visibleInView, ...notInView]
}
