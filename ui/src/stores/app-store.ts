import { create } from 'zustand'

export interface SessionInfo {
  /** The tenant id resolved from the authenticated API key. Empty for legacy/global admin keys. */
  tenantId: string
  /** True when the key is a legacy/global admin key (sees all tenants). */
  isAdmin: boolean
  /** Whether dashboard access is granted for this key. */
  dashboardAccess: boolean
  /** The authenticated API key id (display only). */
  keyId?: string
  /** The authenticated API key name (display only). */
  keyName?: string
}

interface AppState {
  sidebarOpen: boolean
  setSidebarOpen: (open: boolean) => void
  toggleSidebar: () => void

  // Session (multi-tenancy)
  session: SessionInfo | null
  setSession: (session: SessionInfo | null) => void
  clearSession: () => void
}

export const useAppStore = create<AppState>((set) => ({
  sidebarOpen: true,
  setSidebarOpen: (open) => set({ sidebarOpen: open }),
  toggleSidebar: () => set((state) => ({ sidebarOpen: !state.sidebarOpen })),

  session: null,
  setSession: (session) => set({ session }),
  clearSession: () => set({ session: null }),
}))
