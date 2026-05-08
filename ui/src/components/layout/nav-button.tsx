import React from "react";
import { cn } from "@/lib/utils";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";

interface NavItem {
  path: string;
  label: string;
  icon: React.ElementType;
  group: string;
}

interface NavButtonProps {
  item: NavItem;
  isActive: boolean;
  sidebarOpen: boolean;
  onSelect: (path: string) => void;
}

export function NavButton({
  item,
  isActive,
  sidebarOpen,
  onSelect,
}: NavButtonProps) {
  const Icon = item.icon;
  const link = (
    <a
      href={`/dashboard${item.path}`}
      onClick={(e) => {
        e.preventDefault();
        onSelect(item.path);
      }}
      className={cn(
        "flex items-center gap-3 w-full px-3 py-2 mx-2 rounded-lg text-sm font-medium transition-colors",
        isActive
          ? "bg-primary text-primary-foreground"
          : "text-muted-foreground hover:bg-accent hover:text-accent-foreground",
      )}
    >
      <Icon className="w-4 h-4 shrink-0" />
      {sidebarOpen && <span className="truncate">{item.label}</span>}
    </a>
  );

  if (!sidebarOpen) {
    return (
      <Tooltip delayDuration={0}>
        <TooltipTrigger asChild>{link}</TooltipTrigger>
        <TooltipContent side="right" sideOffset={8}>
          {item.label}
        </TooltipContent>
      </Tooltip>
    );
  }

  return link;
}
