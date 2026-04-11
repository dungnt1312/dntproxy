import { useEffect, useMemo, useRef, useState } from 'react'
import { Bot, Loader2, Send, Trash2 } from 'lucide-react'
import ReactMarkdown from 'react-markdown'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Badge } from '@/components/ui/badge'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { goApi } from '@/lib/go-api'

type Model = {
  id: string
  displayName: string
  provider: string
}

type Message = {
  id: string
  role: 'user' | 'assistant'
  content: string
}

type ChatApiMessage = {
  role: 'user' | 'assistant'
  content: string
}

const GO_API_BASE = import.meta.env.VITE_GO_API_URL || ''

function extractDeltaContent(line: string): string {
  if (!line.startsWith('data:')) return ''
  const payload = line.slice(5).trim()
  if (!payload || payload === '[DONE]') return ''

  try {
    const parsed = JSON.parse(payload)
    const choice = parsed?.choices?.[0]
    return choice?.delta?.content || choice?.message?.content || ''
  } catch {
    return ''
  }
}

export default function PlaygroundScreen() {
  const [models, setModels] = useState<Model[]>([])
  const [selectedModelId, setSelectedModelId] = useState('')
  const [messages, setMessages] = useState<Message[]>([])
  const [input, setInput] = useState('')
  const [loadingModels, setLoadingModels] = useState(true)
  const [sending, setSending] = useState(false)

  const abortControllerRef = useRef<AbortController | null>(null)
  const scrollAnchorRef = useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    async function loadModels() {
      setLoadingModels(true)
      try {
        const data = await goApi.getModels()
        const availableModels = Array.isArray(data) ? data : []

        setModels(availableModels)
        if (availableModels.length > 0) {
          setSelectedModelId(availableModels[0].id)
        }
      } catch {
        toast.error('Failed to load models')
      } finally {
        setLoadingModels(false)
      }
    }

    loadModels()
  }, [])

  useEffect(() => {
    scrollAnchorRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  const selectedModel = useMemo(
    () => models.find((model) => model.id === selectedModelId),
    [models, selectedModelId]
  )

  async function handleSend() {
    const content = input.trim()
    if (!content || sending || !selectedModelId) return

    const userMessage: Message = {
      id: crypto.randomUUID(),
      role: 'user',
      content,
    }
    const assistantId = crypto.randomUUID()

    setMessages((current) => [...current, userMessage, { id: assistantId, role: 'assistant', content: '' }])
    setInput('')
    setSending(true)

    const nextMessages: ChatApiMessage[] = [...messages, userMessage].map((message) => ({
      role: message.role,
      content: message.content,
    }))

    const controller = new AbortController()
    abortControllerRef.current = controller

    try {
      const response = await fetch(`${GO_API_BASE}/v1/chat/completions`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          model: selectedModelId,
          stream: true,
          messages: nextMessages,
        }),
        signal: controller.signal,
      })

      if (!response.ok) {
        const errorBody = await response.json().catch(() => null)
        throw new Error(errorBody?.error?.message || response.statusText)
      }

      if (!response.body) {
        throw new Error('No response stream available')
      }

      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''

      while (true) {
        const { value, done } = await reader.read()
        if (done) break

        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n')
        buffer = lines.pop() || ''

        for (const line of lines) {
          const delta = extractDeltaContent(line)
          if (!delta) continue

          setMessages((current) =>
            current.map((message) =>
              message.id === assistantId
                ? { ...message, content: message.content + delta }
                : message
            )
          )
        }
      }
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Chat request failed'

      setMessages((current) =>
        current.map((item) =>
          item.id === assistantId
            ? { ...item, content: `Request failed: ${message}` }
            : item
        )
      )
      toast.error(message)
    } finally {
      abortControllerRef.current = null
      setSending(false)
    }
  }

  function handleClear() {
    abortControllerRef.current?.abort()
    abortControllerRef.current = null
    setSending(false)
    setMessages([])
  }

  return (
    <div className="flex h-full flex-col">
      <div className="flex flex-col gap-3 border-b bg-background/95 px-4 py-3 backdrop-blur-sm md:flex-row md:items-center md:justify-between md:px-6">
        <div className="flex items-center gap-2">
          <Bot className="h-5 w-5 text-emerald-600" />
          <div>
            <h1 className="text-lg font-semibold">Playground</h1>
            <p className="text-xs text-muted-foreground">Send real chat requests to `/v1/chat/completions` using a backend-supported model.</p>
          </div>
        </div>

        <div className="flex w-full flex-col gap-2 sm:w-auto sm:flex-row">
          <Select value={selectedModelId} onValueChange={setSelectedModelId} disabled={loadingModels || models.length === 0}>
            <SelectTrigger className="w-full sm:w-[260px]">
              <SelectValue placeholder={loadingModels ? 'Loading models…' : 'Select model'} />
            </SelectTrigger>
            <SelectContent>
              {models.map((model) => (
                <SelectItem key={model.id} value={model.id}>
                  {model.displayName}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>

          <Button variant="outline" size="icon" onClick={handleClear} className="h-9 w-9 shrink-0">
            <Trash2 className="h-4 w-4" />
          </Button>
        </div>
      </div>

      <ScrollArea className="flex-1">
        <div className="mx-auto flex max-w-3xl flex-col gap-4 p-4 md:p-6">
          {messages.length === 0 ? (
            <div className="flex min-h-[45vh] flex-col items-center justify-center gap-4 text-center">
              <div className="flex h-16 w-16 items-center justify-center rounded-full bg-emerald-100 dark:bg-emerald-950">
                <Bot className="h-8 w-8 text-emerald-600" />
              </div>
              <div>
                <h2 className="text-xl font-semibold">Choose a model & send a real request</h2>
                <p className="mt-2 max-w-md text-sm text-muted-foreground">
                  This screen uses the backend chat endpoint directly. Select one model target, then test routing and streaming with a normal chat prompt.
                </p>
              </div>
              {selectedModel && (
                <Badge variant="secondary">{selectedModel.id}</Badge>
              )}
            </div>
          ) : (
            messages.map((message) => (
              <div
                key={message.id}
                className={`flex ${message.role === 'user' ? 'justify-end' : 'justify-start'}`}
              >
                <div
                  className={`max-w-[88%] rounded-xl px-4 py-3 text-sm ${
                    message.role === 'user'
                      ? 'bg-primary text-primary-foreground'
                      : 'bg-muted text-foreground'
                  }`}
                >
                  {message.role === 'assistant' ? (
                    message.content ? (
                      <div className="prose prose-sm max-w-none dark:prose-invert">
                        <ReactMarkdown>{message.content}</ReactMarkdown>
                      </div>
                    ) : (
                      <div className="flex items-center gap-2 text-muted-foreground">
                        <Loader2 className="h-4 w-4 animate-spin" />
                        <span>Waiting for stream…</span>
                      </div>
                    )
                  ) : (
                    <p className="whitespace-pre-wrap">{message.content}</p>
                  )}
                </div>
              </div>
            ))
          )}
          <div ref={scrollAnchorRef} />
        </div>
      </ScrollArea>

      <div className="border-t bg-background/95 px-4 py-3 backdrop-blur-sm md:px-6">
        <div className="mx-auto flex max-w-3xl gap-2">
          <Textarea
            value={input}
            onChange={(event) => setInput(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Enter' && !event.shiftKey) {
                event.preventDefault()
                handleSend()
              }
            }}
            placeholder="Type your message…"
            className="min-h-[44px] resize-none rounded-xl"
            rows={1}
            disabled={sending || !selectedModelId}
          />
          <Button
            onClick={handleSend}
            disabled={sending || !input.trim() || !selectedModelId}
            size="icon"
            className="h-[44px] w-[44px] shrink-0 rounded-xl"
          >
            {sending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}
          </Button>
        </div>
      </div>
    </div>
  )
}
