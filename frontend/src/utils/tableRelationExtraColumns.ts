import type {
  Field,
  ForeignKey,
  GetTableStructureResponse,
  Item,
  RelationPullThroughField,
} from '../gen/sickrock_pb'
import type { createApiClient } from '../stores/api'
import {
  buildRelationExtraFieldDefs,
  enrichItemsWithPullThrough,
  pullThroughSpecsFromRelationColumns,
  type ReferencedPullThroughContext,
} from './relationPullThrough'

function isSourceForeignKey(tableConfiguration: string, fk: ForeignKey): boolean {
  const source = (fk.tableTcName?.trim() || fk.tableName).trim()
  return source === tableConfiguration.trim()
}

/** Relation pull-through / calculated config for outgoing FKs on this table. */
export function relationColumnsForSourceTable(
  tableConfiguration: string,
  foreignKeys: ForeignKey[],
): RelationPullThroughField[] {
  const merged: RelationPullThroughField[] = []
  const seen = new Set<string>()
  for (const fk of foreignKeys) {
    if (!isSourceForeignKey(tableConfiguration, fk)) {
      continue
    }
    for (const col of fk.pullThroughFields ?? []) {
      const key = col.expr?.trim()
        ? `calc:${col.displayLabel}:${col.expr}`
        : `pt:${col.fkColumn}:${col.field}`
      if (seen.has(key)) {
        continue
      }
      seen.add(key)
      merged.push(col)
    }
  }
  return merged
}

export function tableHasRelationCalculatedColumns(
  tableConfiguration: string,
  foreignKeys: ForeignKey[],
): boolean {
  return relationColumnsForSourceTable(tableConfiguration, foreignKeys)
    .some(col => col.expr?.trim())
}

async function ensureTableStructure(
  client: ReturnType<typeof createApiClient>,
  tableConfiguration: string,
  cache: Record<string, GetTableStructureResponse>,
): Promise<GetTableStructureResponse> {
  if (cache[tableConfiguration]) {
    return cache[tableConfiguration]
  }
  const structure = await client.getTableStructure({ pageId: tableConfiguration })
  cache[tableConfiguration] = structure
  return structure
}

async function buildReferencedContextByFk(
  client: ReturnType<typeof createApiClient>,
  pullSpecs: ReturnType<typeof pullThroughSpecsFromRelationColumns>,
  childStructure: GetTableStructureResponse,
  structureCache: Record<string, GetTableStructureResponse>,
): Promise<Record<string, ReferencedPullThroughContext>> {
  const referencedContextByFk: Record<string, ReferencedPullThroughContext> = {}
  const childForeignKeys = childStructure.foreignKeys ?? []
  for (const spec of pullSpecs) {
    if (referencedContextByFk[spec.fkColumn]) {
      continue
    }
    const childFk = childForeignKeys.find(entry => entry.columnName === spec.fkColumn)
    if (!childFk) {
      continue
    }
    const refTc = childFk.referencedTableTcName || childFk.referencedTable
    const refStructure = await ensureTableStructure(client, refTc, structureCache)
    const refFks = refStructure.foreignKeys ?? []
    const referencedItemsByColumn: Record<string, Item[]> = {}
    for (const refFk of refFks) {
      const targetTc = refFk.referencedTableTcName || refFk.referencedTable
      try {
        const targetRes = await client.listItems({ tcName: targetTc })
        referencedItemsByColumn[refFk.columnName] = targetRes.items ?? []
      } catch {
        referencedItemsByColumn[refFk.columnName] = []
      }
    }
    referencedContextByFk[spec.fkColumn] = {
      fields: refStructure.fields ?? [],
      foreignKeys: refFks,
      referencedItemsByColumn,
    }
  }
  return referencedContextByFk
}

export async function enrichTableItemsWithRelationExtras(
  client: ReturnType<typeof createApiClient>,
  tableConfiguration: string,
  items: Item[],
  structure: GetTableStructureResponse,
): Promise<{ items: Item[]; extraFields: Field[] }> {
  const relationColumns = relationColumnsForSourceTable(
    tableConfiguration,
    structure.foreignKeys ?? [],
  )
  if (!relationColumns.length) {
    return { items, extraFields: [] }
  }

  const pullSpecs = pullThroughSpecsFromRelationColumns(relationColumns)
  const structureCache: Record<string, GetTableStructureResponse> = {
    [tableConfiguration]: structure,
  }
  const referencedContextByFk = await buildReferencedContextByFk(
    client,
    pullSpecs,
    structure,
    structureCache,
  )
  const extraFields = buildRelationExtraFieldDefs(
    relationColumns,
    structure.fields ?? [],
    referencedContextByFk,
    structure.columnDisplaySettings,
  )
  const displayFieldsByStorageKey = Object.fromEntries(
    extraFields.map(field => [field.name, field]),
  )
  let out = items
  if (pullSpecs.length > 0) {
    out = await enrichItemsWithPullThrough(
      out,
      pullSpecs,
      structure.foreignKeys ?? [],
      async (pageId, id) => {
        const res = await client.getItem({ pageId, id })
        return res.item ?? null
      },
      referencedContextByFk,
      displayFieldsByStorageKey,
    )
  }
  return { items: out, extraFields }
}
