import { useState } from "react";
import { Key, Zap, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { goApi } from "@/lib/go-api";
import { setStoredApiKey } from "@/lib/go-api";
import { useAppStore } from "@/stores/app-store";

interface LoginScreenProps {
  onSuccess: () => void;
}

export default function LoginScreen({ onSuccess }: LoginScreenProps) {
  const [key, setKey] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const setSession = useAppStore((s) => s.setSession);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!key.trim()) return;

    setLoading(true);
    setError("");

    try {
      const res = await goApi.validateKey(key.trim());
      if (res.valid && res.dashboardAccess) {
        setStoredApiKey(key.trim());
        // Capture tenant/admin context from the validated key.
        setSession({
          tenantId: res.tenantId ?? "",
          isAdmin: Boolean(res.isAdmin),
          dashboardAccess: true,
        });
        onSuccess();
      } else if (res.valid && !res.dashboardAccess) {
        setError("This key does not have dashboard access");
      } else {
        setError("Invalid API key");
      }
    } catch {
      setError("Failed to validate key");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-background p-4">
      <div className="w-full max-w-sm space-y-6">
        <div className="flex flex-col items-center gap-3">
          <div className="flex items-center justify-center w-12 h-12 rounded-xl bg-primary text-primary-foreground">
            <Zap className="w-6 h-6" />
          </div>
          <h1 className="text-xl font-semibold tracking-tight">Dntproxy</h1>
          <p className="text-sm text-muted-foreground text-center">
            Enter a dashboard API key to continue.
          </p>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="relative">
            <Key className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
            <Input
              type="password"
              placeholder="sk-dnt-..."
              value={key}
              onChange={(e) => setKey(e.target.value)}
              className="pl-10"
              autoFocus
            />
          </div>

          {error && (
            <p className="text-sm text-destructive text-center">{error}</p>
          )}

          <Button type="submit" className="w-full" disabled={loading || !key.trim()}>
            {loading ? <Loader2 className="w-4 h-4 animate-spin mr-2" /> : null}
            Authenticate
          </Button>
        </form>
      </div>
    </div>
  );
}
