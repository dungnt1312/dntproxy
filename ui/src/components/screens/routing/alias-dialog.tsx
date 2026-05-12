import { useEffect, useMemo, useState } from "react";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";
import { goApi } from "@/lib/go-api";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import type { AliasMap, UiModel } from "./types";
import { ModelPicker } from "./model-picker";

interface AliasDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  aliases: AliasMap;
  models: UiModel[];
  onRefresh: () => Promise<void>;
}

export function AliasDialog({ open, onOpenChange, aliases, models, onRefresh }: AliasDialogProps) {
  const [alias, setAlias] = useState("");
  const [target, setTarget] = useState("");
  const [saving, setSaving] = useState(false);
  const duplicate = useMemo(() => Boolean(alias.trim() && aliases[alias.trim()]), [alias, aliases]);

  useEffect(() => {
    if (!open) {
      setAlias("");
      setTarget("");
      setSaving(false);
    }
  }, [open]);

  async function handleSave() {
    const aliasName = alias.trim();
    const modelTarget = target.trim();

    if (!aliasName || !modelTarget) {
      toast.error("Alias name and target are required");
      return;
    }

    if (duplicate) {
      toast.error("Alias already exists");
      return;
    }

    setSaving(true);
    try {
      await goApi.setAlias(aliasName, modelTarget);
      toast.success("Alias created");
      onOpenChange(false);
      await onRefresh();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to create alias");
    } finally {
      setSaving(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>Create alias</DialogTitle>
          <DialogDescription>Map a short request model name to a registry model.</DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="alias-name">Alias name</Label>
            <Input
              id="alias-name"
              value={alias}
              onChange={(event) => setAlias(event.target.value)}
              placeholder="e.g. fast-coder"
              aria-invalid={duplicate}
            />
            {duplicate && <p className="text-xs text-destructive">This alias already exists.</p>}
          </div>
          <div className="space-y-2">
            <Label>Target model</Label>
            <ModelPicker models={models} value={target} onChange={setTarget} allowManual />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button onClick={handleSave} disabled={saving || !alias.trim() || !target.trim() || duplicate}>
            {saving ? <Loader2 className="mr-2 h-3.5 w-3.5 animate-spin" /> : null}
            Create alias
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
