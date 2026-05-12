import { useCallback, useRef } from "react";
import { goStreamFetch } from "@/lib/go-api";
import { toast } from "sonner";
import type { ChatParams, Message, RequestLog } from "./types";
import {
  buildApiMessages,
  errorMessageFromResponseBody,
  processSseBuffer,
  stringifyRequestLog,
  toReadableErrorMessage,
} from "./stream-utils";

type SendTurnOptions = {
  model: string;
  params: ChatParams;
  userMessage: Message;
  messages: Message[];
  setMessages: (fn: (msgs: Message[]) => Message[]) => void;
};

export function useChatStream(
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

  const insertAssistantMessage = useCallback(
    (userMessage: Message, assistantId: string, setMessages: SendTurnOptions["setMessages"]) => {
      setMessages((current) => {
        const existingAssistant = current.some((m) => m.id === assistantId);
        const userIndex = current.findIndex((m) => m.id === userMessage.id);
        const next = current.map((m) =>
          m.id === userMessage.id ? { ...m, status: "sending" as const } : m,
        );
        if (existingAssistant) return next;
        const assistantMessage: Message = { id: assistantId, role: "assistant", content: "" };
        if (userIndex < 0) return [...next, { ...userMessage, status: "sending" }, assistantMessage];
        next.splice(userIndex + 1, 0, assistantMessage);
        return next;
      });
    },
    [],
  );

  const markUserStatus = useCallback(
    (userId: string, status: Message["status"], setMessages: SendTurnOptions["setMessages"]) => {
      setMessages((current) =>
        current.map((m) => (m.id === userId ? { ...m, status } : m)),
      );
    },
    [],
  );

  const queueDelta = useCallback(
    (assistantId: string, setMessages: SendTurnOptions["setMessages"], delta: string) => {
      pendingDeltaRef.current += delta;
      if (!deltaTimerRef.current) {
        deltaTimerRef.current = setTimeout(() => flushDelta(assistantId, setMessages), 48);
      }
    },
    [flushDelta],
  );

  const sendTurn = useCallback(
    async ({ model, params, userMessage, messages, setMessages }: SendTurnOptions) => {
      const content = userMessage.content.trim();
      if ((!content && !userMessage.attachments?.length) || !model) return;
      const assistantId = crypto.randomUUID();

      insertAssistantMessage(userMessage, assistantId, setMessages);

      const apiMessages = buildApiMessages(messages, userMessage, params.systemPrompt);
      const requestBody = {
        model,
        stream: true,
        messages: apiMessages,
        temperature: params.temperature,
        top_p: params.topP,
        max_tokens: params.maxTokens > 0 ? params.maxTokens : undefined,
      };

      const logEntry: RequestLog = {
        id: crypto.randomUUID(),
        timestamp: new Date().toISOString(),
        model,
        status: "success",
        requestBody: stringifyRequestLog(requestBody),
      };

      const controller = new AbortController();
      abortRef.current = controller;
      const startTime = Date.now();

      try {
        const response = await goStreamFetch("/v1/chat/completions", {
          method: "POST",
          body: JSON.stringify(requestBody),
          signal: controller.signal,
        });

        if (!response.ok) {
          const errBody = await response.json().catch(() => null);
          throw new Error(errorMessageFromResponseBody(errBody, response.statusText));
        }
        if (!response.body) throw new Error("No response stream available");

        const reader = response.body.getReader();
        const decoder = new TextDecoder();
        let buffer = "";
        let fullResponse = "";
        let lastUsage: unknown = null;

        while (true) {
          const { value, done } = await reader.read();
          if (done) break;
          buffer += decoder.decode(value, { stream: true });
          ({ buffer, fullResponse, lastUsage } = processSseBuffer(
            buffer,
            fullResponse,
            lastUsage,
            (delta) => queueDelta(assistantId, setMessages, delta),
          ));
        }

        buffer += decoder.decode();
        if (buffer.trim()) {
          const result = processSseBuffer(
            `${buffer}\n`,
            fullResponse,
            lastUsage,
            (delta) => queueDelta(assistantId, setMessages, delta),
          );
          fullResponse = result.fullResponse;
          lastUsage = result.lastUsage;
        }

        flushDelta(assistantId, setMessages);
        markUserStatus(userMessage.id, "done", setMessages);
        logEntry.responseBody = fullResponse;
        logEntry.durationMs = Date.now() - startTime;
        logEntry.statusCode = response.status;
        if (isUsagePayload(lastUsage)) {
          logEntry.inputTokens = lastUsage.prompt_tokens;
          logEntry.outputTokens = lastUsage.completion_tokens;
          logEntry.totalTokens = lastUsage.total_tokens;
        }
        onLog(logEntry);
      } catch (error) {
        flushDelta(assistantId, setMessages);
        if (error instanceof DOMException && error.name === "AbortError") {
          markUserStatus(userMessage.id, "error", setMessages);
          logEntry.status = "error";
          logEntry.error = "Request aborted";
          logEntry.durationMs = Date.now() - startTime;
          onLog(logEntry);
          return;
        }
        const msg = toReadableErrorMessage(error);
        logEntry.status = "error";
        logEntry.error = msg;
        logEntry.durationMs = Date.now() - startTime;
        markUserStatus(userMessage.id, "error", setMessages);
        setMessages((current) =>
          current.map((m) => (m.id === assistantId ? { ...m, content: msg, status: "error" } : m)),
        );
        onLog(logEntry);
        toast.error(msg);
      } finally {
        clearTimer();
        pendingDeltaRef.current = "";
        abortRef.current = null;
      }
    },
    [clearTimer, flushDelta, insertAssistantMessage, markUserStatus, onLog, queueDelta],
  );

  const abort = useCallback(() => {
    abortRef.current?.abort();
    abortRef.current = null;
  }, []);

  return { sendTurn, abort };
}

function isUsagePayload(value: unknown): value is {
  prompt_tokens?: number;
  completion_tokens?: number;
  total_tokens?: number;
} {
  return Boolean(value && typeof value === "object");
}
