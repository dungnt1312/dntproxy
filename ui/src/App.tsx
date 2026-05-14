import React, { useState, useEffect, useCallback, useSyncExternalStore } from "react";
import {
  Routes,
  Route,
  useNavigate,
  useLocation,
  Navigate,
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
} from "lucide-react";
import { useTheme } from "next-themes";

import { Button } from "@/components/ui/button";
import { useIsMobile } from "@/hooks/use-mobile";
import { cn } from "@/lib/utils";
import { useAppStore } from "@/stores/app-store";
import { getStoredApiKey, onUnauthorized } from "@/lib/go-api";
import { Sidebar } from "@/components/layout/sidebar";
import { MobileMenu } from "@/components/layout/mobile-menu";

import DashboardScreen from "@/components/screens/dashboard-screen";
import ConnectionsScreen from "@/components/screens/connections-screen";
import ModelsScreen from "@/components/screens/models-screen";
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
    group: "Management",
  },
  // { path: "/profiles", label: "Profiles", icon: UserCircle, group: "Management" },
  { path: "/tools", label: "AI Tools", icon: Wrench, group: "Management" },
  { path: "/api-keys", label: "API Keys", icon: Key, group: "Security" },
  { path: "/tunnel", label: "Tunnel", icon: Globe, group: "Network" },
  { path: "/usage", label: "Usage", icon: BarChart2, group: "Monitoring" },
  { path: "/logs", label: "Logs", icon: ScrollText, group: "Monitoring" },
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

export default function App() {
  const { sidebarOpen, toggleSidebar } = useAppStore();
  const { theme, setTheme } = useTheme();
  const mounted = useIsMounted();
  const isMobile = useIsMobile();
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const navigate = useNavigate();
  const location = useLocation();

  // Auth gate: check if requireApiKey is enabled
  const [authRequired, setAuthRequired] = useState<boolean | null>(null);
  const [authenticated, setAuthenticated] = useState(false);

  const showLogin = useCallback(() => {
    setAuthenticated(false);
  }, []);

  useEffect(() => {
    onUnauthorized(showLogin);
  }, [showLogin]);

  useEffect(() => {
    fetch((import.meta.env.VITE_GO_API_URL || "/api") + "/settings")
      .then((r) => r.json())
      .then((s) => {
        const required = Boolean(s.requireApiKey);
        setAuthRequired(required);
        if (!required) {
          setAuthenticated(true);
        } else if (getStoredApiKey()) {
          setAuthenticated(true);
        }
      })
      .catch(() => {
        setAuthRequired(false);
        setAuthenticated(true);
      });
  }, []);

  // Still loading settings
  if (authRequired === null) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-background">
        <div className="animate-pulse text-muted-foreground text-sm">Loading...</div>
      </div>
    );
  }

  // Need login
  if (authRequired && !authenticated) {
    return <LoginScreen onSuccess={() => setAuthenticated(true)} />;
  }

  const currentNavItem = navItems.find((item) =>
    item.path === "/"
      ? location.pathname === "/"
      : location.pathname.startsWith(item.path),
  );

  const groupedItems = navItems.reduce<Record<string, NavItem[]>>(
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
      />

      {/* Mobile Sheet */}
      <MobileMenu
        open={mobileMenuOpen}
        onOpenChange={setMobileMenuOpen}
        groupedItems={groupedItems}
        isActive={isActive}
        onNavigate={handlePageSelect}
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
          <Routes>
            <Route path="/" element={<DashboardScreen />} />
            <Route path="/connections" element={<ConnectionsScreen />} />
            <Route path="/connections/add" element={<AddConnectionPage />} />
            <Route path="/models" element={<ModelsScreen />} />
            <Route path="/profiles" element={<ProfilesScreen />} />
            <Route path="/tools" element={<ToolsScreen />} />
            <Route path="/playground" element={<PlaygroundScreen />} />
            <Route path="/usage" element={<UsageScreen />} />
            <Route path="/logs" element={<LogsScreen />} />
            <Route path="/settings" element={<SettingsScreen />} />
            <Route path="/api-keys" element={<ApiKeysScreen />} />
            <Route path="/tunnel" element={<TunnelScreen />} />
            <Route path="/backup" element={<BackupScreen />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </div>
      </main>
    </div>
  );
}
