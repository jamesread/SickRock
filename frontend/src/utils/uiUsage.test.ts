import { describe, expect, it } from 'vitest'

import {
  appendCapped,
  controlLabel,
  describeClick,
  linkPath,
  newSessionId,
  parsePending,
  pendingStorageKey,
  type UiUsageEvent,
} from './uiUsage'

const ORIGIN = 'https://sickrock.example'

/** A fake element: its own attributes and text, and the selectors its ancestors match. */
function fakeElement(opts: { attrs?: Record<string, string>, text?: string, clickable?: boolean, within?: string[] }) {
  const el = {
    textContent: opts.text ?? '',
    getAttribute: (name: string) => opts.attrs?.[name] ?? null,
    closest: (selector: string) => {
      if (selector.includes('button') || selector.includes('a[href]')) return opts.clickable === false ? null : el
      return (opts.within ?? []).includes(selector) ? {} : null
    },
  }
  return el as unknown as Element
}

function event(i: number): UiUsageEvent {
  return { kind: 'route', fromPath: '', toPath: `/p/${i}`, label: '', timestampMs: i, sessionId: 's', appVersion: 'v' }
}

describe('describeClick', () => {
  it('classifies sidebar and header controls', () => {
    const sidebarLink = fakeElement({ attrs: { href: '/table/pantry' }, text: ' Pantry ', within: ['#picocrank-sidebar'] })
    expect(describeClick(sidebarLink, ORIGIN)).toEqual({ kind: 'sidebar', toPath: '/table/pantry', label: 'Pantry' })

    const headerButton = fakeElement({ attrs: { 'aria-label': 'Bookmarks' }, within: ['header'] })
    expect(describeClick(headerButton, ORIGIN)).toEqual({ kind: 'header', toPath: '', label: 'Bookmarks' })
  })

  it('ignores clicks in page content, on non-controls and on non-elements', () => {
    expect(describeClick(fakeElement({ attrs: { href: '/x' } }), ORIGIN)).toBeNull()
    expect(describeClick(fakeElement({ clickable: false, within: ['header'] }), ORIGIN)).toBeNull()
    expect(describeClick(null, ORIGIN)).toBeNull()
    expect(describeClick({} as EventTarget, ORIGIN)).toBeNull()
  })
})

describe('linkPath', () => {
  it('keeps same-site links as paths and others in full', () => {
    expect(linkPath(fakeElement({ attrs: { href: '/table/x?view=2#top' } }), ORIGIN)).toBe('/table/x?view=2#top')
    expect(linkPath(fakeElement({ attrs: { href: `${ORIGIN}/a` } }), ORIGIN)).toBe('/a')
    expect(linkPath(fakeElement({ attrs: { href: 'https://other.example/b' } }), ORIGIN)).toBe('https://other.example/b')
    expect(linkPath(fakeElement({}), ORIGIN)).toBe('')
  })
})

describe('controlLabel', () => {
  it('prefers aria-label, then title, then text, collapsed and shortened', () => {
    expect(controlLabel(fakeElement({ attrs: { 'aria-label': 'A', title: 'B' }, text: 'C' }))).toBe('A')
    expect(controlLabel(fakeElement({ attrs: { title: 'B' }, text: 'C' }))).toBe('B')
    expect(controlLabel(fakeElement({ text: '  Line\n  two  ' }))).toBe('Line two')
    expect(controlLabel(fakeElement({ text: 'x'.repeat(300) }))).toHaveLength(120)
  })
})

describe('pending events', () => {
  it('drop the oldest beyond the cap', () => {
    const out = appendCapped([event(1), event(2)], [event(3), event(4)], 3)
    expect(out.map(e => e.timestampMs)).toEqual([2, 3, 4])
  })

  it('parse stored events, ignoring anything malformed', () => {
    expect(parsePending(null)).toEqual([])
    expect(parsePending('not json')).toEqual([])
    expect(parsePending('{"a":1}')).toEqual([])
    expect(parsePending(JSON.stringify([event(1), { kind: 'scroll', timestampMs: 1 }, null]))).toEqual([event(1)])
  })

  it('are stored per user', () => {
    expect(pendingStorageKey('alice')).not.toBe(pendingStorageKey('bob'))
  })
})

describe('newSessionId', () => {
  it('is a short random hex id', () => {
    const a = newSessionId()
    expect(a).toMatch(/^[0-9a-f]{12}$/)
    expect(newSessionId()).not.toBe(a)
  })
})
