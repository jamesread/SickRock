// Shared by the Notify dialog: limits matching the server's NotifyUsers checks, and result text.
import type { NotifyUserResult } from '../gen/sickrock_pb'

export const NOTIFY_MAX_TITLE_LENGTH = 200
export const NOTIFY_MAX_MESSAGE_LENGTH = 2000

/** What happened for one recipient, e.g. "Sent to bob." */
export function notifyResultText(result: Pick<NotifyUserResult, 'username' | 'status' | 'deliveredChannels'>): string {
  switch (result.status) {
    case 'sent':
      return result.deliveredChannels > 1
        ? `Sent to ${result.username} (${result.deliveredChannels} channels).`
        : `Sent to ${result.username}.`
    case 'not_subscribed':
      return `Not sent to ${result.username}: they have not chosen a channel for "Notified by another user".`
    default:
      return `Could not reach ${result.username}: their notification channels failed.`
  }
}
