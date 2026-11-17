import { describe, expect, it } from 'vitest'

import { notifyResultText } from './notifyUsers'

describe('notifyResultText', () => {
  it('describes each outcome', () => {
    expect(notifyResultText({ username: 'bob', status: 'sent', deliveredChannels: 1 })).toBe('Sent to bob.')
    expect(notifyResultText({ username: 'bob', status: 'sent', deliveredChannels: 2 })).toBe('Sent to bob (2 channels).')
    expect(notifyResultText({ username: 'bob', status: 'not_subscribed', deliveredChannels: 0 }))
      .toBe('Not sent to bob: they have not chosen a channel for "Notified by another user".')
    expect(notifyResultText({ username: 'bob', status: 'failed', deliveredChannels: 0 }))
      .toBe('Could not reach bob: their notification channels failed.')
  })
})
