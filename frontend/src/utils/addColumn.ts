// What can be added to a table: standard fields (well-known columns, added in one step) and the
// data types a custom column can have. Types other than the storage types are column semantics
// stored on one (user_ref and image on BIGINT, icon and barcode on TEXT).

export type NewColumnType = 'string' | 'int64' | 'tinyint' | 'datetime' | 'user_ref' | 'image' | 'icon' | 'barcode'

export type ColumnTypeOption = {
  value: NewColumnType
  label: string
  description: string
  /** Enhanced types get their own editor in forms (picker, upload, scanner). */
  enhanced: boolean
}

export const COLUMN_TYPE_OPTIONS: ColumnTypeOption[] = [
  { value: 'string', label: 'Text', description: 'Any text.', enhanced: false },
  { value: 'int64', label: 'Number', description: 'A whole number.', enhanced: false },
  { value: 'tinyint', label: 'Yes / no', description: 'A checkbox.', enhanced: false },
  { value: 'datetime', label: 'Date and time', description: 'A date and time, picked from a calendar.', enhanced: false },
  { value: 'user_ref', label: 'User', description: 'A SickRock user, shown as a username chip and picked from a list.', enhanced: true },
  { value: 'image', label: 'Image', description: 'An image, uploaded from a file, pasted, or taken with the device camera.', enhanced: true },
  { value: 'icon', label: 'Icon', description: 'An icon, chosen with the icon picker.', enhanced: true },
  { value: 'barcode', label: 'Barcode', description: 'A barcode or QR code, typed, read by a barcode scanner, or scanned with the device camera.', enhanced: true },
]

export function isNewColumnType(value: unknown): value is NewColumnType {
  return COLUMN_TYPE_OPTIONS.some(o => o.value === value)
}

export function columnTypeOption(value: NewColumnType): ColumnTypeOption {
  return COLUMN_TYPE_OPTIONS.find(o => o.value === value) ?? COLUMN_TYPE_OPTIONS[0]
}

export type StandardField = {
  name: string
  type: NewColumnType
  label: string
  description: string
  recommended: boolean
  defaultToCurrentTimestamp?: boolean
  /** Also make it the bulk entry scan column, when the table has none yet. */
  bulkEntryScan?: boolean
}

export const STANDARD_FIELDS: StandardField[] = [
  {
    name: 'name',
    type: 'string',
    label: 'Name',
    description: 'The row\'s name, used to identify it in lists, links and pickers.',
    recommended: true,
  },
  {
    name: 'sr_created',
    type: 'datetime',
    label: 'Created',
    description: 'When the row was created, filled in automatically. Useful for sorting and auditing.',
    recommended: true,
    defaultToCurrentTimestamp: true,
  },
  {
    name: 'sr_updated',
    type: 'datetime',
    label: 'Updated',
    description: 'When the row was last changed, kept up to date automatically.',
    recommended: true,
    defaultToCurrentTimestamp: true,
  },
  {
    name: 'barcode',
    type: 'barcode',
    label: 'Barcode',
    description: 'A product barcode, scanned with the device camera or a barcode scanner. Also turns on Bulk entry, scanning into this column, if the table does not use it yet.',
    recommended: false,
    bulkEntryScan: true,
  },
  {
    name: 'uuid',
    type: 'string',
    label: 'UUID',
    description: 'A universally unique identifier, for external systems and integrations.',
    recommended: false,
  },
  {
    name: 'owner_id',
    type: 'user_ref',
    label: 'Owner',
    description: 'The SickRock user who owns the row, shown as a username chip.',
    recommended: false,
  },
]

export function isStandardFieldName(name: string): boolean {
  return STANDARD_FIELDS.some(f => f.name === name)
}

/** Standard fields the table does not have yet (column names compare case-insensitively). */
export function missingStandardFields(existingColumns: string[]): StandardField[] {
  const have = new Set(existingColumns.map(c => c.toLowerCase()))
  return STANDARD_FIELDS.filter(f => !have.has(f.name.toLowerCase()))
}

/** Whether a column name is already used (case-insensitively). */
export function columnNameTaken(name: string, existingColumns: string[]): boolean {
  const lower = name.trim().toLowerCase()
  return lower !== '' && existingColumns.some(c => c.toLowerCase() === lower)
}
