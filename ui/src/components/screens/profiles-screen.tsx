import { useEffect, useState } from 'react'
import {
  UserCircle,
  Plus,
  Trash2,
  Power,
  PowerOff,
  ArrowRight,
  Sparkles,
  Star,
  Download,
  Upload,
  Pencil,
} from 'lucide-react'
import { toast } from 'sonner'

import { goApi } from '@/lib/go-api'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

type ProfileData = {
  id: string
  name: string
  description: string
  aliases: Record<string, string>
  combos?: Array<{ name: string; models: string[] }>
  createdAt?: string
  updatedAt?: string
}

type PresetData = {
  name: string
  description: string
  aliases: Record<string, string>
}

export default function ProfilesScreen() {
  const [profiles, setProfiles] = useState<ProfileData[]>([])
  const [activeProfile, setActiveProfile] = useState('')
  const [presets, setPresets] = useState<PresetData[]>([])
  const [loading, setLoading] = useState(true)

  // Dialog states
  const [createDialogOpen, setCreateDialogOpen] = useState(false)
  const [presetDialogOpen, setPresetDialogOpen] = useState(false)
  const [editDialogOpen, setEditDialogOpen] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<ProfileData | null>(null)

  // Form states
  const [formName, setFormName] = useState('')
  const [formDescription, setFormDescription] = useState('')
  const [formAliases, setFormAliases] = useState<Array<{ alias: string; model: string }>>([
    { alias: '', model: '' },
  ])
  const [selectedPreset, setSelectedPreset] = useState('')
  const [editingProfile, setEditingProfile] = useState<ProfileData | null>(null)

  // Loading states
  const [saving, setSaving] = useState(false)
  const [activating, setActivating] = useState<string | null>(null)
  const [deleting, setDeleting] = useState(false)

  async function load() {
    setLoading(true)
    try {
      const [profileData, presetData] = await Promise.all([
        goApi.getProfiles(),
        goApi.getProfilePresets(),
      ])
      setProfiles(profileData.profiles || [])
      setActiveProfile(profileData.activeProfile || '')
      setPresets(Array.isArray(presetData) ? presetData : [])
    } catch {
      toast.error('Failed to load profiles')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  // --- Create Profile ---
  function openCreateDialog() {
    setFormName('')
    setFormDescription('')
    setFormAliases([{ alias: '', model: '' }])
    setCreateDialogOpen(true)
  }

  function addAliasRow() {
    setFormAliases((prev) => [...prev, { alias: '', model: '' }])
  }

  function removeAliasRow(index: number) {
    setFormAliases((prev) => prev.filter((_, i) => i !== index))
  }

  function updateAliasRow(index: number, field: 'alias' | 'model', value: string) {
    setFormAliases((prev) => prev.map((row, i) => (i === index ? { ...row, [field]: value } : row)))
  }

  async function handleCreate() {
    const name = formName.trim()
    if (!name) {
      toast.error('Profile name is required')
      return
    }

    const aliases: Record<string, string> = {}
    for (const row of formAliases) {
      const a = row.alias.trim()
      const m = row.model.trim()
      if (a && m) aliases[a] = m
    }

    if (Object.keys(aliases).length === 0) {
      toast.error('At least one alias mapping is required')
      return
    }

    setSaving(true)
    try {
      await goApi.createProfile({ name, description: formDescription.trim(), aliases })
      toast.success(`Profile "${name}" created`)
      setCreateDialogOpen(false)
      await load()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Failed to create profile')
    } finally {
      setSaving(false)
    }
  }

  // --- Create from Preset ---
  function openPresetDialog() {
    setSelectedPreset('')
    setPresetDialogOpen(true)
  }

  async function handleCreateFromPreset() {
    if (!selectedPreset) {
      toast.error('Select a preset')
      return
    }

    setSaving(true)
    try {
      await goApi.createProfileFromPreset(selectedPreset)
      toast.success(`Profile created from preset "${selectedPreset}"`)
      setPresetDialogOpen(false)
      await load()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Failed to create from preset')
    } finally {
      setSaving(false)
    }
  }

  // --- Edit Profile ---
  function openEditDialog(profile: ProfileData) {
    setEditingProfile(profile)
    setFormName(profile.name)
    setFormDescription(profile.description || '')
    const aliasRows = Object.entries(profile.aliases || {}).map(([alias, model]) => ({
      alias,
      model,
    }))
    setFormAliases(aliasRows.length > 0 ? aliasRows : [{ alias: '', model: '' }])
    setEditDialogOpen(true)
  }

  async function handleEdit() {
    if (!editingProfile) return

    const addAliases: Record<string, string> = {}
    const currentAliasKeys = new Set<string>()

    for (const row of formAliases) {
      const a = row.alias.trim()
      const m = row.model.trim()
      if (a && m) {
        addAliases[a] = m
        currentAliasKeys.add(a)
      }
    }

    // Find removed aliases
    const removeAliases = Object.keys(editingProfile.aliases || {}).filter(
      (key) => !currentAliasKeys.has(key),
    )

    setSaving(true)
    try {
      await goApi.updateProfile(editingProfile.name, { addAliases, removeAliases })
      toast.success(`Profile "${editingProfile.name}" updated`)
      setEditDialogOpen(false)
      await load()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Failed to update profile')
    } finally {
      setSaving(false)
    }
  }

  // --- Activate / Deactivate ---
  async function handleActivate(name: string) {
    setActivating(name)
    try {
      await goApi.activateProfile(name)
      toast.success(`Profile "${name}" activated`)
      await load()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Failed to activate')
    } finally {
      setActivating(null)
    }
  }

  async function handleDeactivate() {
    setActivating('__deactivate__')
    try {
      await goApi.deactivateProfile()
      toast.success('Profile deactivated')
      await load()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Failed to deactivate')
    } finally {
      setActivating(null)
    }
  }

  // --- Delete ---
  async function handleDelete() {
    if (!deleteTarget) return
    setDeleting(true)
    try {
      await goApi.deleteProfile(deleteTarget.name)
      toast.success(`Profile "${deleteTarget.name}" deleted`)
      setDeleteTarget(null)
      await load()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Failed to delete')
    } finally {
      setDeleting(false)
    }
  }

  // --- Alias editor rows component ---
  const AliasEditor = () => (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <Label>Alias Mappings</Label>
        <Button type="button" variant="outline" size="sm" onClick={addAliasRow} className="gap-1.5">
          <Plus className="h-3 w-3" />
          Add Row
        </Button>
      </div>
      <div className="space-y-2 rounded-lg border bg-muted/20 p-3">
        {formAliases.length === 0 && (
          <p className="py-2 text-center text-sm text-muted-foreground">
            No aliases. Click "Add Row" to start.
          </p>
        )}
        {formAliases.map((row, index) => (
          <div key={index} className="flex items-center gap-2">
            <Input
              value={row.alias}
              onChange={(e) => updateAliasRow(index, 'alias', e.target.value)}
              placeholder="claude-sonnet"
              className="flex-1 font-mono text-xs"
            />
            <ArrowRight className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
            <Input
              value={row.model}
              onChange={(e) => updateAliasRow(index, 'model', e.target.value)}
              placeholder="kr/claude-sonnet-4.5"
              className="flex-1 font-mono text-xs"
            />
            <Button
              type="button"
              variant="ghost"
              size="icon"
              className="h-8 w-8 shrink-0 text-muted-foreground hover:text-destructive"
              onClick={() => removeAliasRow(index)}
            >
              <Trash2 className="h-3.5 w-3.5" />
            </Button>
          </div>
        ))}
      </div>
      <p className="text-xs text-muted-foreground">
        Map model names (as sent by CLI tools) to provider/model targets in dntproxy.
      </p>
    </div>
  )

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-3">
          <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-violet-500/10">
            <UserCircle className="h-5 w-5 text-violet-500" />
          </div>
          <div>
            <h1 className="text-2xl font-bold tracking-tight">Profiles</h1>
            <p className="text-sm text-muted-foreground">
              Named alias sets for quick provider switching. Activate a profile to route CLI tools
              through different providers.
            </p>
          </div>
        </div>
        <div className="flex items-center gap-2 self-start sm:self-auto">
          <Button variant="outline" onClick={openPresetDialog} className="gap-2">
            <Sparkles className="h-4 w-4" />
            From Preset
          </Button>
          <Button onClick={openCreateDialog} className="gap-2">
            <Plus className="h-4 w-4" />
            Create Profile
          </Button>
        </div>
      </div>

      {/* Active profile banner */}
      {activeProfile && (
        <div className="flex items-center justify-between rounded-lg border border-violet-500/30 bg-violet-500/5 px-4 py-3">
          <div className="flex items-center gap-3">
            <Star className="h-5 w-5 fill-violet-500 text-violet-500" />
            <div>
              <p className="text-sm font-medium">
                Active Profile: <span className="text-violet-500">{activeProfile}</span>
              </p>
              <p className="text-xs text-muted-foreground">
                Model aliases from this profile are currently routing requests.
              </p>
            </div>
          </div>
          <Button
            variant="outline"
            size="sm"
            onClick={handleDeactivate}
            disabled={activating === '__deactivate__'}
            className="gap-1.5"
          >
            <PowerOff className="h-3.5 w-3.5" />
            {activating === '__deactivate__' ? 'Deactivating…' : 'Deactivate'}
          </Button>
        </div>
      )}

      {/* Content */}
      {loading ? (
        <Card>
          <CardContent className="p-6 text-sm text-muted-foreground">
            Loading profiles...
          </CardContent>
        </Card>
      ) : profiles.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-lg border border-dashed py-16 text-center">
          <UserCircle className="mb-4 h-12 w-12 text-muted-foreground/40" />
          <h3 className="mb-1 text-lg font-medium">No profiles yet</h3>
          <p className="mb-4 max-w-md text-sm text-muted-foreground">
            Profiles group model aliases for quick provider switching. Create one from a preset or
            build your own.
          </p>
          <div className="flex gap-2">
            <Button onClick={openPresetDialog} variant="outline" className="gap-2">
              <Sparkles className="h-4 w-4" />
              From Preset
            </Button>
            <Button onClick={openCreateDialog} className="gap-2">
              <Plus className="h-4 w-4" />
              Create Profile
            </Button>
          </div>
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
          {profiles.map((profile) => {
            const isActive = profile.name === activeProfile
            const aliasEntries = Object.entries(profile.aliases || {})
            return (
              <Card
                key={profile.id}
                className={
                  isActive ? 'border-violet-500/40 shadow-[0_0_0_1px] shadow-violet-500/20' : ''
                }
              >
                <CardHeader className="pb-3">
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
                        <CardTitle className="truncate text-base">{profile.name}</CardTitle>
                        {isActive && (
                          <Badge
                            variant="default"
                            className="shrink-0 bg-violet-500 hover:bg-violet-600"
                          >
                            <Star className="mr-1 h-3 w-3 fill-current" />
                            Active
                          </Badge>
                        )}
                      </div>
                      {profile.description && (
                        <p className="mt-1 text-xs text-muted-foreground">
                          {profile.description}
                        </p>
                      )}
                    </div>
                    <div className="flex items-center gap-1.5">
                      {!isActive ? (
                        <Button
                          variant="default"
                          size="sm"
                          onClick={() => handleActivate(profile.name)}
                          disabled={activating === profile.name}
                          className="gap-1.5 bg-violet-600 hover:bg-violet-700"
                        >
                          <Power className="h-3.5 w-3.5" />
                          {activating === profile.name ? 'Activating…' : 'Activate'}
                        </Button>
                      ) : (
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={handleDeactivate}
                          disabled={activating === '__deactivate__'}
                          className="gap-1.5"
                        >
                          <PowerOff className="h-3.5 w-3.5" />
                          {activating === '__deactivate__' ? '…' : 'Deactivate'}
                        </Button>
                      )}
                      <Button
                        variant="outline"
                        size="icon"
                        className="h-8 w-8"
                        onClick={() => openEditDialog(profile)}
                      >
                        <Pencil className="h-3.5 w-3.5" />
                      </Button>
                      <Button
                        variant="outline"
                        size="icon"
                        className="h-8 w-8 text-destructive hover:text-destructive"
                        onClick={() => setDeleteTarget(profile)}
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    </div>
                  </div>
                </CardHeader>
                <CardContent>
                  <div className="space-y-1.5 rounded-md border bg-muted/20 p-3">
                    <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                      Aliases ({aliasEntries.length})
                    </p>
                    {aliasEntries.slice(0, 6).map(([alias, model]) => (
                      <div key={alias} className="flex items-center gap-2 text-xs">
                        <span className="min-w-0 truncate font-mono text-foreground">{alias}</span>
                        <ArrowRight className="h-3 w-3 shrink-0 text-muted-foreground/60" />
                        <span className="min-w-0 truncate font-mono text-muted-foreground">
                          {model}
                        </span>
                      </div>
                    ))}
                    {aliasEntries.length > 6 && (
                      <p className="pt-1 text-xs text-muted-foreground">
                        +{aliasEntries.length - 6} more aliases
                      </p>
                    )}
                  </div>
                </CardContent>
              </Card>
            )
          })}
        </div>
      )}

      {/* Create Profile Dialog */}
      <Dialog open={createDialogOpen} onOpenChange={setCreateDialogOpen}>
        <DialogContent className="sm:max-w-xl">
          <DialogHeader>
            <DialogTitle>Create Profile</DialogTitle>
            <DialogDescription>
              Define model alias mappings. When activated, these aliases route CLI tool requests to
              your chosen providers.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label htmlFor="profile-name">Name</Label>
                <Input
                  id="profile-name"
                  value={formName}
                  onChange={(e) => setFormName(e.target.value)}
                  placeholder="my-profile"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="profile-desc">Description</Label>
                <Input
                  id="profile-desc"
                  value={formDescription}
                  onChange={(e) => setFormDescription(e.target.value)}
                  placeholder="Optional description"
                />
              </div>
            </div>
            <AliasEditor />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateDialogOpen(false)}>
              Cancel
            </Button>
            <Button onClick={handleCreate} disabled={saving}>
              {saving ? 'Creating…' : 'Create Profile'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Create from Preset Dialog */}
      <Dialog open={presetDialogOpen} onOpenChange={setPresetDialogOpen}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>Create from Preset</DialogTitle>
            <DialogDescription>
              Choose a built-in preset to quickly create a profile with pre-configured alias
              mappings.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-2">
              <Label>Preset</Label>
              <Select value={selectedPreset} onValueChange={setSelectedPreset}>
                <SelectTrigger>
                  <SelectValue placeholder="Select a preset…" />
                </SelectTrigger>
                <SelectContent>
                  {presets.map((preset) => (
                    <SelectItem key={preset.name} value={preset.name}>
                      <span className="font-medium">{preset.name}</span>
                      <span className="ml-2 text-muted-foreground">— {preset.description}</span>
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            {selectedPreset && (
              <div className="space-y-2 rounded-lg border bg-muted/20 p-3">
                <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                  Preview
                </p>
                {presets
                  .find((p) => p.name === selectedPreset)
                  ?.aliases &&
                  Object.entries(
                    presets.find((p) => p.name === selectedPreset)!.aliases,
                  ).map(([alias, model]) => (
                    <div key={alias} className="flex items-center gap-2 text-xs">
                      <span className="font-mono text-foreground">{alias}</span>
                      <ArrowRight className="h-3 w-3 shrink-0 text-muted-foreground/60" />
                      <span className="font-mono text-muted-foreground">{model}</span>
                    </div>
                  ))}
              </div>
            )}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setPresetDialogOpen(false)}>
              Cancel
            </Button>
            <Button onClick={handleCreateFromPreset} disabled={saving || !selectedPreset}>
              {saving ? 'Creating…' : 'Create from Preset'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Edit Profile Dialog */}
      <Dialog open={editDialogOpen} onOpenChange={setEditDialogOpen}>
        <DialogContent className="sm:max-w-xl">
          <DialogHeader>
            <DialogTitle>Edit Profile: {editingProfile?.name}</DialogTitle>
            <DialogDescription>
              Update the alias mappings for this profile.
              {editingProfile?.name === activeProfile &&
                ' Changes will immediately affect active routing.'}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <AliasEditor />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setEditDialogOpen(false)}>
              Cancel
            </Button>
            <Button onClick={handleEdit} disabled={saving}>
              {saving ? 'Saving…' : 'Save Changes'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete Confirmation */}
      <AlertDialog
        open={!!deleteTarget}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete Profile</AlertDialogTitle>
            <AlertDialogDescription>
              Delete &quot;{deleteTarget?.name}&quot;?
              {deleteTarget?.name === activeProfile &&
                ' This profile is currently active — it will be deactivated and aliases removed.'}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleting}>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={(e) => {
                e.preventDefault()
                handleDelete()
              }}
              disabled={deleting}
              className="bg-destructive text-white hover:bg-destructive/90"
            >
              {deleting ? 'Deleting…' : 'Delete'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
