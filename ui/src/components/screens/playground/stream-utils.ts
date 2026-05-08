import type { Message } from "./types";

type ChatApiMessage = {
  role: "user" | "assistant" | "system";
  content: string | Array<{ type: string; text?: string; image_url?: { url: string } }>;
};

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

  result.push(...messages.filter((m) => m.role !== "system").map(toApiMsg));
  result.push(toApiMsg(userMessage));
  return result;
}
