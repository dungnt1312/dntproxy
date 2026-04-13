import React, { useEffect, useState, useRef } from 'react';
import { Terminal, RefreshCw, AlertCircle, Search, Download } from 'lucide-react';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { ScrollArea } from '@/components/ui/scroll-area';
import { cn } from '@/lib/utils';
import { api } from '../../api'; // Make sure this matches your api configuration

export interface LogFilter {
    connectionId?: string;
    model?: string;
    comboName?: string;
    aliasName?: string;
}

interface LogsViewerModalProps {
    isOpen: boolean;
    onClose: () => void;
    title: string;
    filter: LogFilter;
}

export default function LogsViewerModal({ isOpen, onClose, title, filter }: LogsViewerModalProps) {
    const [logs, setLogs] = useState<any[]>([]);
    const [isLoading, setIsLoading] = useState(false);
    const [searchQuery, setSearchQuery] = useState("");
    const bottomRef = useRef<HTMLDivElement>(null);

    const fetchLogs = async () => {
        setIsLoading(true);
        try {
            // Assuming your backend supports fetching logs via api.getLogs()
            // We pass the filter object as params.
            const queryParams = new URLSearchParams(filter as any).toString();
            // TODO: adjust to your real api method!
            // const data = await api.getLogs(queryParams); 
            // setLogs(data);

            // Mock Data for now so it doesn't crash if endpoint is missing:
            setTimeout(() => {
                setLogs([
                    { id: 1, time: new Date(Date.now() - 50000).toISOString(), level: 'info', message: 'Connection initialized.' },
                    { id: 2, time: new Date(Date.now() - 40000).toISOString(), level: 'warn', message: 'Rate limit approaching (80%).' },
                    { id: 3, time: new Date(Date.now() - 10000).toISOString(), level: 'error', message: 'HTTP 429 Too Many Requests: Backoff initiated Level 1' },
                    { id: 4, time: new Date(Date.now() - 5000).toISOString(), level: 'info', message: 'Re-authenticating token...' },
                    { id: 5, time: new Date(Date.now() - 1000).toISOString(), level: 'error', message: 'Authentication failed: Invalid credentials.' }
                ]);
                setIsLoading(false);
            }, 600);
        } catch (error) {
            console.error("Failed to load logs:", error);
            setIsLoading(false);
        }
    };

    useEffect(() => {
        if (isOpen) {
            fetchLogs();
        } else {
            setLogs([]);
            setSearchQuery("");
        }
    }, [isOpen, filter]);

    useEffect(() => {
        if (bottomRef.current) {
            bottomRef.current.scrollIntoView({ behavior: 'smooth' });
        }
    }, [logs]);

    const filteredLogs = logs.filter(log => 
        log.message.toLowerCase().includes(searchQuery.toLowerCase()) || 
        log.level.toLowerCase().includes(searchQuery.toLowerCase())
    );

    const getLogColor = (level: string) => {
        switch (level) {
            case 'error': return 'text-red-400';
            case 'warn': return 'text-amber-400';
            case 'info': return 'text-emerald-400';
            default: return 'text-slate-300';
        }
    };

    return (
        <Dialog open={isOpen} onOpenChange={(open) => !open && onClose()}>
            <DialogContent className="max-w-3xl h-[80vh] flex flex-col p-0 gap-0 overflow-hidden bg-background border-border">
                <DialogHeader className="p-4 border-b bg-muted/30 flex flex-row items-center justify-between space-y-0">
                    <div className="flex items-center gap-2">
                        <Terminal className="h-5 w-5 text-muted-foreground" />
                        <DialogTitle className="text-base font-semibold">{title}</DialogTitle>
                    </div>
                </DialogHeader>

                <div className="flex items-center justify-between p-2 px-4 border-b bg-muted/10 gap-4">
                    <div className="relative flex-1 max-w-sm">
                        <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
                        <Input 
                            placeholder="Filter logs..." 
                            value={searchQuery}
                            onChange={(e) => setSearchQuery(e.target.value)}
                            className="h-8 pl-8 text-xs bg-background"
                            autoComplete="off"
                            data-1p-ignore
                        />
                    </div>
                    <div className="flex items-center gap-2">
                        <Button variant="outline" size="sm" className="h-8 px-2 text-xs" onClick={fetchLogs} disabled={isLoading}>
                            <RefreshCw className={cn("h-3.5 w-3.5 mr-1.5", isLoading && "animate-spin")} /> Refresh
                        </Button>
                        <Button variant="ghost" size="sm" className="h-8 px-2 text-xs" title="Export Logs">
                            <Download className="h-3.5 w-3.5" />
                        </Button>
                    </div>
                </div>

                <ScrollArea className="flex-1 bg-[#0f111a] font-mono text-[13px] text-slate-300 p-4">
                    {isLoading && logs.length === 0 ? (
                        <div className="flex items-center justify-center h-full text-slate-500 gap-2">
                            <RefreshCw className="h-4 w-4 animate-spin" /> Loading system logs...
                        </div>
                    ) : filteredLogs.length === 0 ? (
                        <div className="flex flex-col items-center justify-center h-full text-slate-500 gap-2 opacity-50 pt-10">
                            <AlertCircle className="h-8 w-8" />
                            <span>No logs found for this filter.</span>
                        </div>
                    ) : (
                        <div className="flex flex-col space-y-1 pb-4">
                            {filteredLogs.map((log) => (
                                <div key={log.id} className="flex items-start break-words hover:bg-white/5 p-0.5 rounded transition-colors group">
                                    <span className="text-slate-500 shrink-0 mr-3 select-none">
                                        [{new Date(log.time).toLocaleTimeString()}]
                                    </span>
                                    <span className={cn("shrink-0 uppercase mr-3 w-12 font-semibold", getLogColor(log.level))}>
                                        {log.level}
                                    </span>
                                    <span className={cn(log.level === 'error' && 'text-red-300')}>
                                        {log.message}
                                    </span>
                                </div>
                            ))}
                            <div ref={bottomRef} />
                        </div>
                    )}
                </ScrollArea>
            </DialogContent>
        </Dialog>
    );
}
