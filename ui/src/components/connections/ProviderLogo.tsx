import { useMemo } from 'react'

// ─── Provider to logo file mapping ────────────────────────────────────────────

const PROVIDER_LOGO_MAP: Record<string, string> = {
  // Core providers from AGENTS.md
  'kiro': 'kiro.png',
  'openai': 'openai.png',
  'openai-compatible': 'oai-cc.png',
  'anthropic': 'anthropic.png',
  'glm': 'glm.png',
  'minimax': 'minimax.png',
  'qwen': 'qwen.png',

  // Additional providers
  'deepseek': 'deepseek.png',
  'google': 'gemini.png',
  'groq': 'groq.png',
  'mistral': 'mistral.png',
  'cohere': 'cohere.png',
  'ollama': 'ollama.png',
  'openrouter': 'openrouter.png',
  'fireworks': 'fireworks.png',
  'together': 'together.png',
  'huggingface': 'huggingface.png',
  'nvidia': 'nvidia.png',
  'cerebras': 'cerebras.png',
  'nebius': 'nebius.png',
  'xai': 'xai.png',
  'perplexity': 'perplexity.png',

  // AI IDE tools
  'cursor': 'cursor.png',
  'copilot': 'copilot.png',
  'cline': 'cline.png',
  'continue': 'continue.png',
  'roo': 'roo.png',
  'opencode': 'opencode.png',
  'kilocode': 'kilocode.png',
  'openclaw': 'openclaw.png',
  'codex': 'codex.png',

  // Specialized
  'claude': 'claude.png',
  'gemini': 'gemini.png',
  'kimi': 'kimi.png',
  'glm-cn': 'glm-cn.png',
  'minimax-cn': 'minimax-cn.png',

  // TTS/Media
  'elevenlabs': 'elevenlabs.png',
  'deepgram': 'deepgram.png',
  'playht': 'playht.png',
  'google-tts': 'google-tts.png',
  'edge-tts': 'edge-tts.png',

  // Image/Video
  'comfyui': 'comfyui.png',
  'sdwebui': 'sdwebui.png',
  'nanobanana': 'nanobanana.png',

  // Other
  'hyperbolic': 'hyperbolic.png',
  'chutes': 'chutes.png',
  'cartesia': 'cartesia.png',
  'assemblyai': 'assemblyai.png',
  'vertex': 'vertex.png',
  'vertex-partner': 'vertex-partner.png',
  'antigravity': 'antigravity.png',
  'droid': 'droid.png',
  'iflow': 'iflow.png',
  'kimi-coding': 'kimi-coding.png',
  'siliconflow': 'siliconflow.png',
  'local-device': 'local-device.png',
  'alicode': 'alicode.png',
  'alicode-intl': 'alicode-intl.png',
}

// ─── ProviderLogo Component ───────────────────────────────────────────────────

export interface ProviderLogoProps {
  provider: string
  size?: number
  className?: string
}

/**
 * Unified provider logo component using PNG images.
 * Falls back to text-based icon if image not found or fails to load.
 */
export function ProviderLogo({ provider, size = 24, className = '' }: ProviderLogoProps) {
  const logoFile = useMemo(() => {
    const normalized = provider.toLowerCase().trim()

    // Exact match
    if (PROVIDER_LOGO_MAP[normalized]) return PROVIDER_LOGO_MAP[normalized]

    // Base match (e.g., "kiro-v2" -> "kiro")
    const base = normalized.split('-')[0]
    if (PROVIDER_LOGO_MAP[base]) return PROVIDER_LOGO_MAP[base]

    // Partial match
    for (const [key, value] of Object.entries(PROVIDER_LOGO_MAP)) {
      if (normalized.includes(key) || key.includes(normalized)) {
        return value
      }
    }

    return null
  }, [provider])

  if (!logoFile) {
    return <TextFallback provider={provider} size={size} />
  }

  return (
    <img
      src={`/providers/${logoFile}`}
      alt={provider}
      width={size}
      height={size}
      className={className}
      style={{ width: size, height: size, objectFit: 'contain' }}
      onError={(e) => {
        const target = e.target as HTMLImageElement
        target.style.display = 'none'
        const parent = target.parentNode
        if (parent) {
          const fallback = document.createElement('div')
          fallback.className = 'flex items-center justify-center rounded font-bold bg-muted/50 text-muted-foreground'
          fallback.style.width = `${size}px`
          fallback.style.height = `${size}px`
          fallback.style.fontSize = `${size * 0.4}px`
          fallback.textContent = getProviderInitials(provider)
          parent.appendChild(fallback)
        }
      }}
    />
  )
}

// ─── Text Fallback ────────────────────────────────────────────────────────────

function TextFallback({ provider, size }: { provider: string; size: number }) {
  return (
    <div
      className="flex items-center justify-center rounded font-bold bg-muted/50 text-muted-foreground"
      style={{ width: size, height: size, fontSize: size * 0.4 }}
    >
      {getProviderInitials(provider)}
    </div>
  )
}

function getProviderInitials(provider: string): string {
  const normalized = provider.toLowerCase().trim()

  const initialsMap: Record<string, string> = {
    'kiro': 'KI',
    'openai': 'OA',
    'openai-compatible': 'OC',
    'anthropic': 'AN',
    'glm': 'GL',
    'minimax': 'MM',
    'qwen': 'QW',
    'deepseek': 'DS',
    'google': 'GO',
    'groq': 'GQ',
    'mistral': 'MI',
    'cohere': 'CO',
    'ollama': 'OL',
    'openrouter': 'OR',
    'fireworks': 'FW',
    'together': 'TG',
    'huggingface': 'HF',
    'nvidia': 'NV',
    'cerebras': 'CB',
    'nebius': 'NB',
    'xai': 'XA',
    'perplexity': 'PX',
  }

  if (initialsMap[normalized]) return initialsMap[normalized]

  const base = normalized.split('-')[0]
  if (initialsMap[base]) return initialsMap[base]

  const parts = normalized.replace(/[-_]/g, ' ').split(/\s+/)
  if (parts.length >= 2) return (parts[0][0] + parts[1][0]).toUpperCase()

  return normalized.substring(0, 2).toUpperCase()
}
