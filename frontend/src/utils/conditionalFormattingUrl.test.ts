import { describe, expect, it } from 'vitest'
import {
  parseUrlForConditionalFormat,
  urlConditionalFormatLink,
} from './conditionalFormattingUrl'

describe('parseUrlForConditionalFormat', () => {
  it('strips https scheme from the display label', () => {
    expect(parseUrlForConditionalFormat('https://example.com/path')).toEqual({
      href: 'https://example.com/path',
      label: 'example.com/path',
    })
  })

  it('adds https when the value has no scheme', () => {
    expect(parseUrlForConditionalFormat('example.com')).toEqual({
      href: 'https://example.com/',
      label: 'example.com',
    })
  })
})

describe('urlConditionalFormatLink', () => {
  it('returns a link when an active URL rule applies', () => {
    const link = urlConditionalFormatLink('website', 'https://acme.test', [{
      columnName: 'website',
      conditionType: 'always',
      conditionValue: '',
      formatType: 'url',
      priority: 0,
      isActive: true,
    }])
    expect(link?.label).toBe('acme.test')
    expect(link?.href).toBe('https://acme.test/')
  })
})
