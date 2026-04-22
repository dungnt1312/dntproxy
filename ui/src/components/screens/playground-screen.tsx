import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  Bot,
  ChevronDown,
  ChevronRight,
  Clock,
  Copy,
  Eraser,
  Loader2,
  Send,
  Settings2,
  Shield,
  Terminal,
  Trash2,
  Zap,
} from "lucide-react";
import ReactMarkdown from "react-markdown";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import { Label } from "@/components/ui/label";
import { ScrollArea } from "@/components/ui/scroll-area";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { Slider } from "@/components/ui/slider";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { goApi, goStreamFetch } from "@/lib/go-api";

// ─── Types ───────────────────────────────────────────────────────────────────

import type {
  Model,
  Connection,
  Message,
  RequestLog,
  ChatParams,
  Attachment,
} from "./playground/types";
import { PlaygroundParamsPanel } from "./playground/PlaygroundParamsPanel";
import { ModelSelector } from "./playground/model-selector";
import { AttachmentInput } from "./playground/attachment-input";

type ChatApiMessage = {
  role: "user" | "assistant" | "system";
  content: string | Array<{ type: string; text?: string; image_url?: { url: string } }>;
};

// ─── Constants ───────────────────────────────────────────────────────────────

const DEFAULT_PARAMS: ChatParams = {
  temperature: 1,
  topP: 1,
  maxTokens: 4096,
  systemPrompt: "",
};

// ─── Helpers ─────────────────────────────────────────────────────────────────

function extractDeltaContent(line: string): string {
  if (!line.startsWith("data:")) return "";
  const payload = line.slice(5).trim();
  if (!payload || payload === "[DONE]") return "";

  try {
    const parsed = JSON.parse(payload);
    const choice = parsed?.choices?.[0];
    return choice?.delta?.content || choice?.message?.content || "";
  } catch {
    return "";
  }
}

function extractUsage(payload: string) {
  try {
    const parsed = JSON.parse(payload);
    return parsed?.usage || null;
  } catch {
    return null;
  }
}

function formatDuration(ms?: number): string {
  if (!ms) return "-";
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(2)}s`;
}

function formatCost(value?: number): string {
  if (!value) return "-";
  return new Intl.NumberFormat(undefined, {
    style: "currency",
    currency: "USD",
    maximumFractionDigits: 6,
  }).format(value);
}

function prettyJSON(raw: string): string {
  try {
    return JSON.stringify(JSON.parse(raw), null, 2);
  } catch {
    return raw;
  }
}

function copyToClipboard(text: string) {
  navigator.clipboard.writeText(text);
  toast.success("Copied to clipboard");
}

// ─── Provider icon mapping ───────────────────────────────────────────────────

function ProviderBadge({ provider }: { provider: string }) {
  const colors: Record<string, string> = {
    kiro: "bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300",
    openai: "bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300",
    codex:
      "bg-purple-100 text-purple-700 dark:bg-purple-900 dark:text-purple-300",
    combo: "bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300",
    alias: "bg-pink-100 text-pink-700 dark:bg-pink-900 dark:text-pink-300",
  };
  const color =
    colors[provider.toLowerCase()] ||
    "bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-300";

  return (
    <Badge variant="secondary" className={`text-xs ${color}`}>
      {provider}
    </Badge>
  );
}

// ─── Main Component ──────────────────────────────────────────────────────────

export default function PlaygroundScreen() {
  // Data
  const [models, setModels] = useState<Model[]>([]);
  const [connections, setConnections] = useState<Connection[]>([]);
  const [loadingModels, setLoadingModels] = useState(true);

  // Selection
  const [selectedProvider, setSelectedProvider] = useState("");
  const [selectedModel, setSelectedModel] = useState("");
  const [selectedAccount, setSelectedAccount] = useState("auto");

  // Chat
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState("");
  const [attachments, setAttachments] = useState<Attachment[]>([]);
  const [sending, setSending] = useState(false);
  const [params, setParams] = useState<ChatParams>(DEFAULT_PARAMS);

  // Logs
  const [requestLogs, setRequestLogs] = useState<RequestLog[]>([]);
  const [selectedLogId, setSelectedLogId] = useState<string | null>(null);

  // UI state
  const [showParams, setShowParams] = useState(false);
  const [activeTab, setActiveTab] = useState("chat");

  const abortControllerRef = useRef<AbortController | null>(null);
  const scrollAnchorRef = useRef<HTMLDivElement | null>(null);
  const pendingAssistantDeltaRef = useRef("");
  const deltaFlushTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const clearDeltaFlushTimer = useCallback(() => {
    if (!deltaFlushTimerRef.current) return;
    clearTimeout(deltaFlushTimerRef.current);
    deltaFlushTimerRef.current = null;
  }, []);

  const flushAssistantDelta = useCallback((assistantId: string) => {
    clearDeltaFlushTimer();
    const delta = pendingAssistantDeltaRef.current;
    if (!delta) return;

    pendingAssistantDeltaRef.current = "";
    setMessages((current) =>
      current.map((message) =>
        message.id === assistantId
          ? { ...message, content: message.content + delta }
          : message,
      ),
    );
  }, [clearDeltaFlushTimer]);

  useEffect(() => {
    return () => {
      clearDeltaFlushTimer();
      pendingAssistantDeltaRef.current = "";
    };
  }, [clearDeltaFlushTimer]);

  // ─── Load data ───────────────────────────────────────────────────────────

  useEffect(() => {
    async function loadData() {
      setLoadingModels(true);
      try {
        const [modelsData, connectionsData] = await Promise.all([
          goApi.getModels().catch(() => []),
          goApi.getConnections().catch(() => []),
        ]);

        const availableModels = Array.isArray(modelsData) ? modelsData : [];
        setModels(availableModels);

        const availableConnections = Array.isArray(connectionsData)
          ? connectionsData.filter((c: Connection) => c.isActive)
          : [];
        setConnections(availableConnections);

        // Auto-select first model
        if (availableModels.length > 0) {
          const firstModel = availableModels[0];
          setSelectedProvider(firstModel.provider);
          // Strip provider prefix from model ID
          const modelName = firstModel.id.includes('/') 
            ? firstModel.id.split('/').slice(1).join('/')
            : firstModel.id;
          setSelectedModel(modelName);
        }
      } catch {
        toast.error("Failed to load models and connections");
      } finally {
        setLoadingModels(false);
      }
    }

    loadData();
  }, []);

  // Auto-scroll
  useEffect(() => {
    scrollAnchorRef.current?.scrollIntoView({ behavior: sending ? "auto" : "smooth" });
  }, [messages.length, sending]);

  // Build final model string for API: "provider/model@account"
  const finalModelString = useMemo(() => {
    if (!selectedProvider || !selectedModel) return '';
    
    const base = `${selectedProvider}/${selectedModel}`;
    if (selectedAccount && selectedAccount !== 'auto') {
      return `${base}@${selectedAccount}`;
    }
    return base;
  }, [selectedProvider, selectedModel, selectedAccount]);

  // ─── Send message ────────────────────────────────────────────────────────

  async function handleSend() {
    const content = input.trim();
    if ((!content && attachments.length === 0) || sending || !finalModelString) return;

    const userMessage: Message = {
      id: crypto.randomUUID(),
      role: "user",
      content,
      attachments: attachments.length > 0 ? [...attachments] : undefined,
    };
    const assistantId = crypto.randomUUID();

    setMessages((current) => [
      ...current,
      userMessage,
      { id: assistantId, role: "assistant", content: "" },
    ]);
    setInput("");
    setAttachments([]);
    setSending(true);

    // Build messages array with optional system prompt
    const nextMessages: ChatApiMessage[] = [];
    if (params.systemPrompt.trim()) {
      nextMessages.push({
        role: "system",
        content: params.systemPrompt.trim(),
      });
    }
    
    // Convert previous messages
    nextMessages.push(
      ...messages
        .filter((m) => m.role !== "system")
        .map((m) => {
          if (m.attachments && m.attachments.length > 0) {
            // Multimodal message
            const contentParts: Array<{ type: string; text?: string; image_url?: { url: string } }> = [];
            if (m.content) {
              contentParts.push({ type: "text", text: m.content });
            }
            m.attachments.forEach((att) => {
              contentParts.push({
                type: "image_url",
                image_url: { url: att.dataUrl },
              });
            });
            return {
              role: m.role,
              content: contentParts,
            };
          }
          return {
            role: m.role,
            content: m.content,
          };
        })
    );

    // Add current user message
    if (userMessage.attachments && userMessage.attachments.length > 0) {
      const contentParts: Array<{ type: string; text?: string; image_url?: { url: string } }> = [];
      if (userMessage.content) {
        contentParts.push({ type: "text", text: userMessage.content });
      }
      userMessage.attachments.forEach((att) => {
        contentParts.push({
          type: "image_url",
          image_url: { url: att.dataUrl },
        });
      });
      nextMessages.push({
        role: userMessage.role,
        content: contentParts,
      });
    } else {
      nextMessages.push({
        role: userMessage.role,
        content: userMessage.content,
      });
    }

    const requestBody = {
      model: finalModelString,
      stream: true,
      messages: nextMessages,
      temperature: params.temperature,
      top_p: params.topP,
      max_tokens: params.maxTokens > 0 ? params.maxTokens : undefined,
    };

    const logId = crypto.randomUUID();
    const logEntry: RequestLog = {
      id: logId,
      timestamp: new Date().toISOString(),
      model: finalModelString,
      status: "success",
      requestBody: JSON.stringify(requestBody, null, 2),
    };

    const controller = new AbortController();
    abortControllerRef.current = controller;

    const startTime = Date.now();

    try {
      const response = await goStreamFetch("/v1/chat/completions", {
        method: "POST",
        body: JSON.stringify(requestBody),
        signal: controller.signal,
      });

      if (!response.ok) {
        const errorBody = await response.json().catch(() => null);
        throw new Error(errorBody?.error?.message || response.statusText);
      }

      if (!response.body) {
        throw new Error("No response stream available");
      }

      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let buffer = "";
      let fullResponse = "";
      let lastUsage: { prompt_tokens?: number; completion_tokens?: number; total_tokens?: number } | null = null;

      while (true) {
        const { value, done } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split("\n");
        buffer = lines.pop() || "";

        for (const line of lines) {
          const delta = extractDeltaContent(line);
          if (delta) {
            fullResponse += delta;
            pendingAssistantDeltaRef.current += delta;
            if (!deltaFlushTimerRef.current) {
              deltaFlushTimerRef.current = setTimeout(() => {
                flushAssistantDelta(assistantId);
              }, 48);
            }
          }

          // Extract usage if available
          const usage = extractUsage(line.slice(5).trim());
          if (usage) lastUsage = usage;
        }
      }

      const duration = Date.now() - startTime;
      flushAssistantDelta(assistantId);
      logEntry.responseBody = fullResponse;
      logEntry.durationMs = duration;
      logEntry.status = "success";
      logEntry.statusCode = response.status;

      if (lastUsage) {
        logEntry.inputTokens = lastUsage.prompt_tokens;
        logEntry.outputTokens = lastUsage.completion_tokens;
        logEntry.totalTokens = lastUsage.total_tokens;
      }

      setRequestLogs((prev) => [logEntry, ...prev].slice(0, 50)); // Keep last 50 logs
    } catch (error) {
      flushAssistantDelta(assistantId);
      const message =
        error instanceof Error ? error.message : "Chat request failed";
      const duration = Date.now() - startTime;

      logEntry.status = "error";
      logEntry.error = message;
      logEntry.durationMs = duration;

      setMessages((current) =>
        current.map((item) =>
          item.id === assistantId
            ? { ...item, content: `Request failed: ${message}` }
            : item,
        ),
      );
      setRequestLogs((prev) => [logEntry, ...prev].slice(0, 50));
      toast.error(message);
    } finally {
      clearDeltaFlushTimer();
      pendingAssistantDeltaRef.current = "";
      abortControllerRef.current = null;
      setSending(false);
    }
  }

  function handleClear() {
    abortControllerRef.current?.abort();
    abortControllerRef.current = null;
    setSending(false);
    setMessages([]);
    setAttachments([]);
    setSelectedLogId(null);
  }

  function handleClearLogs() {
    setRequestLogs([]);
    setSelectedLogId(null);
    toast.success("Logs cleared");
  }

  // ─── Render ──────────────────────────────────────────────────────────────

  return (
    <div className="flex h-full flex-col">
      {/* Header */}
      <div className="flex flex-col gap-3 border-b bg-background/95 px-4 py-3 backdrop-blur-sm md:px-6">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Bot className="h-5 w-5 text-emerald-600" />
            <div>
              <h1 className="text-lg font-semibold">Playground</h1>
              <p className="text-xs text-muted-foreground">
                Test models, inspect requests, and debug responses
              </p>
            </div>
          </div>

          <div className="flex items-center gap-2">
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="outline"
                    size="icon"
                    onClick={() => setShowParams(!showParams)}
                    className={showParams ? "bg-accent" : ""}
                  >
                    <Settings2 className="h-4 w-4" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>
                  <p>Toggle parameters</p>
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>

            <Button variant="outline" size="icon" onClick={handleClear}>
              <Trash2 className="h-4 w-4" />
            </Button>
          </div>
        </div>

        {/* Model Selection */}
        <ModelSelector
          models={models}
          connections={connections}
          selectedProvider={selectedProvider}
          selectedModel={selectedModel}
          selectedAccount={selectedAccount}
          onProviderChange={setSelectedProvider}
          onModelChange={setSelectedModel}
          onAccountChange={setSelectedAccount}
          disabled={loadingModels}
        />

        {/* Parameters panel */}
        {showParams && (
          <PlaygroundParamsPanel 
            params={params} 
            setParams={setParams} 
            defaultParams={DEFAULT_PARAMS} 
          />
        )}
      </div>

      {/* Tabs */}
      <Tabs
        value={activeTab}
        onValueChange={setActiveTab}
        className="flex-1 flex flex-col min-h-0"
      >
        <div className="px-4 md:px-6 border-b">
          <TabsList className="h-8">
            <TabsTrigger value="chat" className="text-xs gap-1">
              <Bot className="h-3 w-3" />
              Chat
            </TabsTrigger>
            <TabsTrigger value="logs" className="text-xs gap-1">
              <Terminal className="h-3 w-3" />
              Logs
              {requestLogs.length > 0 && (
                <Badge
                  variant="secondary"
                  className="ml-1 text-[10px] h-4 px-1"
                >
                  {requestLogs.length}
                </Badge>
              )}
            </TabsTrigger>
          </TabsList>
        </div>

        {/* Chat Tab */}
        <TabsContent value="chat" className="flex-1 flex flex-col min-h-0 m-0">
          <ScrollArea className="flex-1">
            <div className="mx-auto flex max-w-3xl flex-col gap-4 p-4 md:p-6">
              {messages.length === 0 ? (
                <div className="flex min-h-[45vh] flex-col items-center justify-center gap-4 text-center">
                  <div className="flex h-16 w-16 items-center justify-center rounded-full bg-emerald-100 dark:bg-emerald-950">
                    <Bot className="h-8 w-8 text-emerald-600" />
                  </div>
                  <div>
                    <h2 className="text-xl font-semibold">
                      Choose a model & send a real request
                    </h2>
                    <p className="mt-2 max-w-md text-sm text-muted-foreground">
                      Select a model from any provider, customize parameters,
                      and test routing with real chat prompts.
                    </p>
                  </div>
                  {finalModelString && (
                    <div className="flex items-center gap-2">
                      <Badge variant="secondary">{finalModelString}</Badge>
                      <Zap className="h-4 w-4 text-emerald-600" />
                    </div>
                  )}
                </div>
              ) : (
                messages.map((message) => (
                  <div
                    key={message.id}
                    className={`flex ${
                      message.role === "user" ? "justify-end" : "justify-start"
                    }`}
                  >
                    <div
                      className={`max-w-[88%] rounded-xl px-4 py-3 text-sm ${
                        message.role === "system"
                          ? "bg-yellow-50 text-yellow-900 dark:bg-yellow-950 dark:text-yellow-100 border border-yellow-200 dark:border-yellow-800"
                          : message.role === "user"
                            ? "bg-primary text-primary-foreground"
                            : "bg-muted text-foreground"
                      }`}
                    >
                      {message.role === "assistant" ? (
                        message.content ? (
                          <div className="prose prose-sm max-w-none dark:prose-invert">
                            <ReactMarkdown>{message.content}</ReactMarkdown>
                          </div>
                        ) : (
                          <div className="flex items-center gap-2 text-muted-foreground">
                            <Loader2 className="h-4 w-4 animate-spin" />
                            <span>Streaming…</span>
                          </div>
                        )
                      ) : message.role === "system" ? (
                        <div className="flex items-start gap-2">
                          <Shield className="h-4 w-4 mt-0.5 shrink-0" />
                          <p className="whitespace-pre-wrap text-xs">
                            {message.content}
                          </p>
                        </div>
                      ) : (
                        <>
                          {message.attachments && message.attachments.length > 0 && (
                            <div className="flex flex-wrap gap-2 mb-2">
                              {message.attachments.map((att) => (
                                <img
                                  key={att.id}
                                  src={att.dataUrl}
                                  alt={att.name}
                                  className="max-w-[200px] max-h-[200px] rounded-lg border"
                                />
                              ))}
                            </div>
                          )}
                          <p className="whitespace-pre-wrap">{message.content}</p>
                        </>
                      )}
                    </div>
                  </div>
                ))
              )}
              <div ref={scrollAnchorRef} />
            </div>
          </ScrollArea>

          {/* Input area */}
          <div className="border-t bg-background/95 px-4 py-3 backdrop-blur-sm md:px-6">
            <div className="mx-auto max-w-3xl space-y-2">
              <AttachmentInput
                attachments={attachments}
                onAttachmentsChange={setAttachments}
                disabled={sending || !finalModelString}
              />
              
              <div className="flex gap-2">
                <Textarea
                  value={input}
                  onChange={(event) => setInput(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === "Enter" && !event.shiftKey) {
                      event.preventDefault();
                      handleSend();
                    }
                  }}
                  placeholder="Type your message…"
                  className="min-h-[44px] resize-none rounded-xl"
                  rows={1}
                  disabled={sending || !finalModelString}
                />
                <Button
                  onClick={handleSend}
                  disabled={sending || (!input.trim() && attachments.length === 0) || !finalModelString}
                  size="icon"
                  className="h-[44px] w-[44px] shrink-0 rounded-xl"
                >
                  {sending ? (
                    <Loader2 className="h-4 w-4 animate-spin" />
                  ) : (
                    <Send className="h-4 w-4" />
                  )}
                </Button>
              </div>
            </div>
          </div>
        </TabsContent>

        {/* Logs Tab */}
        <TabsContent value="logs" className="flex-1 flex flex-col min-h-0 m-0">
          <div className="flex items-center justify-between px-4 py-2 border-b">
            <p className="text-xs text-muted-foreground">
              {requestLogs.length} request{requestLogs.length !== 1 ? "s" : ""}{" "}
              logged
            </p>
            {requestLogs.length > 0 && (
              <Button
                variant="outline"
                size="sm"
                onClick={handleClearLogs}
                className="text-xs h-7"
              >
                <Trash2 className="mr-1 h-3 w-3" />
                Clear
              </Button>
            )}
          </div>

          {requestLogs.length === 0 ? (
            <div className="flex flex-1 flex-col items-center justify-center gap-3 text-center p-8">
              <Terminal className="h-12 w-12 text-muted-foreground/50" />
              <div>
                <p className="font-medium">No logs yet</p>
                <p className="text-sm text-muted-foreground mt-1">
                  Send a chat request to see detailed logs here
                </p>
              </div>
            </div>
          ) : (
            <div className="flex flex-1 min-h-0">
              {/* Log list */}
              <ScrollArea className="flex-1 max-w-md border-r">
                <div className="p-2 space-y-2">
                  {requestLogs.map((log) => (
                    <button
                      key={log.id}
                      onClick={() => setSelectedLogId(log.id)}
                      className={`w-full text-left p-3 rounded-lg border transition-colors ${
                        selectedLogId === log.id
                          ? "border-emerald-500 bg-emerald-50 dark:bg-emerald-950/30"
                          : "hover:bg-accent/50"
                      }`}
                    >
                      <div className="flex items-center justify-between mb-1">
                        <Badge
                          variant={
                            log.status === "success" ? "default" : "destructive"
                          }
                          className="text-[10px] h-4"
                        >
                          {log.status === "success" ? "200 OK" : "Error"}
                        </Badge>
                        <div className="flex items-center gap-1 text-xs text-muted-foreground">
                          <Clock className="h-3 w-3" />
                          {formatDuration(log.durationMs)}
                        </div>
                      </div>

                      <p className="text-xs font-medium truncate">
                        {log.model}
                      </p>

                      <div className="flex items-center gap-2 mt-1 text-[10px] text-muted-foreground">
                        <span>
                          {new Date(log.timestamp).toLocaleTimeString()}
                        </span>
                        {log.totalTokens && (
                          <>
                            <span>·</span>
                            <span>{log.totalTokens} tokens</span>
                          </>
                        )}
                        {log.costTotal && (
                          <>
                            <span>·</span>
                            <span>{formatCost(log.costTotal)}</span>
                          </>
                        )}
                      </div>
                    </button>
                  ))}
                </div>
              </ScrollArea>

              {/* Log detail */}
              <ScrollArea className="flex-1">
                {selectedLogId ? (
                  (() => {
                    const log = requestLogs.find((l) => l.id === selectedLogId);
                    if (!log) return null;

                    return (
                      <div className="p-4 space-y-4">
                        {/* Header */}
                        <div className="flex items-center justify-between">
                          <div className="space-y-1">
                            <div className="flex items-center gap-2">
                              <Badge
                                variant={
                                  log.status === "success"
                                    ? "default"
                                    : "destructive"
                                }
                              >
                                {log.status === "success"
                                  ? "Success"
                                  : "Failed"}
                              </Badge>
                              <span className="text-xs font-mono">
                                {log.model}
                              </span>
                            </div>
                            <p className="text-xs text-muted-foreground">
                              {new Date(log.timestamp).toLocaleString()} ·{" "}
                              {formatDuration(log.durationMs)}
                            </p>
                          </div>

                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() =>
                              copyToClipboard(JSON.stringify(log, null, 2))
                            }
                            className="text-xs"
                          >
                            <Copy className="mr-1 h-3 w-3" />
                            Copy
                          </Button>
                        </div>

                        {/* Stats */}
                        {log.totalTokens && (
                          <div className="grid grid-cols-3 gap-3">
                            <Card>
                              <CardContent className="pt-4 text-center">
                                <p className="text-2xl font-bold">
                                  {log.inputTokens || "-"}
                                </p>
                                <p className="text-xs text-muted-foreground">
                                  Input tokens
                                </p>
                              </CardContent>
                            </Card>
                            <Card>
                              <CardContent className="pt-4 text-center">
                                <p className="text-2xl font-bold">
                                  {log.outputTokens || "-"}
                                </p>
                                <p className="text-xs text-muted-foreground">
                                  Output tokens
                                </p>
                              </CardContent>
                            </Card>
                            <Card>
                              <CardContent className="pt-4 text-center">
                                <p className="text-2xl font-bold">
                                  {formatCost(log.costTotal)}
                                </p>
                                <p className="text-xs text-muted-foreground">
                                  Cost
                                </p>
                              </CardContent>
                            </Card>
                          </div>
                        )}

                        {/* Request body */}
                        <Collapsible defaultOpen>
                          <CollapsibleTrigger className="flex items-center gap-2 w-full text-sm font-medium">
                            <ChevronDown className="h-4 w-4" />
                            Request Body
                          </CollapsibleTrigger>
                          <CollapsibleContent className="mt-2">
                            <div className="relative">
                              <pre className="bg-muted p-3 rounded-lg text-xs overflow-x-auto max-h-[400px]">
                                {prettyJSON(log.requestBody)}
                              </pre>
                              <Button
                                variant="ghost"
                                size="sm"
                                className="absolute top-2 right-2 h-6 w-6 p-0"
                                onClick={() => copyToClipboard(log.requestBody)}
                              >
                                <Copy className="h-3 w-3" />
                              </Button>
                            </div>
                          </CollapsibleContent>
                        </Collapsible>

                        {/* Response body */}
                        {log.responseBody && (
                          <Collapsible>
                            <CollapsibleTrigger className="flex items-center gap-2 w-full text-sm font-medium">
                              <ChevronRight className="h-4 w-4" />
                              Response
                            </CollapsibleTrigger>
                            <CollapsibleContent className="mt-2">
                              <div className="relative">
                                <pre className="bg-muted p-3 rounded-lg text-xs overflow-x-auto max-h-[400px]">
                                  {prettyJSON(log.responseBody)}
                                </pre>
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  className="absolute top-2 right-2 h-6 w-6 p-0"
                                  onClick={() =>
                                    copyToClipboard(log.responseBody || "")
                                  }
                                >
                                  <Copy className="h-3 w-3" />
                                </Button>
                              </div>
                            </CollapsibleContent>
                          </Collapsible>
                        )}

                        {/* Error */}
                        {log.error && (
                          <Card className="border-destructive">
                            <CardContent className="pt-4">
                              <div className="flex items-start gap-2">
                                <Badge variant="destructive">Error</Badge>
                                <pre className="text-xs text-destructive whitespace-pre-wrap">
                                  {log.error}
                                </pre>
                              </div>
                            </CardContent>
                          </Card>
                        )}
                      </div>
                    );
                  })()
                ) : (
                  <div className="flex flex-1 items-center justify-center text-muted-foreground text-sm">
                    Select a log to view details
                  </div>
                )}
              </ScrollArea>
            </div>
          )}
        </TabsContent>
      </Tabs>
    </div>
  );
}
