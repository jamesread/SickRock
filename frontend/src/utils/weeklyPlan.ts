/**
 * The WeeklyPlan view type: a week of days, each split into user-configured slots (e.g. Breakfast,
 * Lunch, Dinner), with the table's rows placed in them. Settings live in the view's layout_json
 * under "weeklyPlan".
 *
 * A row's day comes from dateColumn. Its slot comes from slotColumn (matched against slot labels)
 * when one is set, otherwise from the time of day in dateColumn: the last slot whose time is not
 * after the row's time.
 */
import type { Item } from '../gen/sickrock_pb'
import { filterPickerOptions, type RowPickerOption } from './rowPicker'

export const WEEKLY_PLAN_VIEW_TYPE = 'weeklyplan'

export type WeeklyPlanSlot = {
  label: string
  /** Time of day, "HH:MM". New entries are dated at it; without a slot column it places rows. */
  time: string
  /** Search text the picker starts with when adding to this slot (e.g. "breakfast"). */
  keywords?: string
}

export type WeeklyPlanConfig = {
  dateColumn: string
  /** Column the slot label is stored in; empty to place rows by time of day instead. */
  slotColumn: string
  /** Column whose value fills a slot, usually a relation to another table. */
  itemColumn: string
  slots: WeeklyPlanSlot[]
  /** 0 = Sunday, 1 = Monday, ... */
  weekStartsOn: number
  /** Users pre-ticked in the Notify dialog. */
  notifyUserIds: number[]
  /** Columns shown under each entry, e.g. a note such as "James: cooking". */
  detailColumns: string[]
  /** Columns filled in when adding an entry (e.g. who a meal is for), offering recently used values. */
  askColumns: string[]
}

export const DEFAULT_WEEKLY_PLAN_SLOTS: WeeklyPlanSlot[] = [
  { label: 'Breakfast', time: '08:00' },
  { label: 'Lunch', time: '12:30' },
  { label: 'Dinner', time: '18:00' },
]

const TIME_PATTERN = /^([01]\d|2[0-3]):([0-5]\d)$/

/** A slot with a trimmed label, a valid time (else none), and keywords only when there are some. */
function cleanSlot(slot: WeeklyPlanSlot): WeeklyPlanSlot {
  const time = slot.time.trim()
  const cleaned: WeeklyPlanSlot = { label: slot.label.trim(), time: TIME_PATTERN.test(time) ? time : '' }
  const keywords = slot.keywords?.trim().replace(/\s+/g, ' ')
  if (keywords) cleaned.keywords = keywords
  return cleaned
}

export function defaultWeeklyPlanConfig(): WeeklyPlanConfig {
  return {
    dateColumn: '',
    slotColumn: '',
    itemColumn: '',
    slots: DEFAULT_WEEKLY_PLAN_SLOTS.map(slot => ({ ...slot })),
    weekStartsOn: 1,
    notifyUserIds: [],
    detailColumns: [],
    askColumns: [],
  }
}

/** Reads the weeklyPlan settings from a view's layout_json, filling in defaults for anything missing. */
export function parseWeeklyPlanConfig(layoutJson: string | undefined): WeeklyPlanConfig {
  const config = defaultWeeklyPlanConfig()
  const raw = parseLayout(layoutJson).weeklyPlan
  if (!raw || typeof raw !== 'object' || Array.isArray(raw)) return config
  const stored = raw as Record<string, unknown>
  const text = (value: unknown) => (typeof value === 'string' ? value.trim() : '')
  config.dateColumn = text(stored.dateColumn)
  config.slotColumn = text(stored.slotColumn)
  config.itemColumn = text(stored.itemColumn)
  if (Array.isArray(stored.slots)) {
    const slots = stored.slots
      .filter((slot): slot is Record<string, unknown> => !!slot && typeof slot === 'object')
      .map(slot => cleanSlot({ label: text(slot.label), time: text(slot.time), keywords: text(slot.keywords) }))
      .filter(slot => slot.label !== '')
    if (slots.length > 0) config.slots = slots
  }
  const weekStartsOn = Number(stored.weekStartsOn)
  if (Number.isInteger(weekStartsOn) && weekStartsOn >= 0 && weekStartsOn <= 6) {
    config.weekStartsOn = weekStartsOn
  }
  config.notifyUserIds = cleanUserIds(stored.notifyUserIds)
  config.detailColumns = cleanColumns(stored.detailColumns)
  config.askColumns = cleanColumns(stored.askColumns)
  return config
}

/** Column names, trimmed, once each, in order; anything else is dropped. */
function cleanColumns(value: unknown): string[] {
  if (!Array.isArray(value)) return []
  return [...new Set(value.filter((column): column is string => typeof column === 'string').map(column => column.trim()).filter(Boolean))]
}

/** An entry's details: the values of the detail columns that have one, in order. */
export function entryDetails(config: WeeklyPlanConfig, getValue: (column: string) => unknown): string[] {
  return config.detailColumns
    .map(column => String(getValue(column) ?? '').trim())
    .filter(Boolean)
}

/** Most recent values offered for a column asked about when adding. */
export const MAX_RECENT_ASK_VALUES = 6

/**
 * The values most recently used in a column, newest row first (by id), once each ignoring case,
 * e.g. ["Evie", "James"] for "who".
 */
export function recentColumnValues(rows: Item[], column: string, limit = MAX_RECENT_ASK_VALUES): string[] {
  const byNewest = [...rows].sort((a, b) => Number(b.id) - Number(a.id))
  const seen = new Set<string>()
  const values: string[] = []
  for (const row of byNewest) {
    const value = String(row.additionalFields?.[column] ?? '').trim()
    if (!value || seen.has(value.toLowerCase())) continue
    seen.add(value.toLowerCase())
    values.push(value)
    if (values.length >= limit) break
  }
  return values
}

/** The asked-for values to store with a new entry: those filled in, trimmed, for the asked columns only. */
export function askedFields(config: WeeklyPlanConfig, values: Record<string, string>): Record<string, string> {
  return Object.fromEntries(config.askColumns
    .map(column => [column, (values[column] ?? '').trim()])
    .filter(([, value]) => value !== ''))
}

/** Positive integer user ids, once each, in order; anything else is dropped. */
function cleanUserIds(value: unknown): number[] {
  if (!Array.isArray(value)) return []
  const ids = value.map(Number).filter(id => Number.isInteger(id) && id > 0)
  return [...new Set(ids)]
}

/** Returns layoutJson with its weeklyPlan settings replaced, keeping every other key. */
export function withWeeklyPlanConfig(layoutJson: string | undefined, config: WeeklyPlanConfig): string {
  const layout = parseLayout(layoutJson)
  layout.weeklyPlan = {
    dateColumn: config.dateColumn,
    slotColumn: config.slotColumn,
    itemColumn: config.itemColumn,
    slots: config.slots
      .map(cleanSlot)
      .filter(slot => slot.label !== ''),
    weekStartsOn: config.weekStartsOn,
    notifyUserIds: cleanUserIds(config.notifyUserIds),
    detailColumns: cleanColumns(config.detailColumns),
    askColumns: cleanColumns(config.askColumns),
  }
  return JSON.stringify(layout)
}

function parseLayout(layoutJson: string | undefined): Record<string, unknown> {
  if (!layoutJson?.trim()) return {}
  try {
    const parsed = JSON.parse(layoutJson)
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : {}
  } catch {
    return {}
  }
}

/** Problems that stop the view from working, for the settings form and the view itself. */
export function weeklyPlanConfigProblems(config: WeeklyPlanConfig): string[] {
  const problems: string[] = []
  if (!config.dateColumn) problems.push('Choose the date column.')
  if (!config.itemColumn) problems.push('Choose the column that fills each slot.')
  if (config.slots.length === 0) problems.push('Add at least one slot.')
  if (!config.slotColumn && config.slots.some(slot => !slot.time)) {
    problems.push('Without a slot column, every slot needs a time.')
  }
  return problems
}

/**
 * Parses a stored date or datetime ("YYYY-MM-DD", "YYYY-MM-DD HH:MM[:SS]", or ISO with a T) as a
 * local wall-clock time, the same way toSqlDateTime writes it.
 */
export function parseWallClock(value: unknown): Date | null {
  if (value == null) return null
  const match = String(value).trim().match(/^(\d{4})-(\d{2})-(\d{2})(?:[ T](\d{2}):(\d{2})(?::(\d{2}))?)?/)
  if (!match) return null
  const [, y, mo, d, h = '0', mi = '0', s = '0'] = match
  const date = new Date(Number(y), Number(mo) - 1, Number(d), Number(h), Number(mi), Number(s))
  return Number.isNaN(date.getTime()) ? null : date
}

/** Local midnight of the first day of the week containing date. */
export function startOfWeek(date: Date, weekStartsOn: number): Date {
  const start = new Date(date.getFullYear(), date.getMonth(), date.getDate())
  const back = (start.getDay() - weekStartsOn + 7) % 7
  start.setDate(start.getDate() - back)
  return start
}

/** The seven days (local midnights) starting at weekStart. */
export function daysOfWeek(weekStart: Date): Date[] {
  return Array.from({ length: 7 }, (_, i) => new Date(weekStart.getFullYear(), weekStart.getMonth(), weekStart.getDate() + i))
}

export function addDays(date: Date, days: number): Date {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate() + days)
}

/** "YYYY-MM-DD" in local time. */
export function dayKey(date: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}

/** The week start a ?week=YYYY-MM-DD link asks for, or null when it is missing or not a date. */
export function weekFromQuery(value: unknown, weekStartsOn: number): Date | null {
  if (typeof value !== 'string') return null
  const match = value.match(/^(\d{4})-(\d{2})-(\d{2})$/)
  if (!match) return null
  const date = new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]))
  if (Number.isNaN(date.getTime()) || dayKey(date) !== value) return null
  return startOfWeek(date, weekStartsOn)
}

/** The Notify dialog's title for a week, e.g. "Meal plan for 2 Nov – 8 Nov is ready". */
export function notifyTitle(viewName: string, weekStart: Date, locale?: string): string {
  const short: Intl.DateTimeFormatOptions = { day: 'numeric', month: 'short' }
  const range = `${weekStart.toLocaleDateString(locale, short)} – ${addDays(weekStart, 6).toLocaleDateString(locale, short)}`
  return `${viewName.trim() || 'Weekly plan'} for ${range} is ready`
}

/** Link to a week of a weekly plan view. */
export function notifyLinkPath(tableId: string, viewId: number | undefined, weekStart: Date): string {
  const query = new URLSearchParams()
  if (viewId && viewId > 0) query.set('view', String(viewId))
  query.set('week', dayKey(weekStart))
  return `/table/${encodeURIComponent(tableId)}?${query.toString()}`
}

function minutesOf(time: string): number {
  const match = time.match(TIME_PATTERN)
  return match ? Number(match[1]) * 60 + Number(match[2]) : 0
}

/**
 * Index of the slot a row belongs in, or -1 when its slot column names no configured slot.
 * Without a slot column: the last slot (by time) not after the row's time, or the earliest slot.
 */
export function slotIndexFor(config: WeeklyPlanConfig, when: Date, slotValue: unknown): number {
  if (config.slotColumn) {
    const value = String(slotValue ?? '').trim().toLowerCase()
    return config.slots.findIndex(slot => slot.label.toLowerCase() === value)
  }
  const minutes = when.getHours() * 60 + when.getMinutes()
  let best = -1
  let earliest = -1
  config.slots.forEach((slot, index) => {
    const slotMinutes = minutesOf(slot.time)
    if (earliest === -1 || slotMinutes < minutesOf(config.slots[earliest].time)) earliest = index
    if (slotMinutes <= minutes && (best === -1 || slotMinutes >= minutesOf(config.slots[best].time))) best = index
  })
  return best === -1 ? earliest : best
}

/** Column values that put a new or moved row on day in slot. */
export function placementFields(config: WeeklyPlanConfig, day: Date, slotIndex: number): Record<string, string> {
  const slot = config.slots[slotIndex]
  const [hours, minutes] = slot?.time ? slot.time.split(':').map(Number) : [0, 0]
  const pad = (n: number) => String(n).padStart(2, '0')
  const fields: Record<string, string> = {
    [config.dateColumn]: `${dayKey(day)} ${pad(hours)}:${pad(minutes)}:00`,
  }
  if (config.slotColumn && slot) fields[config.slotColumn] = slot.label
  return fields
}

/** Key of a day x slot cell. */
export function cellKey(day: Date, slotIndex: number): string {
  return `${dayKey(day)}#${slotIndex}`
}

/**
 * Groups rows into the cells of the week starting at weekStart. getValue reads a column from a row.
 * Rows outside the week, without a date, or naming an unknown slot are left out.
 */
export function groupIntoCells<T>(
  config: WeeklyPlanConfig,
  weekStart: Date,
  rows: T[],
  getValue: (row: T, column: string) => unknown,
): Map<string, T[]> {
  const cells = new Map<string, T[]>()
  const weekEnd = addDays(weekStart, 7)
  for (const row of rows) {
    const when = parseWallClock(getValue(row, config.dateColumn))
    if (!when || when < weekStart || when >= weekEnd) continue
    const slotIndex = slotIndexFor(config, when, config.slotColumn ? getValue(row, config.slotColumn) : undefined)
    if (slotIndex < 0) continue
    const key = cellKey(when, slotIndex)
    const list = cells.get(key)
    if (list) list.push(row)
    else cells.set(key, [row])
  }
  return cells
}

/** Columns the database fills in itself, so copies of a row leave them out. */
export const GENERATED_COLUMNS = new Set(['id', 'sr_created', 'sr_updated'])

/** The row's columns to give a copy of it: every column except the generated ones, without empty values. */
export function copiedFields(columns: string[], values: Record<string, string> | undefined): Record<string, string> {
  return Object.fromEntries(columns
    .filter(column => !GENERATED_COLUMNS.has(column))
    .map(column => [column, values?.[column] ?? ''])
    .filter(([, value]) => value !== ''))
}

export type PlannedWeek = { start: Date; count: number }

/** The weeks that have entries (rows the grid would place), newest first, with how many each has. */
export function plannedWeeks<T>(config: WeeklyPlanConfig, rows: T[], getValue: (row: T, column: string) => unknown): PlannedWeek[] {
  const weeks = new Map<string, PlannedWeek>()
  for (const row of rows) {
    const when = parseWallClock(getValue(row, config.dateColumn))
    if (!when) continue
    if (slotIndexFor(config, when, config.slotColumn ? getValue(row, config.slotColumn) : undefined) < 0) continue
    const start = startOfWeek(when, config.weekStartsOn)
    const week = weeks.get(dayKey(start))
    if (week) week.count++
    else weeks.set(dayKey(start), { start, count: 1 })
  }
  return [...weeks.values()].sort((a, b) => b.start.getTime() - a.start.getTime())
}

/** Whole days from one date to another (local calendar days, so daylight saving does not matter). */
function daysBetween(from: Date, to: Date): number {
  return Math.round((Date.UTC(to.getFullYear(), to.getMonth(), to.getDate()) - Date.UTC(from.getFullYear(), from.getMonth(), from.getDate())) / 86_400_000)
}

/** "YYYY-MM-DD HH:MM:SS" in local time, as placementFields writes dates. */
function wallClockText(date: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${dayKey(date)} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

export type WeekCopy = {
  /** The new rows' column values: each copied row moved to the same weekday and time in the target week. */
  rows: Record<string, string>[]
  /** Rows left out because the target week already has the same item in the same cell. */
  skipped: number
}

/**
 * Copies the entries of the week starting at fromWeek into the week starting at toWeek. Each row
 * keeps its columns (see copiedFields) and its weekday, time and slot. An entry whose cell in the
 * target week already holds the same item is skipped, so copying twice adds nothing.
 */
export function copyWeek(
  config: WeeklyPlanConfig,
  rows: Item[],
  columns: string[],
  fromWeek: Date,
  toWeek: Date,
): WeekCopy {
  const getValue = (row: Item, column: string) => (column === 'id' ? row.id : row.additionalFields?.[column])
  const shift = daysBetween(fromWeek, toWeek)
  const itemKey = (cell: string, row: Item) => `${cell}|${String(getValue(row, config.itemColumn) ?? '')}`
  const existing = new Set<string>()
  for (const [cell, cellRows] of groupIntoCells(config, toWeek, rows, getValue)) {
    for (const row of cellRows) existing.add(itemKey(cell, row))
  }
  const copy: WeekCopy = { rows: [], skipped: 0 }
  if (shift === 0) return copy
  for (const [, cellRows] of groupIntoCells(config, fromWeek, rows, getValue)) {
    for (const row of cellRows) {
      const when = parseWallClock(getValue(row, config.dateColumn))
      if (!when) continue
      const moved = new Date(when.getFullYear(), when.getMonth(), when.getDate() + shift, when.getHours(), when.getMinutes(), when.getSeconds())
      const slotIndex = slotIndexFor(config, moved, config.slotColumn ? getValue(row, config.slotColumn) : undefined)
      const key = itemKey(cellKey(moved, slotIndex), row)
      if (existing.has(key)) {
        copy.skipped++
        continue
      }
      existing.add(key)
      copy.rows.push({ ...copiedFields(columns, row.additionalFields), [config.dateColumn]: wallClockText(moved) })
    }
  }
  return copy
}

/** An option in the weekly plan's picker (see utils/rowPicker.ts). */
export type WeeklyPlanPickerOption = RowPickerOption

export { filterPickerOptions }
