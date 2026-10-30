/**
 * The WeeklyPlan view type: a week of days, each split into user-configured slots (e.g. Breakfast,
 * Lunch, Dinner), with the table's rows placed in them. Settings live in the view's layout_json
 * under "weeklyPlan".
 *
 * A row's day comes from dateColumn. Its slot comes from slotColumn (matched against slot labels)
 * when one is set, otherwise from the time of day in dateColumn: the last slot whose time is not
 * after the row's time.
 */
import { filterByText } from './cardsView'

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
  return config
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

export type WeeklyPlanPickerOption = {
  value: string
  label: string
  imageId: string
  /** Extra fields shown on the tile, from the referenced table's Cards view. */
  details?: Array<{ label: string; value: string }>
}

/**
 * Picker options whose label (the referenced row's name) and details together contain every word
 * of query, ignoring case and accents.
 */
export function filterPickerOptions<T extends { label: string; details?: Array<{ value: string }> }>(
  options: T[],
  query: string,
): T[] {
  return filterByText(options, query, option => [option.label, ...(option.details ?? []).map(detail => detail.value)])
}
