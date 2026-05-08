import { Eye, EyeOff, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { cn } from '@/lib/utils'
import { CopyButton } from './copy-button'

export interface ApiKey {
  id: string
  name: string
  key: string
  isActive: boolean
  createdAt: string
  updatedAt: string
}

interface KeysTableProps {
  keys: ApiKey[]
  revealedKeys: Set<string>
  onToggleReveal: (id: string) => void
  onDelete: (key: ApiKey) => void
  formatDate: (dateStr: string) => string
  maskKey: (key: string) => string
}

export function KeysTable({
  keys,
  revealedKeys,
  onToggleReveal,
  onDelete,
  formatDate,
  maskKey,
}: KeysTableProps) {
  return (
    <div className="hidden overflow-x-auto md:block">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
            <TableHead>Key</TableHead>
            <TableHead>Status</TableHead>
            <TableHead>Created</TableHead>
            <TableHead className="text-right">Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {keys.map((apiKey) => {
            const isRevealed = revealedKeys.has(apiKey.id)
            return (
              <TableRow key={apiKey.id}>
                <TableCell>
                  <div className="flex items-center gap-2">
                    <div className={cn(
                      'size-2 rounded-full shrink-0',
                      apiKey.isActive ? 'bg-emerald-500' : 'bg-gray-400'
                    )} />
                    <span className="font-medium">{apiKey.name}</span>
                  </div>
                </TableCell>
                <TableCell>
                  <div className="flex items-center gap-1">
                    <code className="text-xs bg-muted px-2 py-1 rounded font-mono max-w-[280px] truncate select-all">
                      {isRevealed ? apiKey.key : maskKey(apiKey.key)}
                    </code>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="size-7"
                          onClick={() => onToggleReveal(apiKey.id)}
                        >
                          {isRevealed ? <EyeOff className="size-3" /> : <Eye className="size-3" />}
                        </Button>
                      </TooltipTrigger>
                      <TooltipContent>{isRevealed ? 'Hide key' : 'Reveal key'}</TooltipContent>
                    </Tooltip>
                    <CopyButton text={apiKey.key} label="Key" />
                  </div>
                </TableCell>
                <TableCell>
                  <Badge
                    variant="outline"
                    className={cn(
                      'font-medium',
                      apiKey.isActive
                        ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20'
                        : 'bg-gray-500/10 text-gray-500 dark:text-gray-400 border-gray-500/20'
                    )}
                  >
                    {apiKey.isActive ? 'Active' : 'Inactive'}
                  </Badge>
                </TableCell>
                <TableCell className="text-muted-foreground text-sm">
                  {formatDate(apiKey.createdAt)}
                </TableCell>
                <TableCell className="text-right">
                  <div className="flex items-center justify-end gap-1">
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="size-8 text-destructive hover:text-destructive hover:bg-destructive/10"
                          onClick={() => onDelete(apiKey)}
                        >
                          <Trash2 className="size-3.5" />
                        </Button>
                      </TooltipTrigger>
                      <TooltipContent>Delete key</TooltipContent>
                    </Tooltip>
                  </div>
                </TableCell>
              </TableRow>
            )
          })}
        </TableBody>
      </Table>
    </div>
  )
}
