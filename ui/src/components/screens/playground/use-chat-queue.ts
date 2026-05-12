import { useCallback, useEffect, useRef, useState } from "react";
import type { ChatParams, Message, RequestLog } from "./types";
import { useChatStream } from "./use-chat-stream";

type QueuedTurn = {
  userMessage: Message;
  model: string;
  params: ChatParams;
};

export function useChatQueue(onLog: (log: RequestLog) => void) {
  const [messages, setMessages] = useState<Message[]>([]);
  const [sending, setSending] = useState(false);
  const [queuedCount, setQueuedCount] = useState(0);
  const messagesRef = useRef<Message[]>([]);
  const queueRef = useRef<QueuedTurn[]>([]);
  const processingRef = useRef(false);
  const { sendTurn, abort } = useChatStream(onLog);

  useEffect(() => {
    messagesRef.current = messages;
  }, [messages]);

  const updateMessages = useCallback((updater: (current: Message[]) => Message[]) => {
    const next = updater(messagesRef.current);
    messagesRef.current = next;
    setMessages(next);
  }, []);

  const processQueue = useCallback(async () => {
    if (processingRef.current) return;
    const nextTurn = queueRef.current.shift();
    setQueuedCount(queueRef.current.length);
    if (!nextTurn) return;

    processingRef.current = true;
    setSending(true);
    try {
      await sendTurn({
        model: nextTurn.model,
        params: nextTurn.params,
        userMessage: nextTurn.userMessage,
        messages: messagesRef.current,
        setMessages: updateMessages,
      });
    } finally {
      processingRef.current = false;
      setSending(false);
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
    abort();
    queueRef.current = [];
    processingRef.current = false;
    setQueuedCount(0);
    setSending(false);
    setMessages([]);
    messagesRef.current = [];
  }, [abort]);

  return {
    messages,
    sending,
    queuedCount,
    enqueueTurn,
    clearQueue,
  };
}
