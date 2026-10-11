export type ConditionalFormatUrlLink = {
  href: string
  label: string
}

/** Build href + display label (scheme omitted) for URL conditional formatting. */
export function parseUrlForConditionalFormat(raw: string): ConditionalFormatUrlLink | null {
  const trimmed = raw.trim()
  if (!trimmed) {
    return null
  }
  let href = trimmed
  if (/^\/\//.test(href)) {
    href = `https:${href}`
  } else if (!/^https?:\/\//i.test(href)) {
    href = `https://${href}`
  }
  try {
    const url = new URL(href)
    if (!url.hostname) {
      return null
    }
    const label = trimmed.replace(/^https?:\/\//i, '')
    return {
      href: url.href,
      label: label || url.host,
    }
  } catch {
    return null
  }
}

export type ConditionalFormattingRuleLike = {
  columnName: string
  conditionType: string
  conditionValue: string
  formatType: string
  formatValue?: string
  priority: number
  isActive: boolean
}

function ruleApplies(rule: ConditionalFormattingRuleLike, content: string): boolean {
  switch (rule.conditionType) {
    case 'always':
      return true
    case 'equals':
      return content === rule.conditionValue
    case 'contains':
      return content.toLowerCase().includes(rule.conditionValue.toLowerCase())
    case 'greater_than':
      return Number(content) > Number(rule.conditionValue)
    case 'less_than':
      return Number(content) < Number(rule.conditionValue)
    default:
      return false
  }
}

/** Highest-priority active URL format rule for a cell value, if any. */
export function urlConditionalFormatLink(
  columnName: string,
  cellValue: unknown,
  rules: ConditionalFormattingRuleLike[],
): ConditionalFormatUrlLink | null {
  const content = cellValue == null ? '' : String(cellValue)
  if (!content) {
    return null
  }
  const columnRules = rules
    .filter(rule =>
      rule.columnName === columnName
      && rule.isActive
      && rule.formatType === 'url',
    )
    .sort((left, right) => right.priority - left.priority)
  for (const rule of columnRules) {
    if (!ruleApplies(rule, content)) {
      continue
    }
    return parseUrlForConditionalFormat(content)
  }
  return null
}
