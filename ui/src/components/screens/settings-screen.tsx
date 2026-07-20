import { useEffect, useState, useCallback } from "react";
import { motion } from "framer-motion";
import { Settings, ShieldAlert, RotateCcw, Save, Loader2, Sparkles, FileText, Send, Boxes } from "lucide-react";
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
  connectionStrategy: string;
  compressionEnabled: boolean;
  compressionMinLength: number;
  compressionLogSavings: boolean;
  logBodies: boolean;
  telegramEnabled: boolean;
  telegramBotToken: string;
  telegramOwnerID: number;
  defaultModels: Record<string, string[]>;
}

const DEFAULT_SETTINGS: SettingsData = {
  id: '',
  serverPort: 20199,
  apiKeyAuthEnabled: true,
  defaultRoutingStrategy: "fallback",
  connectionStrategy: "weighted-random",
  compressionEnabled: false,
  compressionMinLength: 500,
  compressionLogSavings: true,
  logBodies: false,
  telegramEnabled: false,
  telegramBotToken: "",
  telegramOwnerID: 0,
  defaultModels: {},
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
      // Omit requireApiKey / apiKeyAuthEnabled — auth is always enforced by BE.
      const json = await goApi.updateSettings({
        serverPort: settings.serverPort,
        defaultRoutingStrategy: settings.defaultRoutingStrategy,
        connectionStrategy: settings.connectionStrategy,
        compressionEnabled: settings.compressionEnabled,
        compressionMinLength: settings.compressionMinLength,
        compressionLogSavings: settings.compressionLogSavings,
        logBodies: settings.logBodies,
        defaultModels: settings.defaultModels,
        telegramEnabled: settings.telegramEnabled,
        telegramBotToken: settings.telegramBotToken,
        telegramOwnerID: settings.telegramOwnerID,
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

            {/* Connection Strategy */}
            <div className="space-y-2">
              <Label htmlFor="connectionStrategy">Connection Strategy</Label>
              <Select
                value={settings.connectionStrategy}
                onValueChange={(value) =>
                  updateField("connectionStrategy", value)
                }
              >
                <SelectTrigger id="connectionStrategy" className="w-full max-w-xs">
                  <SelectValue placeholder="Select connection strategy" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="weighted-random">Weighted Random</SelectItem>
                  <SelectItem value="priority-fallback">Primary First</SelectItem>
                  <SelectItem value="round-robin">Round-Robin</SelectItem>
                </SelectContent>
              </Select>
              <p className="text-xs text-muted-foreground">
                Controls account selection after a provider/model route is chosen.
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

      {/* Request Body Logging */}
      <motion.div variants={itemVariants}>
        <Card>
          <CardHeader>
            <div className="flex items-center gap-2">
              <FileText className="size-5 text-blue-600" />
              <CardTitle className="text-base">Request Body Logging</CardTitle>
            </div>
            <CardDescription>
              Store full request and response bodies in the logs database for
              debugging. Disabled by default to improve performance and reduce
              storage.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4 p-6 pt-0">
            <div className="flex items-center justify-between gap-4">
              <div className="space-y-0.5">
                <Label htmlFor="logBodies">Save request/response bodies</Label>
                <p className="text-xs text-muted-foreground">
                  When enabled, request and response payloads are persisted in
                  SQLite and viewable in log detail. May increase DB size.
                </p>
              </div>
              <Switch
                id="logBodies"
                checked={settings.logBodies}
                onCheckedChange={(v) => updateField("logBodies", v)}
              />
            </div>
          </CardContent>
        </Card>
      </motion.div>

      {/* Telegram Bot */}
      <motion.div variants={itemVariants}>
        <TelegramCard settings={settings} updateField={updateField} />
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
            <div className="space-y-1">
              <Label>API Key Authentication</Label>
              <p className="text-xs text-muted-foreground">
                Always enabled. Dashboard and /v1 API requests require a valid
                API key. Create keys under API Keys; enable Dashboard Access for
                UI login.
              </p>
            </div>
          </CardContent>
        </Card>
      </motion.div>

      {/* Default Models */}
      <motion.div variants={itemVariants}>
        <DefaultModelsCard settings={settings} updateField={updateField} />
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

// === Telegram Bot Card ===

function TelegramCard({
  settings,
  updateField,
}: {
  settings: SettingsData;
  updateField: <K extends keyof SettingsData>(key: K, value: SettingsData[K]) => void;
}) {
  const [botStatus, setBotStatus] = useState<{
    running: boolean;
    username: string;
  }>({ running: false, username: "" });
  const [testing, setTesting] = useState(false);
  const [toggling, setToggling] = useState(false);

  const fetchStatus = useCallback(async () => {
    try {
      const status = await goApi.getTelegramStatus();
      setBotStatus({ running: status.running, username: status.username || "" });
    } catch {
      // ignore
    }
  }, []);

  useEffect(() => {
    fetchStatus();
  }, [fetchStatus]);

  const handleToggleBot = async () => {
    try {
      setToggling(true);
      if (botStatus.running) {
        await goApi.stopTelegram();
        setBotStatus({ running: false, username: "" });
        toast.success("Telegram bot stopped");
      } else {
        const res = await goApi.startTelegram();
        setBotStatus({ running: true, username: res.username || "" });
        toast.success("Telegram bot started");
      }
    } catch (err: any) {
      toast.error(err?.message || "Failed to toggle bot");
    } finally {
      setToggling(false);
    }
  };

  const handleTest = async () => {
    try {
      setTesting(true);
      await goApi.testTelegram();
      toast.success("Test message sent to Telegram");
    } catch (err: any) {
      toast.error(err?.message || "Failed to send test message");
    } finally {
      setTesting(false);
    }
  };

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-2">
          <Send className="size-5 text-sky-600" />
          <CardTitle className="text-base">Telegram Bot</CardTitle>
          {botStatus.running && (
            <span className="ml-auto inline-flex items-center gap-1.5 text-xs text-emerald-600">
              <span className="size-2 rounded-full bg-emerald-500 animate-pulse" />
              @{botStatus.username}
            </span>
          )}
        </div>
        <CardDescription>
          Real-time alerts and interactive commands via Telegram. Receive
          notifications for quota exhaustion, token expiry, connection failures,
          and more.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4 p-6 pt-0">
        {/* Enable toggle */}
        <div className="flex items-center justify-between gap-4">
          <div className="space-y-0.5">
            <Label htmlFor="tgEnabled">Enable Telegram Bot</Label>
            <p className="text-xs text-muted-foreground">
              Start the bot on server launch
            </p>
          </div>
          <Switch
            id="tgEnabled"
            checked={settings.telegramEnabled}
            onCheckedChange={(v) => updateField("telegramEnabled", v)}
          />
        </div>

        {/* Bot Token */}
        <div className="space-y-2">
          <Label htmlFor="tgToken">Bot Token</Label>
          <Input
            id="tgToken"
            type="text"
            placeholder="123456:ABC-DEF..."
            value={settings.telegramBotToken}
            onChange={(e) => updateField("telegramBotToken", e.target.value)}
            className="max-w-md font-mono text-sm"
          />
          <p className="text-xs text-muted-foreground">
            Get from @BotFather on Telegram
          </p>
        </div>

        {/* Owner ID */}
        <div className="space-y-2">
          <Label htmlFor="tgOwner">Owner Chat ID</Label>
          <Input
            id="tgOwner"
            type="number"
            placeholder="123456789"
            value={settings.telegramOwnerID || ""}
            onChange={(e) => updateField("telegramOwnerID", parseInt(e.target.value, 10) || 0)}
            className="max-w-xs"
          />
          <p className="text-xs text-muted-foreground">
            Your Telegram user ID. Only this user can interact with the bot.
            Get from @userinfobot.
          </p>
        </div>

        {/* Action buttons */}
        <div className="flex gap-2 pt-2">
          <Button
            variant="outline"
            size="sm"
            onClick={handleToggleBot}
            disabled={toggling || !settings.telegramBotToken || !settings.telegramOwnerID}
          >
            {toggling ? (
              <Loader2 className="size-4 animate-spin" />
            ) : botStatus.running ? (
              "Stop Bot"
            ) : (
              "Start Bot"
            )}
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={handleTest}
            disabled={testing || !settings.telegramBotToken || !settings.telegramOwnerID}
          >
            {testing ? (
              <Loader2 className="size-4 animate-spin" />
            ) : (
              "Send Test Message"
            )}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}

// === Default Models Card ===

const PROVIDERS_WITH_MODELS = [
  { id: 'xai', label: 'Grok Build (xAI)' },
  { id: 'kiro', label: 'Kiro (AWS CodeWhisperer)' },
  { id: 'openai', label: 'OpenAI' },
  { id: 'qwen', label: 'Qwen (Alibaba)' },
  { id: 'glm', label: 'GLM (Zhipu AI)' },
  { id: 'minimax', label: 'MiniMax' },
  { id: 'anthropic', label: 'Anthropic (Claude API)' },
  { id: 'cline', label: 'ClinePass' },
  { id: 'gemini', label: 'Google Gemini' },
];

function DefaultModelsCard({
  settings,
  updateField,
}: {
  settings: SettingsData;
  updateField: <K extends keyof SettingsData>(key: K, value: SettingsData[K]) => void;
}) {
  const models = settings.defaultModels || {};

  const handleModelsChange = (providerId: string, value: string) => {
    const models = value
      .split(/[\n,]+/)
      .map((s) => s.trim())
      .filter((s) => s.length > 0);
    const next = { ...settings.defaultModels, [providerId]: models };
    if (models.length === 0) {
      delete next[providerId];
    }
    updateField('defaultModels', next);
  };

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-2">
          <Boxes className="size-5 text-orange-600" />
          <CardTitle className="text-base">Default Connection Models</CardTitle>
        </div>
        <CardDescription>
          Override which models are pre-selected when creating a new connection.
          Enter one model per line or comma-separated. Leave blank to use the
          built-in defaults.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4 p-6 pt-0">
        {PROVIDERS_WITH_MODELS.map((p) => (
          <div key={p.id} className="space-y-1.5">
            <Label className="text-xs font-medium text-muted-foreground">
              {p.label}
            </Label>
            <textarea
              className="glass-input w-full text-xs font-mono min-h-[36px]"
              rows={2}
              placeholder="(use built-in defaults)"
              value={(models[p.id] || []).join('\n')}
              onChange={(e) => handleModelsChange(p.id, e.target.value)}
            />
          </div>
        ))}
      </CardContent>
    </Card>
  );
}
