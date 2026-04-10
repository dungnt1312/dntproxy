import { useEffect, useState, useRef } from 'react'
import { api } from '../api'
import { Send, Bot, User, Loader2, Trash2, ChevronDown } from 'lucide-react'
import { getModelName, PROVIDER_CONFIGS, getProviderFromModelId } from '../models-config'

interface Message {
  role: 'user' | 'assistant' | 'system'
  content: string
  streaming?: boolean
}

export default function Playground() {
  const [models, setModels] = useState<string[]>([])
  const [selectedModel, setSelectedModel] = useState('')
  const [messages, setMessages] = useState<Message[]>([{ role: 'assistant', content: 'Hello! How can I help you today?' }])
  const [input, setInput] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [showModelDropdown, setShowModelDropdown] = useState(false)
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const dropdownRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    api.getModels().then(data => {
      const modelList = (data as Array<{ id: string }>)?.map(m => m.id) || []
      setModels(modelList)
      if (modelList.length > 0 && !selectedModel) {
        setSelectedModel(modelList[0])
      }
    }).catch(() => {})
  }, [selectedModel])

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setShowModelDropdown(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  const handleSend = async () => {
    if (!input.trim() || loading) return

    const userMessage = input.trim()
    setInput('')
    setError('')
    setMessages(prev => [...prev, { role: 'user', content: userMessage }])
    setLoading(true)

    const assistantIndex = messages.length + 1
    setMessages(prev => [...prev, { role: 'assistant', content: '', streaming: true }])

    try {
      const response = await fetch('/v1/chat/completions', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          model: selectedModel,
          messages: messages.concat([{ role: 'user', content: userMessage }]).map(m => ({ role: m.role, content: m.content })),
          stream: true,
        }),
      })

      if (!response.ok) {
        const err = await response.json().catch(() => ({ error: response.statusText }))
        throw new Error(err.error || err.message || response.statusText)
      }

      const reader = response.body?.getReader()
      const decoder = new TextDecoder()
      let fullContent = ''

      if (reader) {
        while (true) {
          const { done, value } = await reader.read()
          if (done) break

          const chunk = decoder.decode(value)
          const lines = chunk.split('\n')

          for (const line of lines) {
            if (line.startsWith('data: ')) {
              const data = line.slice(6)
              if (data === '[DONE]') continue

              try {
                const parsed = JSON.parse(data)
                const delta = parsed.choices?.[0]?.delta?.content
                if (delta) {
                  fullContent += delta
                  setMessages(prev => {
                    const updated = [...prev]
                    if (updated[assistantIndex] && updated[assistantIndex].streaming) {
                      updated[assistantIndex] = { ...updated[assistantIndex], content: fullContent }
                    }
                    return updated
                  })
                }
              } catch (e) {
                console.error('Parse error:', e)
              }
            }
          }
        }
      }

      setMessages(prev => {
        const updated = [...prev]
        if (updated[assistantIndex]?.streaming) {
          updated[assistantIndex] = { ...updated[assistantIndex], content: fullContent, streaming: false }
        }
        return updated
      })
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : 'Request failed'
      setError(errorMessage)
      setMessages(prev => {
        const updated = [...prev]
        if (updated[assistantIndex]?.streaming) {
          updated[assistantIndex] = { role: 'assistant', content: `Error: ${errorMessage}` }
        }
        return updated
      })
    } finally {
      setLoading(false)
    }
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }

  const clearChat = () => {
    setMessages([{ role: 'assistant', content: 'Hello! How can I help you today?' }])
    setError('')
  }

  return (
    <div className="flex flex-col h-[calc(100vh-48px-48px)]">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-2xl font-bold">Playground</h2>
        <div className="flex items-center gap-2">
          <div ref={dropdownRef} className="relative">
            <button
              onClick={() => setShowModelDropdown(!showModelDropdown)}
              className="flex items-center gap-2 px-3 py-2 bg-[var(--bg-card)] border border-[var(--border)] rounded-lg text-sm hover:border-[var(--accent)] transition-colors min-w-[200px]"
            >
              <span className="flex-1 text-left truncate">
                {selectedModel ? (
                  <>
                    <span className="mr-1">{PROVIDER_CONFIGS[getProviderFromModelId(selectedModel)]?.icon || '📦'}</span>
                    {getModelName(selectedModel)}
                  </>
                ) : 'Select model'}
              </span>
              <ChevronDown size={14} className={`transition-transform ${showModelDropdown ? 'rotate-180' : ''}`} />
            </button>
            {showModelDropdown && (
              <div className="absolute top-full left-0 mt-1 w-full bg-[var(--bg-card)] border border-[var(--border)] rounded-lg shadow-xl z-50 max-h-64 overflow-y-auto">
                {models.length === 0 ? (
                  <div className="px-3 py-2 text-sm text-[var(--text-muted)]">No models available</div>
                ) : (
                  models.map(model => (
                    <button
                      key={model}
                      onClick={() => { setSelectedModel(model); setShowModelDropdown(false) }}
                      className={`w-full px-3 py-2 text-left text-sm hover:bg-[var(--bg-hover)] flex items-center gap-2 ${
                        model === selectedModel ? 'text-[var(--accent)]' : ''
                      }`}
                    >
                      <span>{PROVIDER_CONFIGS[getProviderFromModelId(model)]?.icon || '📦'}</span>
                      <span className="flex-1 truncate">{getModelName(model)}</span>
                    </button>
                  ))
                )}
              </div>
            )}
          </div>
          <button
            onClick={clearChat}
            className="p-2 hover:bg-[var(--bg-card)] rounded-lg transition-colors"
            title="Clear chat"
          >
            <Trash2 size={18} className="text-[var(--text-muted)]" />
          </button>
        </div>
      </div>

      {error && (
        <div className="mb-4 p-3 bg-red-900/30 border border-red-800 rounded-lg text-red-400 text-sm">
          {error}
        </div>
      )}

      <div className="flex-1 overflow-y-auto bg-[var(--bg-card)] rounded-xl border border-[var(--border)] mb-4 p-4 space-y-4">
        {messages.map((msg, i) => (
          <div key={i} className={`flex gap-3 ${msg.role === 'user' ? 'flex-row-reverse' : ''}`}>
            <div className={`shrink-0 w-8 h-8 rounded-full flex items-center justify-center ${
              msg.role === 'user' ? 'bg-[var(--accent)]' : 'bg-[var(--bg-hover)]'
            }`}>
              {msg.role === 'user' ? <User size={16} /> : <Bot size={16} />}
            </div>
            <div className={`flex-1 max-w-[80%] ${msg.role === 'user' ? 'text-right' : ''}`}>
              <div className={`inline-block px-4 py-2 rounded-2xl ${
                msg.role === 'user'
                  ? 'bg-[var(--accent)] text-white rounded-tr-sm'
                  : 'bg-[var(--bg-hover)] rounded-tl-sm'
              }`}>
                <p className="text-sm whitespace-pre-wrap">{msg.content}</p>
                {msg.streaming && <Loader2 size={14} className="inline animate-spin ml-2" />}
              </div>
            </div>
          </div>
        ))}
        <div ref={messagesEndRef} />
      </div>

      <div className="flex gap-3">
        <textarea
          value={input}
          onChange={e => setInput(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="Type your message... (Enter to send, Shift+Enter for newline)"
          className="flex-1 bg-[var(--bg-card)] border border-[var(--border)] rounded-xl px-4 py-3 text-sm resize-none h-14 focus:outline-none focus:border-[var(--accent)] transition-colors"
          disabled={loading}
        />
        <button
          onClick={handleSend}
          disabled={loading || !input.trim()}
          className="px-6 py-3 bg-[var(--accent)] hover:bg-[var(--accent-hover)] disabled:opacity-50 disabled:cursor-not-allowed rounded-xl transition-colors flex items-center gap-2"
        >
          <Send size={18} />
          Send
        </button>
      </div>
    </div>
  )
}
