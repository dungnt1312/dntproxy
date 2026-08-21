import { Loader2, Trash2 } from 'lucide-react';
import {
    AlertDialog,
    AlertDialogContent,
    AlertDialogDescription,
    AlertDialogFooter,
    AlertDialogHeader,
    AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Button } from '@/components/ui/button';

interface BulkDeleteDialogProps {
    names: string[];
    busy: boolean;
    onConfirm: () => void;
    onClose: () => void;
}

/** Confirmation for deleting many connections at once, with a name preview. */
export function BulkDeleteDialog({ names, busy, onConfirm, onClose }: BulkDeleteDialogProps) {
    const preview = names.slice(0, 6);
    const rest = names.length - preview.length;

    return (
        <AlertDialog open onOpenChange={(open) => { if (!open && !busy) onClose(); }}>
            <AlertDialogContent>
                <AlertDialogHeader>
                    <AlertDialogTitle>Remove {names.length} Connections</AlertDialogTitle>
                    <AlertDialogDescription asChild>
                        <div className="space-y-2">
                            <span>
                                Are you sure you want to permanently delete{' '}
                                <span className="font-semibold text-foreground">
                                    {names.length} connection{names.length === 1 ? '' : 's'}
                                </span>
                                ? All tokens, settings, and quota data will be lost.
                            </span>
                            <ul className="list-disc pl-5 text-xs text-muted-foreground">
                                {preview.map((n) => (
                                    <li key={n} className="truncate">
                                        {n}
                                    </li>
                                ))}
                                {rest > 0 && <li>and {rest} more…</li>}
                            </ul>
                        </div>
                    </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                    <Button variant="outline" onClick={onClose} disabled={busy}>
                        Cancel
                    </Button>
                    <Button variant="destructive" onClick={onConfirm} disabled={busy} className="gap-2">
                        {busy ? <Loader2 size={14} className="animate-spin" /> : <Trash2 size={14} />}
                        {busy ? 'Removing…' : `Remove ${names.length}`}
                    </Button>
                </AlertDialogFooter>
            </AlertDialogContent>
        </AlertDialog>
    );
}
