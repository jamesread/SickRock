import { describe, expect, it } from 'vitest'

import { formatDocumentTitle, syncAppTitleMeta } from './appTitle'

/** Just enough of a Document for syncAppTitleMeta. */
function fakeDocument(withMeta: boolean) {
  const metas: Array<{ name: string; content: string }> = []
  if (withMeta) metas.push({ name: 'apple-mobile-web-app-title', content: 'SickRock' })
  const doc = {
    querySelector: (selector: string) => (selector === 'meta[name="apple-mobile-web-app-title"]' ? metas[0] ?? null : null),
    createElement: () => ({ name: '', content: '' }),
    head: { appendChild: (el: { name: string; content: string }) => metas.push(el) },
  }
  return { doc: doc as unknown as Document, metas }
}

describe('syncAppTitleMeta', () => {
  it('updates the existing iOS title tag', () => {
    const { doc, metas } = fakeDocument(true)
    syncAppTitleMeta('  Acme Stock ', doc)
    expect(metas).toEqual([{ name: 'apple-mobile-web-app-title', content: 'Acme Stock' }])
  })

  it('adds the tag when the page has none', () => {
    const { doc, metas } = fakeDocument(false)
    syncAppTitleMeta('Acme', doc)
    expect(metas).toEqual([{ name: 'apple-mobile-web-app-title', content: 'Acme' }])
  })

  it('falls back to SickRock for a blank title', () => {
    const { doc, metas } = fakeDocument(true)
    syncAppTitleMeta('   ', doc)
    expect(metas[0].content).toBe('SickRock')
  })
})

describe('formatDocumentTitle', () => {
  it('joins the page and app titles', () => {
    expect(formatDocumentTitle('Jobs', 'Acme')).toBe('Jobs - Acme')
    expect(formatDocumentTitle('  ', 'Acme')).toBe('Acme')
    expect(formatDocumentTitle('Jobs', ' ')).toBe('Jobs - SickRock')
  })
})
