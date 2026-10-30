import { describe, expect, it } from 'vitest'

import {
  cellKey,
  daysOfWeek,
  dayKey,
  filterPickerOptions,
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
