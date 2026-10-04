export type FieldKind = 'string' | 'datetime' | 'integer' | 'boolean'

const DATETIME_FEED_FIELDS = new Set([
  'startdate',
  'enddate',
  'pubdate',
  'updated',
])

export function feedFieldKind(fieldName: string): FieldKind {
  if (DATETIME_FEED_FIELDS.has(fieldName.trim().toLowerCase())) {
    return 'datetime'
  }
  return 'string'
}

export function columnFieldKind(columnType: string): FieldKind {
  const type = columnType.trim().toLowerCase()
  if (
    type === 'date'
    || type === 'time'
    || type === 'datetime'
    || type.startsWith('datetime(')
    || type === 'timestamp'
    || type.startsWith('timestamp(')
  ) {
    return 'datetime'
  }
  if (type === 'bool' || type === 'boolean' || type.startsWith('tinyint')) {
    return 'boolean'
  }
  if (
    type === 'user_ref'
    || /(int|integer|bigint|serial|decimal|numeric|float|double|real)/.test(type)
  ) {
    return 'integer'
  }
  return 'string'
}

/** Null when the feed field can be stored in the column. */
export function mappingMismatch(feedField: string, columnType: string): string | null {
  const required = feedFieldKind(feedField)
  if (required === columnFieldKind(columnType)) {
    return null
  }
  return `${feedField} requires a ${required} field`
}
