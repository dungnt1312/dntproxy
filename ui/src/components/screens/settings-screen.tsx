import { useEffect, useState, useCallback } from "react";
import { motion } from "framer-motion";
import { Settings, ShieldAlert, RotateCcw, Save, Loader2, Sparkles } from "lucide-react";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  CardDescription,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { toast } from "sonner";
import { goApi } from "@/lib/go-api";

interface SettingsData {
  id: string;
  serverPort: number;
  apiKeyAuthEnabled: boolean;
  defaultRoutingStrategy: string;
  compressionEnabled: boolean;
  compressionMinLength: number;
  compressionLogSavings: boolean;
}

const DEFAULT_SETTINGS: SettingsData = {
  id: '',
  serverPort: 20199,
  apiKeyAuthEnabled: true,
  defaultRoutingStrategy: "fallback",
  compressionEnabled: false,
  compressionMinLength: 500,
  compressionLogSavings: true,
};

const containerVariants = {
  hidden: { opacity: 0 },
  visible: {
    opacity: 1,
    transition: { staggerChildren: 0.1 },
  },
};

const itemVariants = {
  hidden: { opacity: 0, y: 20 },
  visible: { opacity: 1, y: 0, transition: { duration: 0.4 } },
};

export default function SettingsScreen() {
  const [settings, setSettings] = useState<SettingsData>(DEFAULT_SETTINGS);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [initialSettings, setInitialSettings] =
    useState<SettingsData>(DEFAULT_SETTINGS);
  const [hasChanges, setHasChanges] = useState(false);

  const fetchSettings = useCallback(async () => {
    try {
      setLoading(true);
      const json = await goApi.getSettings();
      setSettings(json);
      setInitialSettings(json);
      setHasChanges(false);
    } catch {
      toast.error("Failed to load settings");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchSettings();
  }, [fetchSettings]);

  const updateField = <K extends keyof SettingsData>(
    key: K,
    value: SettingsData[K],
  ) => {
    setSettings((prev) => {
      const next = { ...prev, [key]: value };
      setHasChanges(JSON.stringify(next) !== JSON.stringify(initialSettings));
      return next;
    });
  };

  const handleSave = async () => {
    try {
      setSaving(true);
      const json = await goApi.updateSettings({
        serverPort: settings.serverPort,
        apiKeyAuthEnabled: settings.apiKeyAuthEnabled,
        defaultRoutingStrategy: settings.defaultRoutingStrategy,
        compressionEnabled: settings.compressionEnabled,
        compressionMinLength: settings.compressionMinLength,
        compressionLogSavings: settings.compressionLogSavings,
      });
      setSettings(json);
      setInitialSettings(json);
      setHasChanges(false);
      toast.success("Settings saved successfully");
    } catch {
      toast.error("Failed to save settings");
    } finally {
      setSaving(false);
    }
  };

  const handleReset = () => {
    setSettings(DEFAULT_SETTINGS);
    setHasChanges(
      JSON.stringify(DEFAULT_SETTINGS) !== JSON.stringify(initialSettings),
    );
    toast.info("Settings reset to defaults (unsaved)");
  };

  if (loading) {
    return (
      <div className="space-y-6">
        <div>
          <h1 className="text-2xl font-bold">Settings</h1>
          <p className="text-muted-foreground mt-1">
            Configure your proxy server
          </p>
        </div>
        <Card>
          <CardContent className="p-6 space-y-4">
            {Array.from({ length: 4 }).map((_, i) => (
              <div key={i} className="space-y-2">
                <Skeleton className="h-4 w-32" />
                <Skeleton className="h-10 w-full" />
              </div>
            ))}
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <motion.div
      className="space-y-6"
      variants={containerVariants}
      initial="hidden"
      animate="visible"
    >
      <motion.div variants={itemVariants}>
        <h1 className="text-2xl font-bold">Settings</h1>
        <p className="text-muted-foreground mt-1">
          Configure your proxy server
        </p>
      </motion.div>

      {/* Server Configuration */}
      <motion.div variants={itemVariants}>
        <Card>
          <CardHeader>
            <div className="flex items-center gap-2">
              <Settings className="size-5 text-emerald-600" />
              <CardTitle className="text-base">Server Configuration</CardTitle>
            </div>
            <CardDescription>
              Configure the proxy server settings
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-6 p-6 pt-0">
            {/* Server Port */}
            <div className="space-y-2">
              <Label htmlFor="serverPort">Server Port</Label>
              <Input
                id="serverPort"
                type="number"
                min={1}
                max={65535}
                value={settings.serverPort}
                onChange={(e) => updateField('serverPort', parseInt(e.target.value, 10) || 20199)}
                className="max-w-xs"
              />
              <p className="text-xs text-muted-foreground">
                Port the proxy server listens on (1–65535)
              </p>
            </div>

            {/* Default Routing Strategy */}
            <div className="space-y-2">
              <Label htmlFor="routingStrategy">Default Routing Strategy</Label>
              <Select
                value={settings.defaultRoutingStrategy}
                onValueChange={(value) =>
                  updateField("defaultRoutingStrategy", value)
                }
              >
                <SelectTrigger id="routingStrategy" className="w-full max-w-xs">
                  <SelectValue placeholder="Select strategy" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="fallback">Fallback</SelectItem>
                  <SelectItem value="round-robin">Round-Robin</SelectItem>
                </SelectContent>
              </Select>
              <p className="text-xs text-muted-foreground">
                <span className="font-medium">Fallback:</span> Try the first
                provider, fall back to the next on failure.{" "}
                <span className="font-medium">Round-Robin:</span> Rotate the
                starting model across combo requests.
              </p>
            </div>
          </CardContent>
        </Card>
      </motion.div>

      {/* Token Compression */}
      <motion.div variants={itemVariants}>
        <Card>
          <CardHeader>
            <div className="flex items-center gap-2">
              <Sparkles className="size-5 text-violet-600" />
              <CardTitle className="text-base">Token Compression</CardTitle>
            </div>
            <CardDescription>
              Detect verbose command output (git/test/ls/log/json) in tool
              results and compress before forwarding to provider. Reduces token
              cost on tool-heavy agent loops.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4 p-6 pt-0">
            <div className="flex items-center justify-between gap-4">
              <div className="space-y-0.5">
                <Label htmlFor="compEnabled">Enable compression</Label>
                <p className="text-xs text-muted-foreground">
                  Rewrites tool result content using compact,
                  semantically-equivalent forms.
                </p>
              </div>
              <Switch
                id="compEnabled"
                checked={settings.compressionEnabled}
                onCheckedChange={(v) => updateField("compressionEnabled", v)}
              />
            </div>
            <div className="flex items-center justify-between gap-4">
              <div className="space-y-0.5">
                <Label htmlFor="compLog">Log savings</Label>
                <p className="text-xs text-muted-foreground">
                  Track per-request original/compressed bytes in the logs
                  database.
                </p>
              </div>
              <Switch
                id="compLog"
                disabled={!settings.compressionEnabled}
                checked={settings.compressionLogSavings}
                onCheckedChange={(v) =>
                  updateField("compressionLogSavings", v)
                }
              />
            </div>
          </CardContent>
        </Card>
      </motion.div>

      {/* Security */}
      <motion.div variants={itemVariants}>
        <Card>
          <CardHeader>
            <div className="flex items-center gap-2">
              <ShieldAlert className="size-5 text-amber-600" />
              <CardTitle className="text-base">Security</CardTitle>
            </div>
            <CardDescription>
              Manage authentication and security settings
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4 p-6 pt-0">
            {/* API Key Auth */}
            <div className="flex items-center justify-between gap-4">
              <div className="space-y-0.5">
                <Label htmlFor="apiKeyAuth">API Key Authentication</Label>
                <p className="text-xs text-muted-foreground">
                  Require an API key for all incoming requests
                </p>
              </div>
              <Switch
                id="apiKeyAuth"
                checked={settings.apiKeyAuthEnabled}
                onCheckedChange={(checked) =>
                  updateField("apiKeyAuthEnabled", checked)
                }
              />
            </div>

            {/* Warning when disabling */}
            {initialSettings.apiKeyAuthEnabled &&
              !settings.apiKeyAuthEnabled && (
                <Alert
                  variant="destructive"
                  className="border-red-200 dark:border-red-800"
                >
                  <ShieldAlert className="size-4" />
                  <AlertDescription>
                    <strong>Warning:</strong> Disabling API key authentication
                    will allow anyone to use your proxy server without
                    credentials. This may expose your API keys and quota to
                    unauthorized access. Only disable this in trusted network
                    environments.
                  </AlertDescription>
                </Alert>
              )}

            {!initialSettings.apiKeyAuthEnabled &&
              settings.apiKeyAuthEnabled && (
                <Alert className="border-emerald-200 dark:border-emerald-800 bg-emerald-50 dark:bg-emerald-950/20">
                  <AlertDescription className="text-emerald-700 dark:text-emerald-400">
                    <strong>Good choice!</strong> API key authentication will be
                    enabled, protecting your server from unauthorized access.
                  </AlertDescription>
                </Alert>
              )}
          </CardContent>
        </Card>
      </motion.div>

      {/* Actions */}
      <motion.div
        variants={itemVariants}
        className="flex flex-col sm:flex-row gap-3 pt-2"
      >
        <Button
          onClick={handleSave}
          disabled={!hasChanges || saving}
          className="w-full sm:w-auto sm:min-w-[140px]"
        >
          {saving ? (
            <>
              <Loader2 className="size-4 animate-spin" />
              Saving...
            </>
          ) : (
            <>
              <Save className="size-4" />
              Save Settings
            </>
          )}
        </Button>
        <Button
          variant="outline"
          onClick={handleReset}
          disabled={saving}
          className="w-full sm:w-auto"
        >
          <RotateCcw className="size-4" />
          Reset to Defaults
        </Button>
      </motion.div>
    </motion.div>
  );
}
