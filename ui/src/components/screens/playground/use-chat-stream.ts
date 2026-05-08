import { useCallback, useRef } from "react";
import { goStreamFetch } from "@/lib/go-api";
import { toast } from "sonner";
import type { ChatParams, Message, RequestLog, Attachment } from "./types";
import { buildApiMessages, extractDeltaContent, extractUsage } from "./stream-utils";

export function useChatStream(
  finalModelString: string,
  params: ChatParams,
  onLog: (log: RequestLog) => void,
) {
  const pendingDeltaRef = useRef("");
  const deltaTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const abortRef = useRef<AbortController | null>(null);

  const clearTimer = useCallback(() => {
    if (deltaTimerRef.current) { clearTimeout(deltaTimerRef.current); deltaTimerRef.current = null; }
  }, []);

  const flushDelta = useCallback(
    (assistantId: string, setter: (fn: (msgs: Message[]) => Message[]) => void) => {
      clearTimer();
      const delta = pendingDeltaRef.current;
      if (!delta) return;
      pendingDeltaRef.current = "";
      setter((current) =>
        current.map((m) => (m.id === assistantId ? { ...m, content: m.content + delta } : m)),
      );
    },
    [clearTimer],
  );

  const send = useCallback(
    async (
      input: string,
      attachments: Attachment[],
      messages: Message[],
      setMessages: (fn: (msgs: Message[]) => Message[]) => void,
      setSending: (v: boolean) => void,
      setInput: (v: string) => void,
      setAttachments: (v: Attachment[]) => void,
    ) => {
      const content = input.trim();
      if ((!content && attachments.length === 0) || !finalModelString) return;

      const userMessage: Message = {
        id: crypto.randomUUID(),
        role: "user",
        content,
        attachments: attachments.length > 0 ? [...attachments] : undefined,
      };
      const assistantId = crypto.randomUUID();

      setMessages((prev) => [...prev, userMessage, { id: assistantId, role: "assistant", content: "" }]);
      setInput("");
      setAttachments([]);

      const apiMessages = buildApiMessages(messages, userMessage, params.systemPrompt);
      const requestBody = {
        model: finalModelString,
        stream: true,
        messages: apiMessages,
        temperature: params.temperature,
        top_p: params.topP,
        max_tokens: params.maxTokens > 0 ? params.maxTokens : undefined,
      };

      const logEntry: RequestLog = {
        id: crypto.randomUUID(),
        timestamp: new Date().toISOString(),
        model: finalModelString,
        status: "success",
        requestBody: JSON.stringify(requestBody, null, 2),
      };

      const controller = new AbortController();
      abortRef.current = controller;
      setSending(true);
      const startTime = Date.now();

      try {
        const response = await goStreamFetch("/v1/chat/completions", {
          method: "POST",
          body: JSON.stringify(requestBody),
          signal: controller.signal,
        });

        if (!response.ok) {
          const errBody = await response.json().catch(() => null);
          throw new Error(errBody?.error?.message || response.statusText);
        }
        if (!response.body) throw new Error("No response stream available");

        const reader = response.body.getReader();
        const decoder = new TextDecoder();
        let buffer = "";
        let fullResponse = "";
        let lastUsage: any = null;

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
              pendingDeltaRef.current += delta;
              if (!deltaTimerRef.current) {
                deltaTimerRef.current = setTimeout(() => flushDelta(assistantId, setMessages), 48);
              }
            }
            const usage = extractUsage(line.slice(5).trim());
            if (usage) lastUsage = usage;
          }
        }

        flushDelta(assistantId, setMessages);
        logEntry.responseBody = fullResponse;
        logEntry.durationMs = Date.now() - startTime;
        logEntry.statusCode = response.status;
        if (lastUsage) {
          logEntry.inputTokens = lastUsage.prompt_tokens;
          logEntry.outputTokens = lastUsage.completion_tokens;
          logEntry.totalTokens = lastUsage.total_tokens;
        }
        onLog(logEntry);
      } catch (error) {
        flushDelta(assistantId, setMessages);
        const msg = error instanceof Error ? error.message : "Chat request failed";
        logEntry.status = "error";
        logEntry.error = msg;
        logEntry.durationMs = Date.now() - startTime;
        setMessages((current) =>
          current.map((m) => (m.id === assistantId ? { ...m, content: `Request failed: ${msg}` } : m)),
        );
        onLog(logEntry);
        toast.error(msg);
      } finally {
        clearTimer();
        pendingDeltaRef.current = "";
        abortRef.current = null;
        setSending(false);
      }
    },
    [finalModelString, params, onLog, flushDelta, clearTimer],
  );

  const abort = useCallback(() => {
    abortRef.current?.abort();
    abortRef.current = null;
  }, []);

  return { send, abort };
}
