import { Badge } from '@/components/ui/badge'
import type { ApiKey } from './types'

interface PermissionSummaryProps {
  apiKey: Pick<ApiKey, 'allowedConnectionIds' | 'allowedModels'>
}

export function PermissionSummary({ apiKey }: PermissionSummaryProps) {
  const connectionCount = apiKey.allowedConnectionIds.length
  const modelCount = apiKey.allowedModels.length

  if (connectionCount === 0 && modelCount === 0) {
    return <Badge variant="secondary">Unrestricted</Badge>
  }

  return (
    <div className="flex flex-wrap gap-1.5">
      <Badge variant="outline">{connectionCount || 'Any'} conn</Badge>
      <Badge variant="outline">{modelCount || 'Any'} models</Badge>
    </div>
  )
}
