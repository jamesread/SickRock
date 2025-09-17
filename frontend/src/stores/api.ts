import { useAuthStore } from './auth'

// Simple API client that works with the current setup
export const createApiClient = () => {
  const authStore = useAuthStore()
  
  const getHeaders = () => {
    const authHeaders = authStore.getAuthHeaders()
    return {
      'Content-Type': 'application/json',
      ...authHeaders
    }
  }

  return {
    init: async (req: any = {}) => ({ version: '1.0.0', commit: 'test', date: '2025-01-01' }),
    login: async (req: { username: string; password: string }) => {
      const response = await fetch('/api/sickrock.SickRock/Login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(req)
      })
      return response.json()
    },
    logout: async (req: any = {}) => ({ success: true, message: 'Logged out' }),
    validateToken: async (req: { token: string }) => {
      const response = await fetch('/api/sickrock.SickRock/ValidateToken', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(req)
      })
      return response.json()
    },
    getPages: async (req: any = {}) => {
      const response = await fetch('/api/sickrock.SickRock/GetPages', {
        method: 'POST',
        headers: getHeaders(),
        body: JSON.stringify(req)
      })
      return response.json()
    },
    // Add other methods as needed
    ping: async (req: { message: string }) => {
      const response = await fetch('/api/sickrock.SickRock/Ping', {
        method: 'POST',
        headers: getHeaders(),
        body: JSON.stringify(req)
      })
      return response.json()
    },
    listItems: async (req: { pageId: string }) => {
      const response = await fetch('/api/sickrock.SickRock/ListItems', {
        method: 'POST',
        headers: getHeaders(),
        body: JSON.stringify(req)
      })
      return response.json()
    },
    getTableStructure: async (req: { pageId: string }) => {
      const response = await fetch('/api/sickrock.SickRock/GetTableStructure', {
        method: 'POST',
        headers: getHeaders(),
        body: JSON.stringify(req)
      })
      return response.json()
    },
    addTableColumn: async (req: any) => {
      const response = await fetch('/api/sickrock.SickRock/AddTableColumn', {
        method: 'POST',
        headers: getHeaders(),
        body: JSON.stringify(req)
      })
      return response.json()
    },
    createItem: async (req: any) => {
      const response = await fetch('/api/sickrock.SickRock/CreateItem', {
        method: 'POST',
        headers: getHeaders(),
        body: JSON.stringify(req)
      })
      return response.json()
    },
    editItem: async (req: any) => {
      const response = await fetch('/api/sickrock.SickRock/EditItem', {
        method: 'POST',
        headers: getHeaders(),
        body: JSON.stringify(req)
      })
      return response.json()
    },
    deleteItem: async (req: any) => {
      const response = await fetch('/api/sickrock.SickRock/DeleteItem', {
        method: 'POST',
        headers: getHeaders(),
        body: JSON.stringify(req)
      })
      return response.json()
    },
    getForeignKeys: async (req: any) => {
      const response = await fetch('/api/sickrock.SickRock/GetForeignKeys', {
        method: 'POST',
        headers: getHeaders(),
        body: JSON.stringify(req)
      })
      return response.json()
    },
    getTableViews: async (req: any) => {
      const response = await fetch('/api/sickrock.SickRock/GetTableViews', {
        method: 'POST',
        headers: getHeaders(),
        body: JSON.stringify(req)
      })
      return response.json()
    },
    createTableView: async (req: any) => {
      const response = await fetch('/api/sickrock.SickRock/CreateTableView', {
        method: 'POST',
        headers: getHeaders(),
        body: JSON.stringify(req)
      })
      return response.json()
    },
    updateTableView: async (req: any) => {
      const response = await fetch('/api/sickrock.SickRock/UpdateTableView', {
        method: 'POST',
        headers: getHeaders(),
        body: JSON.stringify(req)
      })
      return response.json()
    },
    deleteTableView: async (req: any) => {
      const response = await fetch('/api/sickrock.SickRock/DeleteTableView', {
        method: 'POST',
        headers: getHeaders(),
        body: JSON.stringify(req)
      })
      return response.json()
    },
    createForeignKey: async (req: any) => {
      const response = await fetch('/api/sickrock.SickRock/CreateForeignKey', {
        method: 'POST',
        headers: getHeaders(),
        body: JSON.stringify(req)
      })
      return response.json()
    },
    deleteForeignKey: async (req: any) => {
      const response = await fetch('/api/sickrock.SickRock/DeleteForeignKey', {
        method: 'POST',
        headers: getHeaders(),
        body: JSON.stringify(req)
      })
      return response.json()
    },
    changeColumnType: async (req: any) => {
      const response = await fetch('/api/sickrock.SickRock/ChangeColumnType', {
        method: 'POST',
        headers: getHeaders(),
        body: JSON.stringify(req)
      })
      return response.json()
    },
    dropColumn: async (req: any) => {
      const response = await fetch('/api/sickrock.SickRock/DropColumn', {
        method: 'POST',
        headers: getHeaders(),
        body: JSON.stringify(req)
      })
      return response.json()
    },
    changeColumnName: async (req: any) => {
      const response = await fetch('/api/sickrock.SickRock/ChangeColumnName', {
        method: 'POST',
        headers: getHeaders(),
        body: JSON.stringify(req)
      })
      return response.json()
    }
  }
}
