import { useEffect, useState, useRef } from 'react'
import { api } from '../api'
import { Send, Bot, User, Loader2, Trash2, ChevronDown } from 'lucide-react'
import { getModelName, PROVIDER_CONFIGS, getProviderFromModelId } from '../models-config'
import { AwsLogo, OpenAILogo } from '../components/connections/helpers'

function ModelIcon({ provider }: { provider: string }) {
  if (provider === 'kiro') return <AwsLogo size={14} />
  if (provider === 'openai') return <OpenAILogo size={14} />
  const cfg = PROVIDER_CONFIGS[provider]
  return <span className="text-[10px] font-bold opacity-60">{cfg?.icon || 'OT'}</span>
}

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
      <div className="page-header mb-3">
        <h2 className="page-title">Playground</h2>
        <div className="flex items-center gap-2">
          <div ref={dropdownRef} className="relative">
            <button
              onClick={() => setShowModelDropdown(!showModelDropdown)}
              className="glass-sm flex items-center gap-2 px-3 py-2 text-xs hover:border-[var(--border-hover)] transition-all min-w-[200px] cursor-pointer"
            >
              <span className="flex-1 text-left truncate flex items-center gap-1.5">
                {selectedModel ? (
                  <>
                    <ModelIcon provider={getProviderFromModelId(selectedModel)} />
                    {getModelName(selectedModel)}
                  </>
                ) : 'Select model'}
              </span>
              <ChevronDown size={13} className={`transition-transform text-[var(--text-dim)] ${showModelDropdown ? 'rotate-180' : ''}`} />
            </button>
            {showModelDropdown && (
              <div className="absolute top-full left-0 mt-1 w-full glass shadow-xl z-50 max-h-64 overflow-y-auto animate-fade-in">
                {models.length === 0 ? (
                  <div className="px-3 py-2 text-xs text-[var(--text-dim)]">No models available</div>
                ) : (
                  models.map(model => (
                    <button
                      key={model}
                      onClick={() => { setSelectedModel(model); setShowModelDropdown(false) }}
                      className={`w-full px-3 py-2 text-left text-xs hover:bg-white/[0.04] flex items-center gap-2 transition-colors cursor-pointer ${
                        model === selectedModel ? 'text-[var(--accent)] bg-[var(--accent-glow)]' : 'text-[var(--text-muted)]'
                      }`}
                    >
                      <ModelIcon provider={getProviderFromModelId(model)} />
                      <span className="flex-1 truncate">{getModelName(model)}</span>
                    </button>
                  ))
                )}
              </div>
            )}
          </div>
          <button
            onClick={clearChat}
            className="btn-icon"
            title="Clear chat"
          >
            <Trash2 size={16} />
          </button>
        </div>
      </div>

      {error && (
        <div className="mb-3 glass-sm p-3 border-[var(--danger)]/20 text-[var(--danger)] text-xs animate-slide-up">
          {error}
        </div>
      )}

      <div className="flex-1 overflow-y-auto glass mb-4 p-4 space-y-4">
        {messages.map((msg, i) => (
          <div key={i} className={`flex gap-3 ${msg.role === 'user' ? 'flex-row-reverse' : ''} animate-fade-in`}>
            <div className={`shrink-0 w-8 h-8 rounded-xl flex items-center justify-center ${
              msg.role === 'user' ? 'bg-gradient-to-br from-[var(--accent)] to-[#7c3aed]' : 'bg-white/[0.05] border border-[var(--border)]'
            }`}>
              {msg.role === 'user' ? <User size={14} className="text-white" /> : <Bot size={14} className="text-[var(--text-muted)]" />}
            </div>
            <div className={`flex-1 max-w-[80%] ${msg.role === 'user' ? 'text-right' : ''}`}>
              <div className={`inline-block px-4 py-2.5 ${
                msg.role === 'user'
                  ? 'bg-gradient-to-br from-[var(--accent)] to-[#7c3aed] text-white rounded-2xl rounded-tr-md shadow-lg shadow-[var(--accent)]/10'
                  : 'glass-sm rounded-2xl rounded-tl-md'
              }`}>
                <p className="text-sm whitespace-pre-wrap leading-relaxed">{msg.content}</p>
                {msg.streaming && <Loader2 size={12} className="inline animate-spin ml-1 opacity-60" />}
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
          className="glass-input flex-1 resize-none h-14"
          disabled={loading}
        />
        <button
          onClick={handleSend}
          disabled={loading || !input.trim()}
          className="btn-primary px-6 py-3 rounded-xl"
        >
          <Send size={16} />
        </button>
      </div>
    </div>
  )
}
