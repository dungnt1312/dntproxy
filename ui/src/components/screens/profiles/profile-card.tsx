import { Power, PowerOff, Pencil, Trash2, Star, ArrowRight } from 'lucide-react'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import type { ProfileData } from './types'

type ProfileCardProps = {
  profile: ProfileData
  isActive: boolean
  activating: string | null
  onActivate: (name: string) => void
  onDeactivate: () => void
  onEdit: (profile: ProfileData) => void
  onDelete: (profile: ProfileData) => void
}

export function ProfileCard({
  profile,
  isActive,
  activating,
  onActivate,
  onDeactivate,
  onEdit,
  onDelete,
}: ProfileCardProps) {
  const aliasEntries = Object.entries(profile.aliases || {})

  return (
    <Card
      className={isActive ? 'border-violet-500/40 shadow-[0_0_0_1px] shadow-violet-500/20' : ''}
    >
      <CardHeader className="pb-3">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <CardTitle className="truncate text-base">{profile.name}</CardTitle>
              {isActive && (
                <Badge variant="default" className="shrink-0 bg-violet-500 hover:bg-violet-600">
                  <Star className="mr-1 h-3 w-3 fill-current" />
                  Active
                </Badge>
              )}
            </div>
            {profile.description && (
              <p className="mt-1 text-xs text-muted-foreground">{profile.description}</p>
            )}
          </div>
          <div className="flex items-center gap-1.5">
            {!isActive ? (
              <Button
                variant="default"
                size="sm"
                onClick={() => onActivate(profile.name)}
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
                onClick={onDeactivate}
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
              onClick={() => onEdit(profile)}
            >
              <Pencil className="h-3.5 w-3.5" />
            </Button>
            <Button
              variant="outline"
              size="icon"
              className="h-8 w-8 text-destructive hover:text-destructive"
              onClick={() => onDelete(profile)}
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
              <span className="min-w-0 truncate font-mono text-muted-foreground">{model}</span>
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
}
