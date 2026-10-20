import { describe, expect, it } from 'vitest'
import { orderBySchema } from './schemaFieldOrder'

describe('orderBySchema', () => {
  it('follows the schema column order', () => {
    expect(orderBySchema(['city', 'email', 'name'], ['id', 'name', 'email', 'city'])).toEqual(['name', 'email', 'city'])
  })

  it('keeps names missing from the schema after known ones, in their original order', () => {
    expect(orderBySchema(['zeta', 'name', 'alpha'], ['name'])).toEqual(['name', 'zeta', 'alpha'])
  })
})
