import { describe, expect, it } from 'vitest'

import { fetchManifest } from './pwaDiagnostics'

function responding(body: string, contentType: string, status = 200) {
  return (async () => new Response(body, { status, headers: { 'Content-Type': contentType } })) as unknown as typeof fetch
}

describe('fetchManifest', () => {
  it('accepts a manifest', async () => {
    const result = await fetchManifest('/manifest.json', responding('{"name":"SickRock"}', 'application/manifest+json'))
    expect(result).toEqual({ data: { name: 'SickRock' }, error: '' })
  })

  it('explains an HTML page returned in place of the manifest', async () => {
    for (const type of ['text/html; charset=utf-8', 'application/json']) {
      const result = await fetchManifest('/manifest.json', responding('<!doctype html><html></html>', type))
      expect(result.data).toBeNull()
      expect(result.error).toMatch(/HTML page instead of the manifest/)
      expect(result.error).not.toMatch(/Unexpected token/)
    }
  })

  it('reports HTTP errors, network failures and bad JSON', async () => {
    expect((await fetchManifest('/manifest.json', responding('nope', 'text/plain', 404))).error).toMatch(/HTTP 404/)
    const offline = (async () => { throw new TypeError('Failed to fetch') }) as unknown as typeof fetch
    expect((await fetchManifest('/manifest.json', offline)).error).toMatch(/Could not fetch.*Failed to fetch/)
    expect((await fetchManifest('/manifest.json', responding('{nope', 'application/manifest+json'))).error).toMatch(/not valid JSON/)
    expect((await fetchManifest('/manifest.json', responding('[1]', 'application/json'))).error).toMatch(/not a JSON object/)
  })

  it('flags a manifest served with the wrong type but still reads it', async () => {
    const result = await fetchManifest('/manifest.json', responding('{"name":"x"}', 'text/plain'))
    expect(result.data).toEqual({ name: 'x' })
    expect(result.error).toMatch(/expected application\/manifest\+json/)
  })
})
