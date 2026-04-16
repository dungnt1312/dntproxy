import { Terminal } from 'lucide-react';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import LogsScreen from '../screens/logs-screen';

export interface LogFilter {
    connectionId?: string;
    model?: string;
    comboName?: string;
    aliasName?: string;
    allowedProviders?: string[];
}

interface LogsViewerModalProps {
    isOpen: boolean;
    onClose: () => void;
    title: string;
    filter: LogFilter;
}

export default function LogsViewerModal({ isOpen, onClose, title, filter }: LogsViewerModalProps) {
    if (!isOpen) return null; // Unmount when closed so state resets

    const initialFilters = {
        ...(filter.connectionId ? { connectionId: filter.connectionId } : {}),
        ...((filter.model || filter.comboName || filter.aliasName) ? { q: filter.model || filter.comboName || filter.aliasName } : {}),
    }

    const hiddenFilters: any[] = [];
    if (filter.connectionId) hiddenFilters.push("connectionId");

    return (
        <Dialog open={isOpen} onOpenChange={(open) => !open && onClose()}>
            <DialogContent className="!max-w-[95vw] md:!max-w-[1200px] h-[85vh] flex flex-col p-0 gap-0 overflow-hidden bg-background border-border">
                <DialogHeader className="p-4 border-b bg-muted/40 flex flex-row items-center space-y-0 shadow-sm shrink-0">
                    <div className="flex items-center gap-2">
                        <Terminal className="h-5 w-5 text-muted-foreground" />
                        <DialogTitle className="text-base font-semibold">{title}</DialogTitle>
                    </div>
                </DialogHeader>

                <div className="flex-1 overflow-hidden relative">
                    <LogsScreen 
                        embedded={true} 
                        initialFilters={initialFilters} 
                        hiddenFilters={hiddenFilters}
                        allowedProviders={filter.allowedProviders}
                    />
                </div>
            </DialogContent>
        </Dialog>
    );
}
