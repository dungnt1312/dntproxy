import type { Connection } from '@/types/connections'

export interface ApiKey {
  id: string
  name: string
  key: string
  isActive: boolean
  dashboardAccess: boolean
  createdAt: string
  updatedAt: string
  allowedConnectionIds: string[]
  allowedModels: string[]
}

export interface ApiKeyCreatePayload {
  name: string
  dashboardAccess?: boolean
  allowedConnectionIds: string[]
  allowedModels: string[]
}

export interface ApiKeyUpdatePayload {
  name?: string
  isActive?: boolean
  dashboardAccess?: boolean
  allowedConnectionIds: string[]
  allowedModels: string[]
}

export interface ModelOption {
  id: string
  modelId?: string
  displayName?: string
  provider: string
  connectionId?: string
  connectionName?: string
  isActive?: boolean
}

export type ConnectionOption = Pick<Connection, 'id' | 'name' | 'provider' | 'isActive' | 'supportedModels'>
