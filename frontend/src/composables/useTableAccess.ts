import { ref, watch, type Ref } from 'vue'
import { createApiClient } from '../stores/api'
import { useRbac } from './useRbac'

export type TableAccessFlags = {
  canView: boolean
  canInsert: boolean
  canEdit: boolean
  canDelete: boolean
}

const fullAccess: TableAccessFlags = {
  canView: true,
  canInsert: true,
  canEdit: true,
  canDelete: true,
}

export function useTableAccess(tableName: Ref<string> | string) {
  const client = createApiClient()
  const { isSuperuser } = useRbac()
  const access = ref<TableAccessFlags>({
    canView: false,
    canInsert: false,
    canEdit: false,
    canDelete: false,
  })
  const loading = ref(false)

  async function load(name: string) {
    if (!name) {
      access.value = {
        canView: false,
        canInsert: false,
        canEdit: false,
        canDelete: false,
      }
      return
    }
    if (isSuperuser.value) {
      access.value = { ...fullAccess }
      return
    }
    loading.value = true
    try {
      const res = await client.getTableAccess({ tableName: name })
      access.value = {
        canView: Boolean(res.canView),
        canInsert: Boolean(res.canInsert),
        canEdit: Boolean(res.canEdit),
        canDelete: Boolean(res.canDelete),
      }
    } catch {
      access.value = {
        canView: false,
        canInsert: false,
        canEdit: false,
        canDelete: false,
      }
    } finally {
      loading.value = false
    }
  }

  if (typeof tableName === 'string') {
    void load(tableName)
  } else {
    watch(tableName, (n) => void load(n), { immediate: true })
  }

  return { access, loading, reload: () => load(typeof tableName === 'string' ? tableName : tableName.value) }
}
