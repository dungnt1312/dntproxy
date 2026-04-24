import { useState } from "react";
import { Check, ChevronsUpDown } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from "@/components/ui/command";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { cn } from "@/lib/utils";

import type { ModelOption } from "./types";

interface ModelComboboxProps {
  value: string;
  models: ModelOption[];
  placeholder?: string;
  onChange: (value: string) => void;
}

export function ModelCombobox({ value, models, placeholder = "Select model...", onChange }: ModelComboboxProps) {
  const [open, setOpen] = useState(false);

  const selected = models.find((m) => m.id === value);
  const label = selected ? (selected.displayName || selected.name || selected.id) : "";

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          variant="outline"
          role="combobox"
          aria-expanded={open}
          className="w-full justify-between font-mono text-xs font-normal"
        >
          <span className="truncate">{value ? label : placeholder}</span>
          <ChevronsUpDown className="ml-2 size-3.5 shrink-0 opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-[--radix-popover-trigger-width] p-0" align="start">
        <Command>
          <CommandInput placeholder="Search models..." className="text-xs" />
          <CommandList>
            <CommandEmpty className="py-3 text-center text-xs">No model found.</CommandEmpty>
            <CommandGroup>
              {models.map((m) => (
                <CommandItem
                  key={m.id}
                  value={m.id}
                  keywords={[m.displayName || "", m.name || "", m.provider]}
                  onSelect={(v) => { onChange(v); setOpen(false); }}
                  className="text-xs"
                >
                  <Check className={cn("mr-2 size-3.5", value === m.id ? "opacity-100" : "opacity-0")} />
                  <span className="truncate">{m.displayName || m.name || m.id}</span>
                </CommandItem>
              ))}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  );
}
