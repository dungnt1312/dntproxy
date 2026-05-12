import type { ChatParams, Message } from "./types";

type ChatApiMessage = {
  role: "user" | "assistant" | "system";
  content: string | Array<{ type: string; text?: string; image_url?: { url: string } }>;
};

type ModelLike = {
  id: string;
  provider: string;
};

const DATA_URL_PATTERN = /^data:([^;,]+).*?,/;

export function getSelectableModelValue(model: ModelLike): string {
  if (model.provider === "combo" || model.provider === "alias") return model.id;
  return model.id.includes("/") ? model.id.split("/").slice(1).join("/") : model.id;
}

export function buildPlaygroundModelId(provider: string, model: string, account: string): string {
  if (!provider || !model) return "";
  if (provider === "combo" || provider === "alias") return model;
  const base = `${provider}/${model}`;
  return account && account !== "auto" ? `${base}@${account}` : base;
}

export function extractDeltaContent(line: string): string {
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

export function extractUsage(payload: string) {
  try {
    const parsed = JSON.parse(payload);
    return parsed?.usage || null;
  } catch {
    return null;
  }
}

export function processSseBuffer(
  rawBuffer: string,
  currentResponse: string,
  currentUsage: unknown,
  onDelta: (delta: string) => void,
) {
  const lines = rawBuffer.split("\n");
  const nextBuffer = lines.pop() || "";
  let fullResponse = currentResponse;
  let lastUsage = currentUsage;

  for (const line of lines) {
    const delta = extractDeltaContent(line);
    if (delta) {
      fullResponse += delta;
      onDelta(delta);
    }
    if (line.startsWith("data:")) {
      const usage = extractUsage(line.slice(5).trim());
      if (usage) lastUsage = usage;
    }
  }

  return { buffer: nextBuffer, fullResponse, lastUsage };
}

export function buildApiMessages(
  messages: Message[],
  userMessage: Message,
  systemPrompt: string,
): ChatApiMessage[] {
  const result: ChatApiMessage[] = [];
  if (systemPrompt.trim()) result.push({ role: "system", content: systemPrompt.trim() });

  const toApiMsg = (m: Message): ChatApiMessage => {
    if (m.attachments && m.attachments.length > 0) {
      const parts: Array<{ type: string; text?: string; image_url?: { url: string } }> = [];
      if (m.content) parts.push({ type: "text", text: m.content });
      m.attachments.forEach((att) => parts.push({ type: "image_url", image_url: { url: att.dataUrl } }));
      return { role: m.role, content: parts };
    }
    return { role: m.role, content: m.content };
  };

  const currentIndex = messages.findIndex((m) => m.id === userMessage.id);
  const history = currentIndex >= 0 ? messages.slice(0, currentIndex) : messages;
  result.push(
    ...history
      .filter((m) => m.role !== "system")
      .filter((m) => m.status !== "queued")
      .filter((m) => m.role !== "assistant" || m.content.trim())
      .map(toApiMsg),
  );
  result.push(toApiMsg(userMessage));
  return result;
}

export function stringifyRequestLog(body: {
  model: string;
  stream: boolean;
  messages: ChatApiMessage[];
  temperature: ChatParams["temperature"];
  top_p: ChatParams["topP"];
  max_tokens?: number;
}): string {
  return JSON.stringify(redactLargePayloads(body), null, 2);
}

export function toReadableErrorMessage(error: unknown): string {
  const raw = error instanceof Error ? error.message : String(error || "Chat request failed");
  const parsed = parseErrorText(raw);
  return parsed || raw;
}

export function errorMessageFromResponseBody(body: unknown, fallback: string): string {
  if (!body) return fallback;
  if (typeof body === "string") return parseErrorText(body) || body || fallback;
  if (typeof body !== "object") return fallback;

  const payload = body as Record<string, unknown>;
  const nested = payload.error;
  if (typeof nested === "string") return parseErrorText(nested) || nested;
  if (nested && typeof nested === "object") {
    const message = (nested as Record<string, unknown>).message;
    if (typeof message === "string") return message;
  }
  if (typeof payload.message === "string") return payload.message;
  return fallback;
}

function redactLargePayloads(value: unknown): unknown {
  if (typeof value === "string") return redactDataUrl(value);
  if (Array.isArray(value)) return value.map(redactLargePayloads);
  if (!value || typeof value !== "object") return value;

  return Object.fromEntries(
    Object.entries(value).map(([key, item]) => [key, redactLargePayloads(item)]),
  );
}

function redactDataUrl(value: string): string {
  const match = value.match(DATA_URL_PATTERN);
  if (!match) return value;
  const approxBytes = Math.round((value.length - match[0].length) * 0.75);
  return `[${match[1]} data URL redacted, ~${formatBytes(approxBytes)}]`;
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function parseErrorText(raw: string): string {
  const jsonStart = raw.indexOf("{");
  if (jsonStart < 0) return raw.replace(/^Request failed:\s*/i, "").trim();

  const prefix = raw.slice(0, jsonStart).replace(/^Request failed:\s*/i, "").trim();
  const jsonText = raw.slice(jsonStart);
  try {
    const parsed = JSON.parse(jsonText);
    const message = errorMessageFromResponseBody(parsed, "");
    if (message) return message;
  } catch {
    // Keep the cleaned prefix below.
  }
  return prefix || raw;
}
