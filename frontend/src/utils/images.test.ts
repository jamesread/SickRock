import { describe, expect, it } from 'vitest'

import { formatBytes, imagePath, pastedImageFilename, pickClipboardImageType, scaleToFit } from './images'
import { hasImageValue, isImageFieldType } from './fieldTypes'

describe('images', () => {
  it('builds the image path', () => {
    expect(imagePath(12)).toBe('/api/images/12')
  })

  it('scales frames down to the longest side, never up', () => {
    expect(scaleToFit(4000, 3000, 2000)).toEqual({ width: 2000, height: 1500 })
    expect(scaleToFit(1080, 1920, 960)).toEqual({ width: 540, height: 960 })
    expect(scaleToFit(640, 480, 2000)).toEqual({ width: 640, height: 480 })
    expect(scaleToFit(0, 0, 2000)).toEqual({ width: 0, height: 0 })
  })

  it('formats byte sizes', () => {
    expect(formatBytes(500)).toBe('500 B')
    expect(formatBytes(2048)).toBe('2 KB')
    expect(formatBytes(2.5 * 1024 * 1024)).toBe('2.5 MB')
  })

  it('detects image columns and stored values', () => {
    expect(isImageFieldType('image')).toBe(true)
    expect(isImageFieldType('IMAGE')).toBe(true)
    expect(isImageFieldType('BIGINT')).toBe(false)
    expect(isImageFieldType(undefined)).toBe(false)
    expect(hasImageValue('5')).toBe(true)
    expect(hasImageValue(5)).toBe(true)
    for (const empty of [null, undefined, '', '0', 0, 'null']) {
      expect(hasImageValue(empty)).toBe(false)
    }
  })
})

describe('pickClipboardImageType', () => {
  it('prefers PNG among accepted types', () => {
    expect(pickClipboardImageType(['text/html', 'image/jpeg', 'image/png'])).toBe('image/png')
  })

  it('ignores types the server refuses', () => {
    expect(pickClipboardImageType(['text/plain', 'image/svg+xml'])).toBe('')
  })
})

describe('pastedImageFilename', () => {
  it('names the file by time and type', () => {
    expect(pastedImageFilename('image/jpeg', new Date('2026-10-30T07:52:18Z'))).toBe('pasted-2026-10-30T07-52-18-000Z.jpg')
  })
})
