import { AlertCircle, Loader2, Shield } from "lucide-react";
import ReactMarkdown from "react-markdown";
import { Badge } from "@/components/ui/badge";
import type { Attachment, Message } from "./types";

export function ChatView({ messages }: { messages: Message[] }) {
  return (
    <>
      {messages.map((message) => (
        <MessageRow key={message.id} message={message} />
      ))}
    </>
  );
}

function MessageRow({ message }: { message: Message }) {
  const isUser = message.role === "user";
  const isError = message.status === "error" && message.role === "assistant";

  return (
    <div className={`flex min-w-0 ${isUser ? "justify-end" : "justify-start"}`}>
      <div className={`flex min-w-0 max-w-[88%] flex-col gap-1.5 ${isUser ? "items-end" : "items-start"}`}>
        {message.role === "system" ? (
          <SystemBubble content={message.content} />
        ) : isError ? (
          <ErrorBubble content={message.content} />
        ) : isUser ? (
          <UserMessage message={message} />
        ) : (
          <AssistantMessage content={message.content} />
        )}
      </div>
    </div>
  );
}

function UserMessage({ message }: { message: Message }) {
  const hasText = Boolean(message.content.trim());

  return (
    <>
      {message.attachments && message.attachments.length > 0 && (
        <AttachmentGrid attachments={message.attachments} />
      )}
      {hasText && (
        <div className="max-w-full rounded-2xl rounded-tr-md bg-primary px-4 py-2.5 text-sm leading-relaxed text-primary-foreground shadow-sm">
          <p className="whitespace-pre-wrap break-words">{message.content}</p>
        </div>
      )}
      {message.status === "queued" && (
        <Badge variant="secondary" className="h-5 text-[10px]">Queued</Badge>
      )}
      {message.status === "sending" && (
        <span className="text-[11px] text-muted-foreground">Sending</span>
      )}
    </>
  );
}

function AttachmentGrid({ attachments }: { attachments: Attachment[] }) {
  return (
    <div className="grid max-w-[320px] grid-cols-2 gap-2 rounded-2xl border bg-card p-2 shadow-sm">
      {attachments.map((attachment) => (
        <figure key={attachment.id} className="min-w-0 overflow-hidden rounded-xl border bg-muted">
          <img
            src={attachment.dataUrl}
            alt={attachment.name}
            className="aspect-[4/3] w-full object-cover"
          />
          <figcaption className="truncate px-2 py-1 text-[10px] text-muted-foreground">
            {attachment.name}
          </figcaption>
        </figure>
      ))}
    </div>
  );
}

function AssistantMessage({ content }: { content: string }) {
  if (!content) {
    return (
      <div className="flex items-center gap-2 rounded-2xl rounded-tl-md border bg-card px-4 py-2.5 text-sm text-muted-foreground shadow-sm">
        <Loader2 className="h-4 w-4 animate-spin" />
        <span>Streaming...</span>
      </div>
    );
  }

  return (
    <div className="max-w-full rounded-2xl rounded-tl-md border bg-card px-4 py-3 text-sm text-card-foreground shadow-sm">
      <div className="prose prose-sm max-w-none break-words dark:prose-invert prose-pre:max-w-full prose-pre:overflow-x-auto prose-code:break-words">
        <ReactMarkdown>{content}</ReactMarkdown>
      </div>
    </div>
  );
}

function ErrorBubble({ content }: { content: string }) {
  return (
    <div className="max-w-full rounded-2xl rounded-tl-md border border-destructive/35 bg-destructive/10 px-4 py-3 text-sm text-foreground shadow-sm">
      <div className="mb-1 flex items-center gap-2 font-medium text-destructive">
        <AlertCircle className="h-4 w-4 shrink-0" />
        <span>Request failed</span>
      </div>
      <p className="whitespace-pre-wrap break-words text-sm">{content}</p>
    </div>
  );
}

function SystemBubble({ content }: { content: string }) {
  return (
    <div className="max-w-full rounded-2xl border border-amber-300/40 bg-amber-50 px-4 py-3 text-amber-950 dark:bg-amber-950/40 dark:text-amber-100">
      <div className="flex items-start gap-2">
        <Shield className="mt-0.5 h-4 w-4 shrink-0" />
        <p className="whitespace-pre-wrap break-words text-xs">{content}</p>
      </div>
    </div>
  );
}
