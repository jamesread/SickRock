import { describe, expect, it } from 'vitest'
import { tableCacheFiltersMatch } from './indexedDB'

describe('offline table cache filters', () => {
  it('matches equivalent filters regardless of key order', () => {
    expect(tableCacheFiltersMatch(
      { status: 'open', owner: '%Ada%' },
      { owner: '%Ada%', status: 'open' },
    )).toBe(true)
  })

  it('does not use a filtered cache as a complete table snapshot', () => {
    expect(tableCacheFiltersMatch({ status: 'open' }, {})).toBe(false)
    expect(tableCacheFiltersMatch({ status: 'open' }, undefined)).toBe(false)
  })

  it('treats missing and empty filters as the same unfiltered request', () => {
    expect(tableCacheFiltersMatch(undefined, {})).toBe(true)
    expect(tableCacheFiltersMatch({}, undefined)).toBe(true)
  })
})
