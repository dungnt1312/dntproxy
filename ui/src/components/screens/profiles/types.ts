export type ProfileData = {
  id: string
  name: string
  description: string
  aliases: Record<string, string>
  combos?: Array<{ name: string; models: string[] }>
  createdAt?: string
  updatedAt?: string
}

export type PresetData = {
  name: string
  description: string
  aliases: Record<string, string>
}

export type AliasRow = {
  alias: string
  model: string
}
