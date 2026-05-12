import { Link2, AlertTriangle, CircleOff } from 'lucide-react';
import { Card, CardContent } from '@/components/ui/card';
import { cn } from '@/lib/utils';

interface ConnectionStatsProps {
    total: number;
    active: number;
    inactive: number;
    needsAttention: number;
}

/**
 * Displays connection statistics in four cards: Total, Active, Inactive, and Issues.
 */
export function ConnectionStats({ total, active, inactive, needsAttention }: ConnectionStatsProps) {
    return (
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
            <Card>
                <CardContent className="p-4">
                    <div className="flex items-center gap-2 text-xs text-muted-foreground mb-1">
                        <Link2 size={12} /> Total Connections
                    </div>
                    <div className="text-2xl font-bold">{total}</div>
                </CardContent>
            </Card>
            <Card>
                <CardContent className="p-4">
                    <div className="flex items-center gap-2 text-xs text-emerald-600 mb-1">
                        <span className="h-2 w-2 rounded-full bg-emerald-500" /> Active
                    </div>
                    <div className="text-2xl font-bold text-emerald-600">{active}</div>
                </CardContent>
            </Card>
            <Card>
                <CardContent className="p-4">
                    <div className="flex items-center gap-2 text-xs text-muted-foreground mb-1">
                        <CircleOff size={12} /> Inactive
                    </div>
                    <div className="text-2xl font-bold text-muted-foreground">{inactive}</div>
                </CardContent>
            </Card>
            <Card className={needsAttention > 0 ? 'border-destructive/40' : ''}>
                <CardContent className="p-4">
                    <div
                        className={cn(
                            'flex items-center gap-2 text-xs mb-1',
                            needsAttention > 0 ? 'text-destructive' : 'text-muted-foreground',
                        )}
                    >
                        <AlertTriangle size={12} /> Issues
                    </div>
                    <div
                        className={cn(
                            'text-2xl font-bold',
                            needsAttention > 0 ? 'text-destructive' : 'text-muted-foreground',
                        )}
                    >
                        {needsAttention}
                    </div>
                </CardContent>
            </Card>
        </div>
    );
}
