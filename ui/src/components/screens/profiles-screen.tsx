import { useEffect, useState } from 'react'
import { UserCircle, Plus, Sparkles, Star, PowerOff } from 'lucide-react'
import { toast } from 'sonner'

import { goApi } from '@/lib/go-api'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
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
import { ProfileCard } from './profiles/profile-card'
import { CreateProfileDialog } from './profiles/create-profile-dialog'
import { EditProfileDialog } from './profiles/edit-profile-dialog'
import { PresetDialog } from './profiles/preset-dialog'
import type { ProfileData, PresetData } from './profiles/types'

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
  const [editingProfile, setEditingProfile] = useState<ProfileData | null>(null)

  // Loading states
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

  function openCreateDialog() {
    setCreateDialogOpen(true)
  }

  function openPresetDialog() {
    setPresetDialogOpen(true)
  }

  function openEditDialog(profile: ProfileData) {
    setEditingProfile(profile)
    setEditDialogOpen(true)
  }

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
          {profiles.map((profile) => (
            <ProfileCard
              key={profile.id}
              profile={profile}
              isActive={profile.name === activeProfile}
              activating={activating}
              onActivate={handleActivate}
              onDeactivate={handleDeactivate}
              onEdit={openEditDialog}
              onDelete={setDeleteTarget}
            />
          ))}
        </div>
      )}

      {/* Dialogs */}
      <CreateProfileDialog
        open={createDialogOpen}
        onOpenChange={setCreateDialogOpen}
        onSuccess={load}
      />

      <PresetDialog
        open={presetDialogOpen}
        presets={presets}
        onOpenChange={setPresetDialogOpen}
        onSuccess={load}
      />

      <EditProfileDialog
        open={editDialogOpen}
        profile={editingProfile}
        activeProfile={activeProfile}
        onOpenChange={setEditDialogOpen}
        onSuccess={load}
      />

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
