import React, { useState, useSyncExternalStore } from "react";
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
  Layers,
  MessageSquare,
  ScrollText,
  Settings as SettingsIcon,
  Key,
  HardDriveDownload,
  ChevronLeft,
  ChevronRight,
  Zap,
  Moon,
  Sun,
  Menu,
} from "lucide-react";
import { useTheme } from "next-themes";

import { Button } from "@/components/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Separator } from "@/components/ui/separator";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { useIsMobile } from "@/hooks/use-mobile";
import { cn } from "@/lib/utils";
import { useAppStore } from "@/stores/app-store";

import DashboardScreen from "@/components/screens/dashboard-screen";
import ConnectionsScreen from "@/components/screens/connections-screen";
import ModelsScreen from "@/components/screens/models-screen";
import CombosScreen from "@/components/screens/combos-screen";
import PlaygroundScreen from "@/components/screens/playground-screen";
import LogsScreen from "@/components/screens/logs-screen";
import SettingsScreen from "@/components/screens/settings-screen";
import ApiKeysScreen from "@/components/screens/api-keys-screen";
import BackupScreen from "@/components/screens/backup-screen";
import AddConnectionPage from "@/components/pages/add-connection-page";

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
    label: "Models & Aliases",
    icon: Boxes,
    group: "Management",
  },
  { path: "/combos", label: "Combos", icon: Layers, group: "Management" },
  { path: "/api-keys", label: "API Keys", icon: Key, group: "Security" },
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

  const NavButton = ({ item }: { item: NavItem }) => {
    const Icon = item.icon;
    const active = isActive(item.path);
    const button = (
      <button
        key={item.path}
        onClick={() => handlePageSelect(item.path)}
        className={cn(
          "flex items-center gap-3 w-full px-3 py-2 mx-2 rounded-lg text-sm font-medium transition-colors",
          active
            ? "bg-primary text-primary-foreground"
            : "text-muted-foreground hover:bg-accent hover:text-accent-foreground",
        )}
      >
        <Icon className="w-4 h-4 shrink-0" />
        {sidebarOpen && <span className="truncate">{item.label}</span>}
      </button>
    );

    if (!sidebarOpen) {
      return (
        <Tooltip key={item.path} delayDuration={0}>
          <TooltipTrigger asChild>{button}</TooltipTrigger>
          <TooltipContent side="right" sideOffset={8}>
            {item.label}
          </TooltipContent>
        </Tooltip>
      );
    }

    return button;
  };

  return (
    <div className="min-h-screen bg-background md:flex">
      {/* Desktop Sidebar */}
      <aside
        className={cn(
          "fixed left-0 top-0 z-40 hidden h-screen border-r border-border bg-card transition-all duration-300 ease-in-out md:flex md:flex-col",
          sidebarOpen ? "w-64" : "w-16",
        )}
      >
        <div className="flex items-center gap-3 px-4 h-16 border-b border-border shrink-0">
          <div className="flex items-center justify-center w-8 h-8 rounded-lg bg-primary text-primary-foreground">
            <Zap className="w-4 h-4" />
          </div>
          {sidebarOpen && (
            <span className="font-semibold text-lg tracking-tight truncate">
              Dntproxy
            </span>
          )}
        </div>

        <ScrollArea className="flex-1 py-3">
          {Object.entries(groupedItems).map(([group, items]) => (
            <div key={group} className="mb-2">
              {sidebarOpen && (
                <div className="px-4 py-1.5 text-xs font-medium text-muted-foreground uppercase tracking-wider">
                  {group}
                </div>
              )}
              {!sidebarOpen && <Separator className="my-2 mx-2" />}
              {items.map((item) => (
                <NavButton key={item.path} item={item} />
              ))}
            </div>
          ))}
        </ScrollArea>

        <div className="border-t border-border p-2 shrink-0 flex items-center gap-1">
          <Tooltip delayDuration={0}>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
                className="w-8 h-8"
              >
                {mounted && theme === "dark" ? (
                  <Sun className="w-4 h-4" />
                ) : (
                  <Moon className="w-4 h-4" />
                )}
              </Button>
            </TooltipTrigger>
            <TooltipContent side="right">
              {mounted && theme === "dark" ? "Light Mode" : "Dark Mode"}
            </TooltipContent>
          </Tooltip>

          <Tooltip delayDuration={0}>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                onClick={toggleSidebar}
                className="w-8 h-8"
              >
                {sidebarOpen ? (
                  <ChevronLeft className="w-4 h-4" />
                ) : (
                  <ChevronRight className="w-4 h-4" />
                )}
              </Button>
            </TooltipTrigger>
            <TooltipContent side="right">
              {sidebarOpen ? "Collapse Sidebar" : "Expand Sidebar"}
            </TooltipContent>
          </Tooltip>
        </div>
      </aside>

      {/* Mobile Sheet */}
      <Sheet open={mobileMenuOpen} onOpenChange={setMobileMenuOpen}>
        <SheetContent side="left" className="w-[280px] p-0 md:hidden">
          <SheetHeader className="border-b border-border px-4 py-3 text-left">
            <div className="flex items-center gap-3">
              <div className="flex items-center justify-center w-8 h-8 rounded-lg bg-primary text-primary-foreground">
                <Zap className="w-4 h-4" />
              </div>
              <div>
                <SheetTitle className="text-base tracking-tight">
                  Dntproxy
                </SheetTitle>
                <SheetDescription className="text-xs">
                  Navigation
                </SheetDescription>
              </div>
            </div>
          </SheetHeader>
          <ScrollArea className="h-[calc(100%-72px)] py-3">
            {Object.entries(groupedItems).map(([group, items]) => (
              <div key={group} className="mb-3">
                <div className="px-4 py-1.5 text-xs font-medium text-muted-foreground uppercase tracking-wider">
                  {group}
                </div>
                {items.map((item) => {
                  const Icon = item.icon;
                  const active = isActive(item.path);
                  return (
                    <button
                      key={item.path}
                      onClick={() => handlePageSelect(item.path)}
                      className={cn(
                        "flex items-center gap-3 w-full px-3 py-2 mx-2 rounded-lg text-sm font-medium transition-colors",
                        active
                          ? "bg-primary text-primary-foreground"
                          : "text-muted-foreground hover:bg-accent hover:text-accent-foreground",
                      )}
                    >
                      <Icon className="w-4 h-4 shrink-0" />
                      <span className="truncate">{item.label}</span>
                    </button>
                  );
                })}
              </div>
            ))}
          </ScrollArea>
        </SheetContent>
      </Sheet>

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
            <Route path="/combos" element={<CombosScreen />} />
            <Route path="/playground" element={<PlaygroundScreen />} />
            <Route path="/logs" element={<LogsScreen />} />
            <Route path="/settings" element={<SettingsScreen />} />
            <Route path="/api-keys" element={<ApiKeysScreen />} />
            <Route path="/backup" element={<BackupScreen />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </div>
      </main>
    </div>
  );
}
