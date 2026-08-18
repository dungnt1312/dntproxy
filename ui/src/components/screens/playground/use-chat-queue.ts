import { useCallback, useEffect, useRef, useState } from "react";
import type { ChatParams, Message, RequestLog } from "./types";
import { useChatStream } from "./use-chat-stream";

type QueuedTurn = {
  userMessage: Message;
  model: string;
  params: ChatParams;
};

const MAX_PLAYGROUND_MESSAGES = 80;

export function useChatQueue(onLog: (log: RequestLog) => void) {
  const [messages, setMessages] = useState<Message[]>([]);
  const [sending, setSending] = useState(false);
  const [queuedCount, setQueuedCount] = useState(0);
  const [streamingId, setStreamingId] = useState<string | undefined>(undefined);
  const messagesRef = useRef<Message[]>([]);
  const queueRef = useRef<QueuedTurn[]>([]);
  const processingRef = useRef(false);
  const generationRef = useRef(0);
  const { sendTurn, abort } = useChatStream(onLog);

  useEffect(() => {
    messagesRef.current = messages;
  }, [messages]);

  const updateMessages = useCallback((updater: (current: Message[]) => Message[]) => {
    const next = updater(messagesRef.current).slice(-MAX_PLAYGROUND_MESSAGES);
    messagesRef.current = next;
    setMessages(next);
  }, []);

  const processQueue = useCallback(async () => {
    if (processingRef.current) return;
    const nextTurn = queueRef.current.shift();
    setQueuedCount(queueRef.current.length);
    if (!nextTurn) return;

    const generation = generationRef.current;
    processingRef.current = true;
    setSending(true);
    const assistantId = crypto.randomUUID();
    setStreamingId(assistantId);
    try {
      await sendTurn({
        model: nextTurn.model,
        params: nextTurn.params,
        userMessage: nextTurn.userMessage,
        messages: messagesRef.current,
        setMessages: updateMessages,
        assistantId,
      });
    } finally {
      if (generation !== generationRef.current) return;
      processingRef.current = false;
      setSending(false);
      setStreamingId(undefined);
      void processQueue();
    }
  }, [sendTurn, updateMessages]);

  const enqueueTurn = useCallback(
    (userMessage: Message, model: string, params: ChatParams) => {
      const queuedMessage = {
        ...userMessage,
        status: processingRef.current ? ("queued" as const) : ("sending" as const),
      };
      updateMessages((prev) => [...prev, queuedMessage]);
      queueRef.current.push({ userMessage: queuedMessage, model, params: { ...params } });
      setQueuedCount(queueRef.current.length);
      void processQueue();
    },
    [processQueue, updateMessages],
  );

  const clearQueue = useCallback(() => {
    generationRef.current += 1;
    abort();
    queueRef.current = [];
    processingRef.current = false;
    setQueuedCount(0);
    setSending(false);
    setStreamingId(undefined);
    setMessages([]);
    messagesRef.current = [];
  }, [abort]);

  useEffect(() => {
    return () => {
      generationRef.current += 1;
      abort();
    };
  }, [abort]);

  return {
    messages,
    sending,
    queuedCount,
    streamingId,
    enqueueTurn,
    clearQueue,
  };
}
