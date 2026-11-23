// Conditional formatting rules (per table, managed in ConditionalFormattingRules.vue): when a
// column's value meets a condition, a cell, calendar event or weekly plan entry is styled. The
// server applies the same conditions to markdown rules (service/internal/server/item_formatting.go).

export type FormattingRule = {
  columnName: string
  conditionType: string
  conditionValue: string
  formatType: string
  formatValue?: string
  priority: number
  isActive: boolean
}

/** The conditions a rule can have, in the order the rule editor offers them. */
export const CONDITION_TYPES: ReadonlyArray<{ value: string; label: string }> = [
  { value: 'always', label: 'Always' },
  { value: 'equals', label: 'Equals' },
  { value: 'contains', label: 'Contains' },
  { value: 'starts_with', label: 'Starts with' },
  { value: 'greater_than', label: 'Greater Than' },
  { value: 'less_than', label: 'Less Than' },
]

/** Whether value meets the rule's condition. Contains and starts with ignore case. */
export function ruleMatches(rule: Pick<FormattingRule, 'conditionType' | 'conditionValue'>, value: unknown): boolean {
  const content = value == null ? '' : String(value)
  const wanted = rule.conditionValue ?? ''
  switch (rule.conditionType) {
    case 'always':
      return true
    case 'equals':
      return content === wanted
    case 'contains':
      return content.toLowerCase().includes(wanted.toLowerCase())
    case 'starts_with':
      return content.trimStart().toLowerCase().startsWith(wanted.toLowerCase())
    case 'greater_than':
      return Number(content) > Number(wanted)
    case 'less_than':
      return Number(content) < Number(wanted)
    default:
      return false
  }
}

const STYLE_FORMATS = new Set(['color', 'text_color', 'bold', 'italic'])

/**
 * The CSS of the active style rules (background colour, text colour, bold, italic) that match.
 * valueOf reads a rule's column. With column set, only that column's rules apply (a table cell);
 * without it, every rule of the table does (a whole calendar event or weekly plan entry). Where two
 * matching rules set the same thing, the higher priority wins.
 */
export function conditionalStyles(
  rules: FormattingRule[],
  valueOf: (column: string) => unknown,
  column?: string,
): Record<string, string> {
  const styles: Record<string, string> = {}
  const applicable = rules
    .filter(rule => rule.isActive && STYLE_FORMATS.has(rule.formatType) && (column === undefined || rule.columnName === column))
    .sort((left, right) => left.priority - right.priority)
  for (const rule of applicable) {
    if (!ruleMatches(rule, valueOf(rule.columnName))) continue
    const value = rule.formatValue ?? ''
    if (rule.formatType === 'color') styles['background-color'] = value
    else if (rule.formatType === 'text_color') styles.color = value
    else if (rule.formatType === 'bold' && value === 'true') styles['font-weight'] = 'bold'
    else if (rule.formatType === 'italic' && value === 'true') styles['font-style'] = 'italic'
  }
  return styles
}
