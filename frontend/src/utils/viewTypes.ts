/** The kinds of table view, as stored in table_views.view_type, with their names. */
export type ViewType = 'table' | 'calendar' | 'ticklist' | 'weeklyplan' | 'cards'

export const VIEW_TYPE_OPTIONS: ReadonlyArray<{ value: ViewType; label: string }> = [
  { value: 'table', label: 'Table' },
  { value: 'calendar', label: 'Calendar' },
  { value: 'ticklist', label: 'Tick List' },
  { value: 'weeklyplan', label: 'Weekly Plan' },
  { value: 'cards', label: 'Cards' },
]

/** A view type's name; an empty type is a table view (the default), and an unknown one is shown as stored. */
export function viewTypeLabel(viewType: string | undefined | null): string {
  const type = (viewType ?? '').trim() || 'table'
  return VIEW_TYPE_OPTIONS.find(option => option.value === type)?.label ?? type
}
