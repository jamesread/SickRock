import type { InjectionKey, Ref } from 'vue'
import type { Field } from '../gen/sickrock_pb'
import type { IamUser } from '../gen/sickrock_pb'

export type RowViewFieldUi = {
  /** Table configuration of the row (image uploads are made against it). */
  tableId: string
  editingField: Ref<string | null>
  editingValue: Ref<string>
  savingField: Ref<boolean>
  users: Ref<IamUser[]>
  setEditInput: (element: unknown) => void
  displayFieldLabel: (key: string) => string
  resolveRawFieldValue: (key: string) => unknown
  getFkDisplay: (key: string) => { to?: string; label?: string }
  isUserRefColumn: (column: string) => boolean
  isTinyintColumn: (column: string) => boolean
  isDatetimeColumn: (column: string) => boolean
  getFieldDef: (fieldName: string) => Field | undefined
  getBooleanValue: (column: string) => boolean
  canEditField: (key: string, valueClass: string) => boolean
  showFullEditIcon: (key: string, valueClass: string) => boolean
  showReadOnlyIcon: (key: string, valueClass: string) => boolean
  fullEditTo: () => string
  fullEditLabel: () => string
  startEdit: (key: string, valueClass: string) => void
  cancelEdit: () => void
  saveField: () => void | Promise<void>
  hasStoredUserRefValue: (value: unknown) => boolean
  displayUserLabel: (value: unknown) => string
}

export const rowViewFieldUiKey: InjectionKey<RowViewFieldUi> = Symbol('rowViewFieldUi')
