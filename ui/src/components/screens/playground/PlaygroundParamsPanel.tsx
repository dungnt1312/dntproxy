import { Card, CardContent } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Slider } from "@/components/ui/slider";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { Textarea } from "@/components/ui/textarea";
import { Eraser, Shield } from "lucide-react";
import type { ChatParams } from "./types";

interface PlaygroundParamsPanelProps {
  params: ChatParams;
  setParams: React.Dispatch<React.SetStateAction<ChatParams>>;
  defaultParams: ChatParams;
}

export function PlaygroundParamsPanel({
  params,
  setParams,
  defaultParams,
}: PlaygroundParamsPanelProps) {
  return (
    <Card className="border-dashed">
      <CardContent className="pt-4">
        <div className="grid gap-4 md:grid-cols-4">
          <div className="space-y-2">
            <Label className="text-xs">
              Temperature: {params.temperature.toFixed(2)}
            </Label>
            <Slider
              aria-label="Temperature"
              value={[params.temperature]}
              onValueChange={([v]) =>
                setParams((p) => ({ ...p, temperature: v }))
              }
              min={0}
              max={2}
              step={0.01}
            />
          </div>

          <div className="space-y-2">
            <Label className="text-xs">Top P: {params.topP.toFixed(2)}</Label>
            <Slider
              aria-label="Top P"
              value={[params.topP]}
              onValueChange={([v]) => setParams((p) => ({ ...p, topP: v }))}
              min={0}
              max={1}
              step={0.01}
            />
          </div>

          <div className="space-y-2">
            <Label className="text-xs">Max Tokens</Label>
            <input
              aria-label="Max tokens"
              type="number"
              value={params.maxTokens}
              onChange={(e) =>
                setParams((p) => ({
                  ...p,
                  maxTokens: parseInt(e.target.value) || 0,
                }))
              }
              className="w-full rounded-md border bg-background px-2 py-1 text-sm"
              min={0}
              max={128000}
            />
          </div>

          <div className="space-y-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setParams(defaultParams)}
              className="w-full text-xs"
            >
              <Eraser className="mr-1 h-3 w-3" />
              Reset
            </Button>
          </div>
        </div>

        <Separator className="my-3" />

        <div className="space-y-2">
          <Label className="text-xs flex items-center gap-1">
            <Shield className="h-3 w-3" />
            System Prompt
          </Label>
            <Textarea
              aria-label="System prompt"
              value={params.systemPrompt}
            onChange={(e) =>
              setParams((p) => ({ ...p, systemPrompt: e.target.value }))
            }
            placeholder="You are a helpful assistant..."
            className="min-h-[60px] text-xs resize-none"
            rows={2}
          />
        </div>
      </CardContent>
    </Card>
  );
}
