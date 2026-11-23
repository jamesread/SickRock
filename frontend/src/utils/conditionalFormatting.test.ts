import { describe, expect, it } from 'vitest'

import { conditionalStyles, ruleMatches, type FormattingRule } from './conditionalFormatting'

function rule(overrides: Partial<FormattingRule>): FormattingRule {
  return {
    columnName: 'note',
    conditionType: 'equals',
    conditionValue: '',
    formatType: 'text_color',
    formatValue: 'blue',
    priority: 0,
    isActive: true,
    ...overrides,
  }
}

describe('ruleMatches', () => {
  it('checks each condition', () => {
    expect(ruleMatches(rule({ conditionType: 'always' }), null)).toBe(true)
    expect(ruleMatches(rule({ conditionValue: 'x' }), 'x')).toBe(true)
    expect(ruleMatches(rule({ conditionValue: 'x' }), 'X')).toBe(false)
    expect(ruleMatches(rule({ conditionType: 'contains', conditionValue: 'MES' }), 'James: early')).toBe(true)
    expect(ruleMatches(rule({ conditionType: 'greater_than', conditionValue: '5' }), '7')).toBe(true)
    expect(ruleMatches(rule({ conditionType: 'less_than', conditionValue: '5' }), 7)).toBe(false)
    expect(ruleMatches(rule({ conditionType: 'unknown' }), 'x')).toBe(false)
  })

  it('matches a prefix, ignoring case and leading spaces', () => {
    const james = rule({ conditionType: 'starts_with', conditionValue: 'James:' })
    expect(ruleMatches(james, 'James: cooks')).toBe(true)
    expect(ruleMatches(james, '  james: cooks')).toBe(true)
    expect(ruleMatches(james, 'Ask James: maybe')).toBe(false)
    expect(ruleMatches(james, undefined)).toBe(false)
  })
})

describe('conditionalStyles', () => {
  const values: Record<string, string> = { note: 'James: cooks', servings: '6' }
  const valueOf = (column: string) => values[column]

  it("styles from every matching rule of the row's columns", () => {
    const rules = [
      rule({ conditionType: 'starts_with', conditionValue: 'James:' }),
      rule({ columnName: 'servings', conditionType: 'greater_than', conditionValue: '4', formatType: 'bold', formatValue: 'true' }),
      rule({ conditionType: 'starts_with', conditionValue: 'Sam:', formatType: 'color', formatValue: 'pink' }),
      rule({ conditionType: 'always', formatType: 'italic', formatValue: 'true', isActive: false }),
      rule({ conditionType: 'always', formatType: 'markdown', formatValue: '**x**' }),
    ]
    expect(conditionalStyles(rules, valueOf)).toEqual({ color: 'blue', 'font-weight': 'bold' })
  })

  it("keeps to one column's rules when asked", () => {
    const rules = [rule({ columnName: 'servings', conditionType: 'always', formatType: 'color', formatValue: 'red' })]
    expect(conditionalStyles(rules, valueOf, 'note')).toEqual({})
    expect(conditionalStyles(rules, valueOf, 'servings')).toEqual({ 'background-color': 'red' })
  })

  it('lets the higher priority win', () => {
    const rules = [
      rule({ conditionType: 'always', formatValue: 'green', priority: 5 }),
      rule({ conditionType: 'always', formatValue: 'blue', priority: 1 }),
    ]
    expect(conditionalStyles(rules, valueOf)).toEqual({ color: 'green' })
  })
})
