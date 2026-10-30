/**
 * The Cards view type: the table's rows as a grid of cards, each with an image (from a chosen
 * image column), a title and a few detail columns. Settings live in the view's layout_json under
 * "cards".
 */
import { isImageFieldType } from './fieldTypes'

export const CARDS_VIEW_TYPE = 'cards'

export const CARD_SIZES = ['small', 'medium', 'large'] as const
export type CardSize = (typeof CARD_SIZES)[number]

export type CardsConfig = {
  /** Image column shown across the top of each card; empty for no image. */
  imageColumn: string
  /** Column used as the card's title; empty to use the row's name, title or id. */
  titleColumn: string
  /** Columns listed under the title. */
  detailColumns: string[]
  size: CardSize
}

/** Most detail columns a card shows. */
export const MAX_CARD_DETAILS = 4

export function defaultCardsConfig(): CardsConfig {
  return { imageColumn: '', titleColumn: '', detailColumns: [], size: 'medium' }
}

/** A sensible starting config for a table: its first image column and its name/title column. */
export function suggestedCardsConfig(fields: Array<{ name: string; type: string }>): CardsConfig {
  const names = fields.map(field => field.name)
  return {
    ...defaultCardsConfig(),
    imageColumn: fields.find(field => isImageFieldType(field.type))?.name ?? '',
    titleColumn: ['name', 'title', 'label'].find(name => names.includes(name)) ?? '',
  }
}

/** Reads the cards settings from a view's layout_json, filling in defaults for anything missing. */
export function parseCardsConfig(layoutJson: string | undefined): CardsConfig {
  const config = defaultCardsConfig()
  const raw = parseLayout(layoutJson).cards
  if (!raw || typeof raw !== 'object' || Array.isArray(raw)) return config
  const stored = raw as Record<string, unknown>
  const text = (value: unknown) => (typeof value === 'string' ? value.trim() : '')
  config.imageColumn = text(stored.imageColumn)
  config.titleColumn = text(stored.titleColumn)
  if (Array.isArray(stored.detailColumns)) {
    config.detailColumns = [...new Set(stored.detailColumns.map(text).filter(Boolean))].slice(0, MAX_CARD_DETAILS)
  }
  if (CARD_SIZES.includes(stored.size as CardSize)) config.size = stored.size as CardSize
  return config
}

/** Returns layoutJson with its cards settings replaced, keeping every other key. */
export function withCardsConfig(layoutJson: string | undefined, config: CardsConfig): string {
  const layout = parseLayout(layoutJson)
  layout.cards = {
    imageColumn: config.imageColumn,
    titleColumn: config.titleColumn,
    detailColumns: [...new Set(config.detailColumns.filter(Boolean))].slice(0, MAX_CARD_DETAILS),
    size: config.size,
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

/**
 * Rows where every word of query appears in one of the row's texts (e.g. its title and the details
 * shown on its card), ignoring case and accents. Words may match different texts.
 */
export function filterByText<T>(rows: T[], query: string, textsOf: (row: T) => string[]): T[] {
  const fold = (text: string) => text.normalize('NFD').replace(/\p{Diacritic}/gu, '').toLowerCase()
  const words = fold(query).split(/\s+/).filter(Boolean)
  if (words.length === 0) return rows
  return rows.filter(row => {
    // Words hold no whitespace, so joining on a newline never matches across two texts.
    const text = fold(textsOf(row).join('\n'))
    return words.every(word => text.includes(word))
  })
}
