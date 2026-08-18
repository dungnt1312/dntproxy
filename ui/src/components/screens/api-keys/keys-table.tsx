import { Edit3, Trash2 } from 'lucide-react'
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
import { PermissionSummary } from './permission-summary'
import type { ApiKey } from './types'

interface KeysTableProps {
  keys: ApiKey[]
  revealedKeys: Set<string>
  onToggleReveal: (id: string) => void
  onDelete: (key: ApiKey) => void
  onEdit: (key: ApiKey) => void
  formatDate: (dateStr: string) => string
  maskKey: (key: string) => string
}

export function KeysTable({
  keys,
  revealedKeys,
  onToggleReveal,
  onDelete,
  onEdit,
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
            <TableHead>Tenant</TableHead>
            <TableHead>Access</TableHead>
            <TableHead>Created</TableHead>
            <TableHead className="text-right">Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {keys.map((apiKey) => {
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
                      {maskKey(apiKey.key)}
                    </code>
                  </div>
                </TableCell>
                <TableCell>
                  <div className="flex items-center gap-1.5 flex-wrap">
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
                    {apiKey.dashboardAccess && (
                      <Badge
                        variant="outline"
                        className="font-medium bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/20"
                      >
                        Dashboard
                      </Badge>
                    )}
                  </div>
                </TableCell>
                <TableCell>
                  <PermissionSummary apiKey={apiKey} />
                </TableCell>
                <TableCell>
                  <span className="text-xs text-muted-foreground">
                    {apiKey.tenantId ? apiKey.tenantId : '—'}
                  </span>
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
                          className="size-8"
                          onClick={() => onEdit(apiKey)}
                        >
                          <Edit3 className="size-3.5" />
                        </Button>
                      </TooltipTrigger>
                      <TooltipContent>Edit key</TooltipContent>
                    </Tooltip>
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
