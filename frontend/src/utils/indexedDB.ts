/**
 * IndexedDB utility for caching table data offline
 * Only tables with a "uuid" column are cached
 */

const DB_NAME = 'sickrock-offline'
const DB_VERSION = 1
const STORE_NAME = 'tableData'

interface TableData {
  tableName: string
  items: any[]
  cachedAt: number
  where?: Record<string, string>
}

let db: IDBDatabase | null = null

/**
 * Initialize IndexedDB database
 */
export async function initDB(): Promise<IDBDatabase> {
  if (db) {
    return db
  }

  return new Promise((resolve, reject) => {
    const request = indexedDB.open(DB_NAME, DB_VERSION)

    request.onerror = () => {
      console.error('Failed to open IndexedDB:', request.error)
      reject(request.error)
    }

    request.onsuccess = () => {
      db = request.result
      resolve(db)
    }

    request.onupgradeneeded = (event) => {
      const database = (event.target as IDBOpenDBRequest).result

      // Create object store if it doesn't exist
      if (!database.objectStoreNames.contains(STORE_NAME)) {
        const objectStore = database.createObjectStore(STORE_NAME, { keyPath: 'tableName' })
        objectStore.createIndex('cachedAt', 'cachedAt', { unique: false })
      }
    }
  })
}

/**
 * Check if a table has a uuid column
 */
export function hasUuidColumn(fields: Array<{ name: string }>): boolean {
  return fields.some(field => field.name.toLowerCase() === 'uuid')
}

/**
 * Save table data to IndexedDB
 */
export async function saveTableData(
  tableName: string,
  items: any[],
  where?: Record<string, string>
): Promise<void> {
  try {
    const database = await initDB()
    const transaction = database.transaction([STORE_NAME], 'readwrite')
    const store = transaction.objectStore(STORE_NAME)

    const data: TableData = {
      tableName,
      items,
      cachedAt: Date.now(),
      where
    }

    await new Promise<void>((resolve, reject) => {
      const request = store.put(data)
      request.onsuccess = () => resolve()
      request.onerror = () => reject(request.error)
    })
  } catch (error) {
    console.error('Failed to save table data to IndexedDB:', error)
  }
}

/**
 * Load table data from IndexedDB
 */
export async function loadTableData(
  tableName: string,
  where?: Record<string, string>
): Promise<any[] | null> {
  try {
    const database = await initDB()
    const transaction = database.transaction([STORE_NAME], 'readonly')
    const store = transaction.objectStore(STORE_NAME)

    return new Promise<any[] | null>((resolve, reject) => {
      const request = store.get(tableName)

      request.onsuccess = () => {
        const data: TableData | undefined = request.result

        if (!data) {
          resolve(null)
          return
        }

        // If where filters are provided, check if they match
        // For simplicity, we'll return cached data if it exists
        // More sophisticated filtering could be added later
        if (where && Object.keys(where).length > 0) {
          // For now, return cached data if available
          // In a production app, you might want to filter the cached items
          resolve(data.items)
        } else {
          resolve(data.items)
        }
      }

      request.onerror = () => {
        reject(request.error)
      }
    })
  } catch (error) {
    console.error('Failed to load table data from IndexedDB:', error)
    return null
  }
}

/**
 * Clear cached data for a specific table
 */
export async function clearTableData(tableName: string): Promise<void> {
  try {
    const database = await initDB()
    const transaction = database.transaction([STORE_NAME], 'readwrite')
    const store = transaction.objectStore(STORE_NAME)

    await new Promise<void>((resolve, reject) => {
      const request = store.delete(tableName)
      request.onsuccess = () => resolve()
      request.onerror = () => reject(request.error)
    })
  } catch (error) {
    console.error('Failed to clear table data from IndexedDB:', error)
  }
}

/**
 * Clear all cached table data
 */
export async function clearAllTableData(): Promise<void> {
  try {
    const database = await initDB()
    const transaction = database.transaction([STORE_NAME], 'readwrite')
    const store = transaction.objectStore(STORE_NAME)

    await new Promise<void>((resolve, reject) => {
      const request = store.clear()
      request.onsuccess = () => resolve()
      request.onerror = () => reject(request.error)
    })
  } catch (error) {
    console.error('Failed to clear all table data from IndexedDB:', error)
  }
}

/**
 * Check if we're online
 */
export function isOnline(): boolean {
  return navigator.onLine
}
