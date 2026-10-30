import { describe, expect, it } from 'vitest'

import {
  MAX_CARD_DETAILS,
  defaultCardsConfig,
  filterByText,
  parseCardsConfig,
  suggestedCardsConfig,
  withCardsConfig,
} from './cardsView'

describe('parseCardsConfig', () => {
  it('defaults when layout_json has no cards settings', () => {
    expect(parseCardsConfig('')).toEqual(defaultCardsConfig())
    expect(parseCardsConfig('not json')).toEqual(defaultCardsConfig())
    expect(parseCardsConfig('{"pageSize":20}')).toEqual(defaultCardsConfig())
  })

  it('reads stored settings, dropping bad values', () => {
    const layout = JSON.stringify({
      cards: { imageColumn: ' main_picture ', titleColumn: 'name', detailColumns: ['a', '', 'a', 'b', 'c', 'd', 'e', 7], size: 'huge' },
    })
    expect(parseCardsConfig(layout)).toEqual({
      imageColumn: 'main_picture',
      titleColumn: 'name',
      detailColumns: ['a', 'b', 'c', 'd'].slice(0, MAX_CARD_DETAILS),
      size: 'medium',
    })
  })
})

describe('withCardsConfig', () => {
  it('replaces the cards settings and keeps other keys', () => {
    const json = withCardsConfig('{"pageSize":20}', { imageColumn: 'img', titleColumn: '', detailColumns: ['x'], size: 'large' })
    expect(JSON.parse(json)).toEqual({
      pageSize: 20,
      cards: { imageColumn: 'img', titleColumn: '', detailColumns: ['x'], size: 'large' },
    })
    expect(parseCardsConfig(json).size).toBe('large')
  })
})

describe('suggestedCardsConfig', () => {
  it('picks the first image column and a name column', () => {
    const config = suggestedCardsConfig([
      { name: 'id', type: 'int' },
      { name: 'title', type: 'varchar' },
      { name: 'name', type: 'varchar' },
      { name: 'main_picture', type: 'image' },
    ])
    expect(config.imageColumn).toBe('main_picture')
    expect(config.titleColumn).toBe('name')
  })
})

describe('filterByText', () => {
  const rows = ['Lasagne', 'Chicken Tikka Masala', 'Crème brûlée']

  it('matches every word, ignoring case, order and accents', () => {
    expect(filterByText(rows, '', r => [r])).toEqual(rows)
    expect(filterByText(rows, 'masala chick', r => [r])).toEqual(['Chicken Tikka Masala'])
    expect(filterByText(rows, 'creme', r => [r])).toEqual(['Crème brûlée'])
  })

  it('matches words across the title and details', () => {
    const cards = [
      { title: 'Lasagne', details: ['Dinner', 'Italian'] },
      { title: 'Porridge', details: ['Breakfast'] },
      { title: 'Dinner rolls', details: ['Side'] },
    ]
    const textsOf = (card: (typeof cards)[number]) => [card.title, ...card.details]
    expect(filterByText(cards, 'dinner', textsOf).map(c => c.title)).toEqual(['Lasagne', 'Dinner rolls'])
    expect(filterByText(cards, 'italian lasa', textsOf).map(c => c.title)).toEqual(['Lasagne'])
    expect(filterByText(cards, 'breakfast lasagne', textsOf)).toEqual([])
  })

  it('does not match a word split across two texts', () => {
    expect(filterByText([['Fish', 'tacos']], 'fishtacos', r => r)).toEqual([])
  })
})
