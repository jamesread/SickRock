/**
 * Orders field names by their column order in the table schema. Item fields arrive as a
 * map (serialized alphabetically), so without this a record page lists them A–Z.
 * Names missing from the schema keep their relative order after the known ones.
 */
export function orderBySchema(names: string[], schemaOrder: string[]): string[] {
  const position = new Map(schemaOrder.map((name, index) => [name, index]))
  return names
    .map((name, index) => ({ name, index, rank: position.get(name) ?? schemaOrder.length + index }))
    .sort((a, b) => a.rank - b.rank)
    .map(entry => entry.name)
}
