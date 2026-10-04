import { describe, expect, it } from 'vitest'

import { columnFieldKind, feedFieldKind, mappingMismatch } from './feedFieldMapping'

describe('feedFieldMapping', () => {
  it('treats calendar instants as datetime and names as string', () => {
    expect(feedFieldKind('startDate')).toBe('datetime')
    expect(feedFieldKind('name')).toBe('string')
    expect(feedFieldKind('title')).toBe('string')
  })

  it('classifies table column types', () => {
    expect(columnFieldKind('datetime')).toBe('datetime')
    expect(columnFieldKind('varchar(255)')).toBe('string')
    expect(columnFieldKind('bigint')).toBe('integer')
  })

  it('allows startDate onto a datetime column with any name', () => {
    expect(mappingMismatch('startDate', 'datetime')).toBeNull()
  })

  it('rejects a string feed field dropped on a datetime column', () => {
    expect(mappingMismatch('name', 'datetime')).toBe('name requires a string field')
  })
})
