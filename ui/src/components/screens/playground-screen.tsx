import { useEffect, useMemo, useRef, useState } from "react";
import { Bot, ImageIcon, Loader2, Send, Settings2, Terminal, Trash2, Zap } from "lucide-react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
import { goApi } from "@/lib/go-api";

import type { Model, Connection, Message, RequestLog, ChatParams, Attachment } from "./playground/types";
import { PlaygroundParamsPanel } from "./playground/PlaygroundParamsPanel";
import { ModelSelector } from "./playground/model-selector";
import { AttachmentInput } from "./playground/attachment-input";
import { ChatView } from "./playground/chat-view";
import { PlaygroundLogView } from "./playground/log-view";
import { ImageGenerator } from "./playground/image-generator";
import { useChatQueue } from "./playground/use-chat-queue";
import { usePlaygroundModelSelection } from "./playground/use-playground-model-selection";

const DEFAULT_PARAMS: ChatParams = { temperature: 1, topP: 1, maxTokens: 4096, systemPrompt: "" };

export default function PlaygroundScreen() {
  const [models, setModels] = useState<Model[]>([]);
  const [connections, setConnections] = useState<Connection[]>([]);
  const [loadingModels, setLoadingModels] = useState(true);

  const [input, setInput] = useState("");
  const [attachments, setAttachments] = useState<Attachment[]>([]);
  const [params, setParams] = useState<ChatParams>(DEFAULT_PARAMS);

  const [requestLogs, setRequestLogs] = useState<RequestLog[]>([]);
  const [selectedLogId, setSelectedLogId] = useState<string | null>(null);
  const [showParams, setShowParams] = useState(false);
	  const [activeTab, setActiveTab] = useState("chat");
	  const scrollAnchorRef = useRef<HTMLDivElement | null>(null);

	  // Filter: chat models exclude image-generation-only models
	  const chatModels = useMemo(() => models.filter((m) => {
	    const caps = m.capabilities;
	    if (!Array.isArray(caps) || caps.length === 0) return true;
	    // If the only capability is image-generation, hide from chat
	    if (caps.length === 1 && caps[0] === "image-generation") return false;
	    return true;
	  }), [models]);

	  const {
    selectedProvider,
    selectedModel,
    selectedAccount,
    setSelectedModel,
    setSelectedAccount,
    finalModelString,
    supportsImages,
	    initializeSelection,
	    handleProviderChange,
	  } = usePlaygroundModelSelection(chatModels);

  const { messages, sending, queuedCount, streamingId, enqueueTurn, clearQueue } = useChatQueue((log) => {
    setRequestLogs((prev) => [log, ...prev].slice(0, 50));
  });

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
        setConnections(Array.isArray(connectionsData) ? connectionsData.filter((c: Connection) => c.isActive) : []);
        initializeSelection(availableModels);
      } catch {
        toast.error("Failed to load models and connections");
      } finally {
        setLoadingModels(false);
      }
    }
    loadData();
  }, []);

  useEffect(() => {
    scrollAnchorRef.current?.scrollIntoView({ behavior: sending ? "auto" : "smooth" });
  }, [messages, sending]);

  const sendingLockRef = useRef(false);

  const handleSend = () => {
    const content = input.trim();
    if ((!content && attachments.length === 0) || !finalModelString) return;
    if (sendingLockRef.current) return;
    if (attachments.length > 0 && !supportsImages) {
      toast.error("Selected model does not support image input");
      return;
    }
    sendingLockRef.current = true;

    const userMessage: Message = {
      id: crypto.randomUUID(),
      role: "user",
      content,
      attachments: attachments.length > 0 ? [...attachments] : undefined,
    };

    setInput("");
    setAttachments([]);
    enqueueTurn(userMessage, finalModelString, params);
    window.setTimeout(() => {
      sendingLockRef.current = false;
    }, 200);
  };

  const handleClear = () => {
    clearQueue();
    setAttachments([]);
    setSelectedLogId(null);
  };

  return (
    <div className="flex h-full flex-col">
      {/* Shared header */}
      <div className="flex items-center justify-between border-b bg-background/95 px-4 py-2.5 backdrop-blur-sm md:px-6">
        <div className="flex items-center gap-2">
          <Bot className="h-5 w-5 text-emerald-600" />
          <div>
            <h1 className="text-lg font-semibold">Playground</h1>
            <p className="text-xs text-muted-foreground">Test chat, generate images, inspect requests</p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          {activeTab === "chat" && (
            <>
              <TooltipProvider>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button aria-label="Toggle parameters" variant="outline" size="icon" onClick={() => setShowParams(!showParams)} className={showParams ? "bg-accent" : ""}>
                      <Settings2 className="h-4 w-4" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent><p>Toggle parameters</p></TooltipContent>
                </Tooltip>
              </TooltipProvider>
              <Button aria-label="Clear chat" variant="outline" size="icon" onClick={handleClear}><Trash2 className="h-4 w-4" /></Button>
            </>
          )}
        </div>
      </div>

      <Tabs value={activeTab} onValueChange={setActiveTab} className="flex-1 flex flex-col min-h-0">
        <div className="px-4 md:px-6 border-b">
          <TabsList className="h-8">
            <TabsTrigger value="chat" className="text-xs gap-1"><Bot className="h-3 w-3" />Chat</TabsTrigger>
            <TabsTrigger value="images" className="text-xs gap-1"><ImageIcon className="h-3 w-3" />Images</TabsTrigger>
            <TabsTrigger value="logs" className="text-xs gap-1">
              <Terminal className="h-3 w-3" />Logs
              {requestLogs.length > 0 && <Badge variant="secondary" className="ml-1 text-[10px] h-4 px-1">{requestLogs.length}</Badge>}
            </TabsTrigger>
          </TabsList>
        </div>

        <TabsContent value="chat" className="flex-1 flex flex-col min-h-0 m-0">
          {/* Chat-specific header: model selector + params */}
          <div className="flex flex-col gap-2 border-b bg-background/95 px-4 py-2.5 md:px-6">
            <ModelSelector
              models={chatModels}
              connections={connections}
              selectedProvider={selectedProvider}
              selectedModel={selectedModel}
              selectedAccount={selectedAccount}
              onProviderChange={handleProviderChange}
              onModelChange={setSelectedModel}
              onAccountChange={setSelectedAccount}
              disabled={loadingModels}
            />
            {showParams && <PlaygroundParamsPanel params={params} setParams={setParams} defaultParams={DEFAULT_PARAMS} />}
          </div>
          <ScrollArea className="flex-1">
            <div className="mx-auto flex max-w-3xl flex-col gap-4 p-4 md:p-6">
              {messages.length === 0 ? (
                <div className="flex min-h-[45vh] flex-col items-center justify-center gap-4 text-center">
                  <div className="flex h-16 w-16 items-center justify-center rounded-full bg-emerald-100 dark:bg-emerald-950"><Bot className="h-8 w-8 text-emerald-600" /></div>
                  <div>
                    <h2 className="text-xl font-semibold">Choose a model & send a real request</h2>
                    <p className="mt-2 max-w-md text-sm text-muted-foreground">Select a model from any provider, customize parameters, and test routing with real chat prompts.</p>
                  </div>
                  {finalModelString && <div className="flex items-center gap-2"><Badge variant="secondary">{finalModelString}</Badge><Zap className="h-4 w-4 text-emerald-600" /></div>}
                </div>
              ) : (
                <ChatView messages={messages} streamingId={streamingId} />
              )}
              <div ref={scrollAnchorRef} />
            </div>
          </ScrollArea>

          <div className="border-t bg-background/95 px-4 py-3 backdrop-blur-sm md:px-6">
            <div className="mx-auto max-w-3xl space-y-2">
              <AttachmentInput attachments={attachments} onAttachmentsChange={setAttachments} disabled={!finalModelString || !supportsImages} />
              {!supportsImages && (
                <p className="text-xs text-muted-foreground">Image input is disabled for this model.</p>
              )}
              {(sending || queuedCount > 0) && (
                <div className="flex items-center gap-2 text-xs text-muted-foreground">
                  {sending && <span>Streaming current request</span>}
                  {queuedCount > 0 && <Badge variant="secondary">{queuedCount} queued</Badge>}
                </div>
              )}
              <div className="flex gap-2">
                <Textarea
                  aria-label="Chat message"
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
                  disabled={!finalModelString}
                />
                <Button aria-label={sending ? "Queue message" : "Send message"} onClick={handleSend} disabled={(!input.trim() && attachments.length === 0) || !finalModelString} size="icon" className="h-[44px] w-[44px] shrink-0 rounded-xl">
                  {sending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}
                </Button>
              </div>
            </div>
          </div>
        </TabsContent>

        <TabsContent value="images" className="flex-1 flex flex-col min-h-0 m-0">
          <ImageGenerator />
        </TabsContent>

        <TabsContent value="logs" className="flex-1 flex flex-col min-h-0 m-0">
          <PlaygroundLogView logs={requestLogs} selectedLogId={selectedLogId} onSelectLog={setSelectedLogId} onClear={() => { setRequestLogs([]); setSelectedLogId(null); toast.success("Logs cleared"); }} />
        </TabsContent>
      </Tabs>
    </div>
  );
}
