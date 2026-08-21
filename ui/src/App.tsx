import React, { useState, useEffect, useCallback, useSyncExternalStore } from "react";
import {
  Outlet,
  useNavigate,
  useLocation,
  Navigate,
  type RouteObject,
} from "react-router-dom";
import {
  LayoutDashboard,
  Link2,
  Boxes,
  MessageSquare,
  ScrollText,
  Settings as SettingsIcon,
  Key,
  HardDriveDownload,
  Moon,
  Sun,
  Menu,
  Globe,
  UserCircle,
  BarChart2,
  Wrench,
  Building2,
  GitBranch,
} from "lucide-react";
import { useTheme } from "next-themes";

import { Button } from "@/components/ui/button";
import { useIsMobile } from "@/hooks/use-mobile";
import { cn } from "@/lib/utils";
import { useAppStore } from "@/stores/app-store";
import { getStoredApiKey, clearStoredApiKey, onUnauthorized, goApi } from "@/lib/go-api";
import { Sidebar } from "@/components/layout/sidebar";
import { MobileMenu } from "@/components/layout/mobile-menu";
import { TenantBadge } from "@/components/layout/tenant-badge";

import DashboardScreen from "@/components/screens/dashboard-screen";
import ConnectionsScreen from "@/components/screens/connections-screen";
import ModelsScreen from "@/components/screens/models-screen";
import CombosScreen from "@/components/screens/combos-screen";
import PlaygroundScreen from "@/components/screens/playground-screen";
import LogsScreen from "@/components/screens/logs-screen";
import SettingsScreen from "@/components/screens/settings-screen";
import ApiKeysScreen from "@/components/screens/api-keys-screen";
import BackupScreen from "@/components/screens/backup-screen";
import UsageScreen from "@/components/screens/usage-screen";
import TunnelScreen from "@/components/screens/tunnel-screen";
import ProfilesScreen from "@/components/screens/profiles-screen";
import ToolsScreen from "@/components/screens/tools-screen";
import AddConnectionPage from "@/components/pages/add-connection-page";
import LoginScreen from "@/components/screens/login-screen";
import TenantsScreen from "@/components/screens/tenants-screen";

interface NavItem {
  path: string;
  label: string;
  icon: React.ElementType;
  group: string;
}

const navItems: NavItem[] = [
  { path: "/", label: "Dashboard", icon: LayoutDashboard, group: "Overview" },
  {
    path: "/playground",
    label: "Playground",
    icon: MessageSquare,
    group: "Overview",
  },
  {
    path: "/connections",
    label: "Connections",
    icon: Link2,
    group: "Management",
  },
  {
    path: "/models",
    label: "Model Registry",
    icon: Boxes,
    group: "Routing",
  },
  {
    path: "/combos",
    label: "Combos",
    icon: GitBranch,
    group: "Routing",
  },
  // Admin-only: registered dynamically below via adminNavItems
  // { path: "/profiles", label: "Profiles", icon: UserCircle, group: "Management" },
  // { path: "/tools", label: "AI Tools", icon: Wrench, group: "Management" },
  { path: "/api-keys", label: "API Keys", icon: Key, group: "Security" },
  { path: "/logs", label: "Logs", icon: ScrollText, group: "Monitoring" },
];

const adminNavItems: NavItem[] = [
  { path: "/tenants", label: "Tenants", icon: Building2, group: "Management" },
  { path: "/tunnel", label: "Tunnel", icon: Globe, group: "Network" },
  { path: "/usage", label: "Usage", icon: BarChart2, group: "Monitoring" },
  { path: "/settings", label: "Settings", icon: SettingsIcon, group: "System" },
  {
    path: "/backup",
    label: "Backup",
    icon: HardDriveDownload,
    group: "System",
  },
];

const emptySubscribe = () => () => {};

function useIsMounted() {
  return useSyncExternalStore(
    emptySubscribe,
    () => true,
    () => false,
  );
}

// Route-level admin gate (session lives in the zustand store, so this works
// outside the App component render where routes are now declared).
function RequireAdmin({ children }: { children: React.ReactNode }) {
  const isAdmin = useAppStore((s) => s.session?.isAdmin);
  return isAdmin ? <>{children}</> : <Navigate to="/" replace />;
}

export const appRoutes: RouteObject[] = [
  { path: "/", element: <DashboardScreen /> },
  { path: "/connections", element: <ConnectionsScreen /> },
  { path: "/connections/add", element: <AddConnectionPage /> },
  { path: "/models", element: <ModelsScreen /> },
  { path: "/combos", element: <CombosScreen /> },
  { path: "/profiles", element: <ProfilesScreen /> },
  { path: "/tools", element: <ToolsScreen /> },
  { path: "/playground", element: <PlaygroundScreen /> },
  { path: "/usage", element: <RequireAdmin><UsageScreen /></RequireAdmin> },
  { path: "/logs", element: <LogsScreen /> },
  { path: "/settings", element: <RequireAdmin><SettingsScreen /></RequireAdmin> },
  { path: "/api-keys", element: <ApiKeysScreen /> },
  { path: "/tenants", element: <RequireAdmin><TenantsScreen /></RequireAdmin> },
  { path: "/tunnel", element: <RequireAdmin><TunnelScreen /></RequireAdmin> },
  { path: "/backup", element: <RequireAdmin><BackupScreen /></RequireAdmin> },
  { path: "*", element: <Navigate to="/" replace /> },
];

export default function App() {
  const { sidebarOpen, toggleSidebar, session, setSession, clearSession } = useAppStore();
  const { theme, setTheme } = useTheme();
  const mounted = useIsMounted();
  const isMobile = useIsMobile();
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const navigate = useNavigate();
  const location = useLocation();

  // Dashboard auth is always required (matches BE dashboardKeyMiddleware).
  const [authReady, setAuthReady] = useState(false);
  const [authenticated, setAuthenticated] = useState(false);

  const showLogin = useCallback(() => {
    setAuthenticated(false);
    clearSession();
  }, [clearSession]);

  const handleLogout = useCallback(() => {
    clearStoredApiKey();
    clearSession();
    setAuthenticated(false);
  }, [clearSession]);

  useEffect(() => {
    onUnauthorized(showLogin);
  }, [showLogin]);

  // On mount: if a key is stored, re-resolve session from the backend so a
  // page reload preserves multi-tenancy context. Fail closed on errors.
  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const stored = getStoredApiKey();
        if (!stored) {
          if (!cancelled) {
            setAuthenticated(false);
            setAuthReady(true);
          }
          return;
        }
        const sess = await goApi.getSession();
        if (cancelled) return;
        if (sess.authenticated) {
          setAuthenticated(true);
          setSession({
            tenantId: sess.tenantId ?? "",
            isAdmin: Boolean(sess.isAdmin),
            dashboardAccess: Boolean(sess.dashboardAccess),
            keyId: sess.keyId,
            keyName: sess.keyName,
          });
        } else {
          clearStoredApiKey();
          setAuthenticated(false);
        }
      } catch {
        // Fail closed: network/server error during bootstrap → login
        if (!cancelled) setAuthenticated(false);
      } finally {
        if (!cancelled) setAuthReady(true);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [setSession]);

  if (!authReady) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-background">
        <div className="animate-pulse text-muted-foreground text-sm">Loading...</div>
      </div>
    );
  }

  if (!authenticated) {
    return <LoginScreen onSuccess={() => setAuthenticated(true)} />;
  }

  const currentNavItem = [...navItems, ...adminNavItems].find((item) =>
    item.path === "/"
      ? location.pathname === "/"
      : location.pathname.startsWith(item.path),
  );

  // Compose the nav list: base items + admin-only items when the session is
  // an admin (global/legacy key). Tenant users never see the Tenants nav item.
  const effectiveNavItems = session?.isAdmin
    ? [...navItems, ...adminNavItems]
    : navItems;

  const groupedItems = effectiveNavItems.reduce<Record<string, NavItem[]>>(
    (acc, item) => {
      if (!acc[item.group]) acc[item.group] = [];
      acc[item.group].push(item);
      return acc;
    },
    {},
  );

  const handlePageSelect = (path: string) => {
    navigate(path);
    setMobileMenuOpen(false);
  };

  const isActive = (path: string) =>
    path === "/"
      ? location.pathname === "/"
      : location.pathname.startsWith(path);

  return (
    <div className="min-h-screen bg-background md:flex">
      {/* Desktop Sidebar */}
      <Sidebar
        sidebarOpen={sidebarOpen}
        groupedItems={groupedItems}
        isActive={isActive}
        onNavigate={handlePageSelect}
        onToggleSidebar={toggleSidebar}
        theme={theme}
        onToggleTheme={() => setTheme(theme === "dark" ? "light" : "dark")}
        mounted={mounted}
        session={session}
        onLogout={handleLogout}
      />

      {/* Mobile Sheet */}
      <MobileMenu
        open={mobileMenuOpen}
        onOpenChange={setMobileMenuOpen}
        groupedItems={groupedItems}
        isActive={isActive}
        onNavigate={handlePageSelect}
        onLogout={handleLogout}
      />

      {/* Main content */}
      <main
        className={cn(
          "w-full flex-1 transition-all duration-300 ease-in-out",
          isMobile ? "ml-0" : sidebarOpen ? "md:ml-64" : "md:ml-16",
        )}
      >
        <header className="sticky top-0 z-30 flex items-center justify-between border-b border-border bg-background/95 px-4 py-3 backdrop-blur md:hidden">
          <div className="flex items-center gap-2 min-w-0">
            <Button
              variant="outline"
              size="icon"
              onClick={() => setMobileMenuOpen(true)}
              className="h-9 w-9 shrink-0"
            >
              <Menu className="h-4 w-4" />
            </Button>
            <p className="truncate text-sm font-semibold">
              {currentNavItem?.label ?? "Dntproxy"}
            </p>
            {session && (
              <TenantBadge
                tenantId={session.tenantId}
                isAdmin={session.isAdmin}
              />
            )}
          </div>
          <Button
            variant="ghost"
            size="icon"
            onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
            className="h-9 w-9 shrink-0"
          >
            {mounted && theme === "dark" ? (
              <Sun className="w-4 h-4" />
            ) : (
              <Moon className="w-4 h-4" />
            )}
          </Button>
        </header>

        <div className="w-full p-4 md:p-6">
          <Outlet />
        </div>
      </main>
    </div>
  );
}
