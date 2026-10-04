import { computed, ref } from 'vue'
import type { IamUser } from '../gen/sickrock_pb'
import { createApiClient } from '../stores/api'

export function useIamUsers() {
  const client = createApiClient()
  const users = ref<IamUser[]>([])
  const loading = ref(false)
  const loaded = ref(false)

  const usersById = computed(() => {
    const map = new Map<string, IamUser>()
    for (const user of users.value) {
      if (user.id != null) {
        map.set(String(user.id), user)
      }
    }
    return map
  })

  async function loadUsers() {
    if (loaded.value || loading.value) return
    loading.value = true
    try {
      const response = await client.listUsers({})
      users.value = response.users ?? []
      loaded.value = true
    } finally {
      loading.value = false
    }
  }

  function displayUserLabel(userId: unknown): string {
    if (userId == null || userId === '') return ''
    const user = usersById.value.get(String(userId))
    return user?.username?.trim() || `User #${userId}`
  }

  return {
    users,
    loading,
    loaded,
    usersById,
    loadUsers,
    displayUserLabel,
  }
}
