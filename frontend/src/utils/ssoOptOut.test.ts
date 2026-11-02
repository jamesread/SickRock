import { describe, expect, it } from 'vitest'

import { clearSSOOptOutCookie, hasSSOOptOut, SSO_OPT_OUT_COOKIE } from './ssoOptOut'

describe('ssoOptOut', () => {
  it('detects the opt-out cookie among others', () => {
    expect(hasSSOOptOut(`foo=bar; ${SSO_OPT_OUT_COOKIE}=1; baz=qux`)).toBe(true)
    expect(hasSSOOptOut(`${SSO_OPT_OUT_COOKIE}=1`)).toBe(true)
  })

  it('ignores a missing, empty or similarly named cookie', () => {
    expect(hasSSOOptOut('')).toBe(false)
    expect(hasSSOOptOut('foo=bar')).toBe(false)
    expect(hasSSOOptOut(`${SSO_OPT_OUT_COOKIE}=`)).toBe(false)
    expect(hasSSOOptOut(`x${SSO_OPT_OUT_COOKIE}=1`)).toBe(false)
  })

  it('builds a cookie string that expires the opt-out', () => {
    expect(clearSSOOptOutCookie(false)).toBe(`${SSO_OPT_OUT_COOKIE}=; Path=/; Max-Age=0; SameSite=Lax`)
    expect(clearSSOOptOutCookie(true)).toContain('; Secure')
  })
})
