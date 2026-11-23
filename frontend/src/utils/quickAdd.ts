/**
 * Quick add: an inline entry row under a relation's related rows panel (e.g. a recipe's
 * ingredients), so child rows can be typed in one after another without leaving the parent's page.
 * Configured per relation in table_relations.quick_add_json (Manage relation → Quick add).
 */
import type { Item } from '../gen/sickrock_pb'
import { rowName } from './tickList'

export type QuickAddConfig = {
  enabled: boolean
  /** Related table columns shown, in order; empty for every writable column. */
  columns: string[]
  /** A name typed into a relation column that matches no row creates that row. */
  createMissing: boolean
}

export type QuickAddField = { name: string; type: string; displayLabel?: string }

export function defaultQuickAddConfig(): QuickAddConfig {
  return { enabled: false, columns: [], createMissing: false }
}

/** Reads a relation's quick add settings; anything invalid gives the (disabled) default. */
export function parseQuickAddConfig(json: string | undefined): QuickAddConfig {
  const config = defaultQuickAddConfig()
  if (!json?.trim()) return config
  try {
    const raw = JSON.parse(json)
    if (!raw || typeof raw !== 'object' || Array.isArray(raw)) return config
    config.enabled = raw.enabled === true
    config.createMissing = raw.createMissing === true
    if (Array.isArray(raw.columns)) {
      config.columns = [...new Set(raw.columns.filter((c: unknown): c is string => typeof c === 'string').map((c: string) => c.trim()).filter(Boolean))] as string[]
    }
  } catch {
    // Keep the default.
  }
  return config
}

export function serializeQuickAddConfig(config: QuickAddConfig): string {
  return JSON.stringify({ enabled: config.enabled, columns: config.columns, createMissing: config.createMissing })
}

const SYSTEM_COLUMNS = new Set(['id', 'sr_created', 'sr_updated'])

/** Columns a quick add row can offer: writable columns other than the link to the parent. */
export function quickAddCandidateColumns(fields: QuickAddField[], linkColumn: string, primaryKey = 'id'): QuickAddField[] {
  return fields.filter(field => field.name !== linkColumn
    && field.name !== primaryKey
    && !SYSTEM_COLUMNS.has(field.name)
    && !field.name.startsWith('__pt_')
    && !field.name.startsWith('__calc_'))
}

/** The columns the row shows: the configured ones that exist, else every candidate. */
export function quickAddColumns(config: QuickAddConfig, fields: QuickAddField[], linkColumn: string, primaryKey = 'id'): QuickAddField[] {
  const candidates = quickAddCandidateColumns(fields, linkColumn, primaryKey)
  if (config.columns.length === 0) return candidates
  const byName = new Map(candidates.map(field => [field.name, field]))
  return config.columns.map(name => byName.get(name)).filter((field): field is QuickAddField => !!field)
}

/** The input to use for a column type. */
export function quickAddInputType(type: string): 'text' | 'number' | 'datetime-local' | 'checkbox' {
  const t = type.toLowerCase()
  if (t.startsWith('tinyint') || t === 'bool' || t === 'boolean') return 'checkbox'
  if (/date|time/.test(t)) return 'datetime-local'
  if (/^(int|bigint|smallint|mediumint|integer|int64|decimal|numeric|float|double|real)/.test(t)) return 'number'
  return 'text'
}

/** The referenced row a typed value means: its name (ignoring case and spacing), else its id. */
export function matchReferencedRow(rows: Item[], typed: string): Item | undefined {
  const text = typed.trim().toLowerCase()
  if (!text) return undefined
  return rows.find(row => rowName(row).toLowerCase() === text)
    ?? rows.find(row => String(row.id) === typed.trim().replace(/^#/, ''))
}

/** The column a new referenced row's name goes in ("name", else "title"), or "" when it has neither. */
export function nameColumnOf(fields: QuickAddField[]): string {
  return ['name', 'title'].find(name => fields.some(field => field.name === name)) ?? ''
}

/** Datetime-local input value ("2026-11-22T18:00") as the server's "2026-11-22 18:00:00". */
export function quickAddValue(type: string, value: string | boolean): string {
  if (typeof value === 'boolean') return value ? '1' : '0'
  if (quickAddInputType(type) === 'datetime-local' && /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/.test(value)) {
    return value.replace('T', ' ') + ':00'
  }
  return value.trim()
}
