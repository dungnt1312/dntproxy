import React from "react";
import { Zap, ChevronLeft, ChevronRight, Moon, Sun } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Separator } from "@/components/ui/separator";
import { NavButton } from "./nav-button";

interface NavItem {
  path: string;
  label: string;
  icon: React.ElementType;
  group: string;
}

interface SidebarProps {
  sidebarOpen: boolean;
  groupedItems: Record<string, NavItem[]>;
  isActive: (path: string) => boolean;
  onNavigate: (path: string) => void;
  onToggleSidebar: () => void;
  theme: string | undefined;
  onToggleTheme: () => void;
  mounted: boolean;
}

export function Sidebar({
  sidebarOpen,
  groupedItems,
  isActive,
  onNavigate,
  onToggleSidebar,
  theme,
  onToggleTheme,
  mounted,
}: SidebarProps) {
  return (
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
              <NavButton
                key={item.path}
                item={item}
                isActive={isActive(item.path)}
                sidebarOpen={sidebarOpen}
                onSelect={onNavigate}
              />
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
              onClick={onToggleTheme}
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
              onClick={onToggleSidebar}
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
  );
}
