import { describe, expect, it } from 'vitest'

import {
  cellKey,
  copiedFields,
  askedFields,
  copyWeek,
  recentColumnValues,
  entryDetails,
  plannedWeeks,
  daysOfWeek,
  dayKey,
  filterPickerOptions,
  notifyLinkPath,
  notifyTitle,
  weekFromQuery,
  defaultWeeklyPlanConfig,
  groupIntoCells,
  parseWallClock,
  parseWeeklyPlanConfig,
  placementFields,
  slotIndexFor,
  startOfWeek,
  weeklyPlanConfigProblems,
  withWeeklyPlanConfig,
  type WeeklyPlanConfig,
} from './weeklyPlan'
import type { Item } from '../gen/sickrock_pb'

function mealConfig(overrides: Partial<WeeklyPlanConfig> = {}): WeeklyPlanConfig {
  return { ...defaultWeeklyPlanConfig(), dateColumn: 'datetime', itemColumn: 'recipe', ...overrides }
}

describe('weeklyPlan config', () => {
  it('defaults to three meal slots starting Monday', () => {
    const config = parseWeeklyPlanConfig('')
    expect(config.slots.map(s => s.label)).toEqual(['Breakfast', 'Lunch', 'Dinner'])
    expect(config.weekStartsOn).toBe(1)
  })

  it('round-trips through layout_json, keeping other keys', () => {
    const layout = JSON.stringify({ filters: [{ column: 'x' }], weeklyPlan: { dateColumn: 'old' } })
    const saved = withWeeklyPlanConfig(layout, mealConfig({ slots: [{ label: ' Brunch ', time: '10:00' }, { label: '', time: '' }], weekStartsOn: 0 }))
    const parsed = JSON.parse(saved)
    expect(parsed.filters).toEqual([{ column: 'x' }])
    expect(parseWeeklyPlanConfig(saved)).toEqual(mealConfig({ slots: [{ label: 'Brunch', time: '10:00' }], weekStartsOn: 0 }))
  })

  it('ignores invalid stored values', () => {
    const config = parseWeeklyPlanConfig('{"weeklyPlan":{"slots":[{"label":"A","time":"25:00"}],"weekStartsOn":9}}')
    expect(config.slots).toEqual([{ label: 'A', time: '' }])
    expect(config.weekStartsOn).toBe(1)
    expect(parseWeeklyPlanConfig('not json').slots).toHaveLength(3)
  })

  it('keeps slot keywords, tidied, and leaves them out when blank', () => {
    const saved = withWeeklyPlanConfig('', mealConfig({
      slots: [{ label: 'Breakfast', time: '08:00', keywords: '  breakfast   cereal ' }, { label: 'Lunch', time: '12:30', keywords: ' ' }],
    }))
    expect(JSON.parse(saved).weeklyPlan.slots).toEqual([
      { label: 'Breakfast', time: '08:00', keywords: 'breakfast cereal' },
      { label: 'Lunch', time: '12:30' },
    ])
    expect(parseWeeklyPlanConfig(saved).slots[0].keywords).toBe('breakfast cereal')
  })

  it('reports missing settings', () => {
    expect(weeklyPlanConfigProblems(mealConfig())).toEqual([])
    expect(weeklyPlanConfigProblems(defaultWeeklyPlanConfig())).toHaveLength(2)
    expect(weeklyPlanConfigProblems(mealConfig({ slots: [{ label: 'Any', time: '' }] }))).toHaveLength(1)
    expect(weeklyPlanConfigProblems(mealConfig({ slotColumn: 'slot', slots: [{ label: 'Any', time: '' }] }))).toEqual([])
  })
})

describe('weeklyPlan dates', () => {
  it('parses stored datetimes as wall-clock time', () => {
    expect(parseWallClock('2026-11-02 18:30:00')).toEqual(new Date(2026, 10, 2, 18, 30, 0))
    expect(parseWallClock('2026-11-02T08:00:00Z')).toEqual(new Date(2026, 10, 2, 8, 0, 0))
    expect(parseWallClock('2026-11-02')).toEqual(new Date(2026, 10, 2))
    expect(parseWallClock('')).toBeNull()
    expect(parseWallClock(null)).toBeNull()
  })

  it('finds the start of the week', () => {
    const thursday = new Date(2026, 9, 29, 15, 0) // Thu 29 Oct 2026
    expect(dayKey(startOfWeek(thursday, 1))).toBe('2026-10-26')
    expect(dayKey(startOfWeek(thursday, 0))).toBe('2026-10-25')
    expect(dayKey(startOfWeek(new Date(2026, 9, 26), 1))).toBe('2026-10-26')
    expect(daysOfWeek(new Date(2026, 9, 26)).map(dayKey)).toEqual([
      '2026-10-26', '2026-10-27', '2026-10-28', '2026-10-29', '2026-10-30', '2026-10-31', '2026-11-01',
    ])
  })
})

describe('weeklyPlan slots', () => {
  it('places rows by time of day without a slot column', () => {
    const config = mealConfig()
    const at = (h: number, m = 0) => new Date(2026, 9, 26, h, m)
    expect(slotIndexFor(config, at(8), undefined)).toBe(0)
    expect(slotIndexFor(config, at(6), undefined)).toBe(0) // before the first slot
    expect(slotIndexFor(config, at(12, 30), undefined)).toBe(1)
    expect(slotIndexFor(config, at(15), undefined)).toBe(1)
    expect(slotIndexFor(config, at(23), undefined)).toBe(2)
  })

  it('places rows by label with a slot column', () => {
    const config = mealConfig({ slotColumn: 'slot' })
    const when = new Date(2026, 9, 26, 8)
    expect(slotIndexFor(config, when, 'dinner')).toBe(2)
    expect(slotIndexFor(config, when, 'Supper')).toBe(-1)
  })

  it('builds the fields for a placement', () => {
    expect(placementFields(mealConfig(), new Date(2026, 9, 27), 2)).toEqual({ datetime: '2026-10-27 18:00:00' })
    expect(placementFields(mealConfig({ slotColumn: 'slot' }), new Date(2026, 9, 27), 1)).toEqual({
      datetime: '2026-10-27 12:30:00',
      slot: 'Lunch',
    })
  })

  it('groups a week of rows into cells', () => {
    const config = mealConfig()
    const rows = [
      { id: 1, datetime: '2026-10-26 08:00:00' },
      { id: 2, datetime: '2026-10-26 18:00:00' },
      { id: 3, datetime: '2026-10-26 18:00:00' },
      { id: 4, datetime: '2026-11-02 08:00:00' }, // next week
      { id: 5, datetime: '' },
    ]
    const weekStart = new Date(2026, 9, 26)
    const cells = groupIntoCells(config, weekStart, rows, (row, column) => (row as Record<string, unknown>)[column])
    expect(cells.get(cellKey(weekStart, 0))?.map(r => r.id)).toEqual([1])
    expect(cells.get(cellKey(weekStart, 2))?.map(r => r.id)).toEqual([2, 3])
    expect(cells.size).toBe(2)
  })
})

describe('filterPickerOptions', () => {
  const options = [
    { label: 'Lasagne' },
    { label: 'Chicken Tikka Masala' },
    { label: 'Crème brûlée' },
  ]

  it('returns everything for an empty query', () => {
    expect(filterPickerOptions(options, '  ')).toEqual(options)
  })

  it('matches every word, ignoring case and order', () => {
    expect(filterPickerOptions(options, 'masala chick')).toEqual([{ label: 'Chicken Tikka Masala' }])
    expect(filterPickerOptions(options, 'chicken lasagne')).toEqual([])
  })

  it('ignores accents', () => {
    expect(filterPickerOptions(options, 'creme brulee')).toEqual([{ label: 'Crème brûlée' }])
  })

  it('matches the details shown on the tiles too', () => {
    const withDetails = [
      { label: 'Spag Bol', details: [{ value: 'Dinner' }, { value: 'italian beef tomato' }] },
      { label: 'Cornflakes', details: [{ value: 'Breakfast' }] },
    ]
    expect(filterPickerOptions(withDetails, 'italian bol')).toEqual([withDetails[0]])
    expect(filterPickerOptions(withDetails, 'breakfast')).toEqual([withDetails[1]])
  })
})

describe('notifying users about a week', () => {
  it('keeps default recipients as positive whole ids, once each', () => {
    const config = parseWeeklyPlanConfig('{"weeklyPlan":{"notifyUserIds":[3,"4",3,0,-2,1.5,"x",null]}}')
    expect(config.notifyUserIds).toEqual([3, 4])
    expect(parseWeeklyPlanConfig('{"weeklyPlan":{"notifyUserIds":"3"}}').notifyUserIds).toEqual([])
    expect(parseWeeklyPlanConfig('').notifyUserIds).toEqual([])

    const saved = withWeeklyPlanConfig('{"other":1}', mealConfig({ notifyUserIds: [5, 5, 2] }))
    expect(JSON.parse(saved).other).toBe(1)
    expect(parseWeeklyPlanConfig(saved).notifyUserIds).toEqual([5, 2])
  })

  it('reads the week from a link', () => {
    expect(dayKey(weekFromQuery('2026-11-04', 1)!)).toBe('2026-11-02')
    expect(dayKey(weekFromQuery('2026-11-04', 0)!)).toBe('2026-11-01')
    for (const bad of ['2026-02-30', '2026-11-4', 'next week', '', undefined, ['2026-11-04']]) {
      expect(weekFromQuery(bad, 1)).toBeNull()
    }
  })

  it('titles and links the week', () => {
    const week = new Date(2026, 10, 2)
    expect(notifyTitle('Meal plan', week, 'en-GB')).toBe('Meal plan for 2 Nov – 8 Nov is ready')
    expect(notifyTitle('  ', week, 'en-GB')).toBe('Weekly plan for 2 Nov – 8 Nov is ready')
    expect(notifyLinkPath('meal plan', 7, week)).toBe('/table/meal%20plan?view=7&week=2026-11-02')
    expect(notifyLinkPath('meals', undefined, week)).toBe('/table/meals?week=2026-11-02')
    expect(weekFromQuery(new URLSearchParams(notifyLinkPath('meals', 7, week).split('?')[1]).get('week'), 1)).toEqual(week)
  })
})

describe('copying a week', () => {
  const plan = (id: string, fields: Record<string, string>) =>
    ({ id, additionalFields: { sr_created: '2026-11-01 00:00:00', ...fields } }) as unknown as Item
  const columns = ['id', 'sr_created', 'sr_updated', 'datetime', 'recipe', 'note']
  const lastWeek = new Date(2026, 10, 16) // Monday 16 Nov
  const thisWeek = new Date(2026, 10, 23)
  const rows = [
    plan('1', { datetime: '2026-11-16 08:00:00', recipe: '4', note: 'James: early' }),
    plan('2', { datetime: '2026-11-18 18:00:00', recipe: '7', note: '' }),
    plan('3', { datetime: '2026-11-25 18:00:00', recipe: '7' }), // already planned this week
    plan('4', { datetime: '2026-11-10 18:00:00', recipe: '9' }),
  ]

  it('copies every column but the generated ones, leaving out empty values', () => {
    expect(copiedFields(columns, { id: '1', sr_created: 'x', recipe: '4', note: '' })).toEqual({ recipe: '4' })
  })

  it('lists the weeks with entries, newest first', () => {
    expect(plannedWeeks(mealConfig(), rows, (row, column) => row.additionalFields?.[column])
      .map(week => [dayKey(week.start), week.count]))
      .toEqual([['2026-11-23', 1], ['2026-11-16', 2], ['2026-11-09', 1]])
  })

  it('moves each entry to the same weekday and time of the target week', () => {
    const copy = copyWeek(mealConfig(), rows, columns, lastWeek, thisWeek)
    // Wednesday dinner (recipe 7) is already planned this week, so only Monday's breakfast is new.
    expect(copy).toEqual({
      rows: [{ datetime: '2026-11-23 08:00:00', recipe: '4', note: 'James: early' }],
      skipped: 1,
    })
  })

  it('adds nothing when the week is copied twice', () => {
    const copy = copyWeek(mealConfig(), rows, columns, lastWeek, thisWeek)
    const again = copyWeek(mealConfig(), [...rows, ...copy.rows.map((fields, i) => plan(`n${i}`, fields))], columns, lastWeek, thisWeek)
    expect(again).toEqual({ rows: [], skipped: 2 })
  })

  it('copies nothing onto the same week', () => {
    expect(copyWeek(mealConfig(), rows, columns, thisWeek, thisWeek).rows).toEqual([])
  })

  it('works across a change of daylight saving time', () => {
    const october = [plan('1', { datetime: '2026-10-19 18:00:00', recipe: '1' })]
    expect(copyWeek(mealConfig(), october, columns, new Date(2026, 9, 19), new Date(2026, 9, 26)).rows[0].datetime)
      .toBe('2026-10-26 18:00:00')
  })
})

describe('entry details', () => {
  it('reads and saves the detail columns', () => {
    const config = parseWeeklyPlanConfig('{"weeklyPlan":{"detailColumns":[" note ","note",3,"","servings"]}}')
    expect(config.detailColumns).toEqual(['note', 'servings'])
    expect(parseWeeklyPlanConfig(withWeeklyPlanConfig('', config)).detailColumns).toEqual(['note', 'servings'])
    expect(parseWeeklyPlanConfig('{"weeklyPlan":{}}').detailColumns).toEqual([])
  })

  it('shows the details that have a value, in order', () => {
    const values: Record<string, unknown> = { note: ' James: cooking ', servings: 4, extra: '' }
    expect(entryDetails(mealConfig({ detailColumns: ['servings', 'extra', 'note', 'missing'] }), column => values[column]))
      .toEqual(['4', 'James: cooking'])
  })
})

describe('asking for columns when adding', () => {
  const row = (id: string, who: string) => ({ id, additionalFields: { who } }) as unknown as Item

  it('reads and saves the asked columns', () => {
    const config = parseWeeklyPlanConfig('{"weeklyPlan":{"askColumns":["who"," who",""]}}')
    expect(config.askColumns).toEqual(['who'])
    expect(parseWeeklyPlanConfig(withWeeklyPlanConfig('', config)).askColumns).toEqual(['who'])
  })

  it('offers the newest values first, once each, up to the limit', () => {
    const rows = [row('2', 'Evie'), row('10', 'james'), row('9', ' James '), row('3', ''), row('4', 'Sam')]
    expect(recentColumnValues(rows, 'who')).toEqual(['james', 'Sam', 'Evie'])
    expect(recentColumnValues(rows, 'who', 2)).toEqual(['james', 'Sam'])
    expect(recentColumnValues(rows, 'missing')).toEqual([])
  })

  it('stores only the asked columns that were filled in', () => {
    const config = mealConfig({ askColumns: ['who', 'note'] })
    expect(askedFields(config, { who: ' Evie ', note: '  ', other: 'x' })).toEqual({ who: 'Evie' })
  })
})
