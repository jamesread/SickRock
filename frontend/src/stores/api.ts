import { useAuthStore } from './auth'
import { createClient } from '@connectrpc/connect'
import { createConnectTransport } from '@connectrpc/connect-web'
import {
  SickRock
} from '../gen/sickrock_pb'
import { Interceptor } from '@connectrpc/connect'

const SESSION_TOKEN_KEY = 'session-token'

function authInterceptor(getToken: () => string | undefined): Interceptor {
  return (next) => async (req) => {
    const token = getToken()
    if (token) {
      req.header.set('Authorization', `Bearer ${token}`)
      req.header.set('Session-Token', token)
    }
    return await next(req)
  }
}


/** The signed-in session token sent with API requests, if any. */
export function currentSessionToken(): string | undefined {
  const authStore = useAuthStore()
  return authStore.user?.token ?? localStorage.getItem(SESSION_TOKEN_KEY) ?? undefined
}

// ConnectRPC-compatible API client using fetch with proper message construction
export const createApiClient = () => {
  const transport = createConnectTransport({
    baseUrl: '/api',
    fetch: (input, init) => fetch(input, { ...init, credentials: 'include' }),
    interceptors: [
      authInterceptor(currentSessionToken),
    ],
  })

  const client = createClient(SickRock, transport)

  return client
}
