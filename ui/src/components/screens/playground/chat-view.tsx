import { Loader2, Shield } from "lucide-react";
import ReactMarkdown from "react-markdown";
import type { Message } from "./types";

export function ChatView({ messages }: { messages: Message[] }) {
  return (
    <>
      {messages.map((message) => (
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
                <p className="whitespace-pre-wrap text-xs">{message.content}</p>
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
      ))}
    </>
  );
}
