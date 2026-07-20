export interface Tenant {
  id: string
  slug: string
  name: string
  status: 'active' | 'disabled'
  notes?: string
  createdAt?: string
  updatedAt?: string
  // Aggregated stats (only present in list responses)
  connections?: number
  combos?: number
  keys?: number
}

export interface TenantCreatePayload {
  slug: string
  name?: string
  notes?: string
}

export interface TenantUpdatePayload {
  name?: string
  notes?: string
  status?: 'active' | 'disabled'
}

export interface TenantKeyPayload {
  name: string
  dashboardAccess?: boolean
  allowedConnectionIds?: string[]
  allowedModels?: string[]
}
