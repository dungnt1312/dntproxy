import React from "react";
import { Zap } from "lucide-react";
import { cn } from "@/lib/utils";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { ScrollArea } from "@/components/ui/scroll-area";

interface NavItem {
  path: string;
  label: string;
  icon: React.ElementType;
  group: string;
}

interface MobileMenuProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  groupedItems: Record<string, NavItem[]>;
  isActive: (path: string) => boolean;
  onNavigate: (path: string) => void;
}

export function MobileMenu({
  open,
  onOpenChange,
  groupedItems,
  isActive,
  onNavigate,
}: MobileMenuProps) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
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
                    onClick={() => onNavigate(item.path)}
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
  );
}
