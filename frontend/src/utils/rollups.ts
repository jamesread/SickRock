// Rollups (system table table_rollups) group and sum the detail rows owned by a table's rows, for
// example the ingredients of the recipes planned in a week, and replace a target table's rows with
// the result. See RunRollup in proto/sickrock.proto.
import { addDays, dayKey } from './weeklyPlan'

export type RollupRunRequest = {
  rollupId: bigint
  dateColumn: string
  from: string
  to: string
  where: Record<string, string>
  sourceIds: string[]
  limitToSourceIds: boolean
}

function baseRequest(rollupId: bigint): RollupRunRequest {
  return { rollupId, dateColumn: '', from: '', to: '', where: {}, sourceIds: [], limitToSourceIds: false }
}

/** A run over every source row, or only the listed ones (e.g. the rows a table's filters leave). */
export function allRowsRollupRequest(rollupId: bigint, sourceIds?: string[]): RollupRunRequest {
  const request = baseRequest(rollupId)
  if (sourceIds) {
    request.sourceIds = sourceIds
    request.limitToSourceIds = true
  }
  return request
}

/** A run over the source rows dated in the week starting weekStart (local days, end exclusive). */
export function weekRollupRequest(rollupId: bigint, dateColumn: string, weekStart: Date): RollupRunRequest {
  return { ...baseRequest(rollupId), dateColumn, from: dayKey(weekStart), to: dayKey(addDays(weekStart, 7)) }
}

/**
 * The source ids for a run from a table, from its Table's filtered-rows-change event: the ids of the
 * rows the filters leave, or undefined when no filter is active (every row counts).
 */
export function rollupSourceIdsFrom(
  rows: Array<{ id?: unknown }>,
  { filtersActive }: { filtersActive: boolean },
): string[] | undefined {
  return filtersActive ? rows.map(row => String(row.id)) : undefined
}

/** One line saying what a finished run did. */
export function rollupResultText(result: { rowsWritten: bigint; sourceRows: bigint }): string {
  const rows = Number(result.rowsWritten)
  const sources = Number(result.sourceRows)
  return `Wrote ${rows} ${rows === 1 ? 'row' : 'rows'} from ${sources} source ${sources === 1 ? 'row' : 'rows'}.`
}
