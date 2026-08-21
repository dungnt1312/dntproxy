import { useState } from 'react';
import { Upload } from 'lucide-react';
import { cn } from '@/lib/utils';

type Props = {
    accept?: string;
    multiple?: boolean;
    disabled?: boolean;
    title: string;
    hint?: string;
    ariaLabel: string;
    onFiles: (files: File[]) => void;
};

export function FileDropzone({
    accept = '.json,application/json',
    multiple = false,
    disabled = false,
    title,
    hint,
    ariaLabel,
    onFiles,
}: Props) {
    const [dragActive, setDragActive] = useState(false);

    const takeFiles = (list: FileList | null) => {
        if (disabled || !list || list.length === 0) return;
        onFiles(Array.from(list));
    };

    return (
        <label
            tabIndex={disabled ? -1 : 0}
            role="button"
            aria-label={ariaLabel}
            aria-disabled={disabled}
            onKeyDown={(event) => {
                if (disabled || (event.key !== 'Enter' && event.key !== ' ')) return;
                event.preventDefault();
                (event.currentTarget.querySelector('input[type="file"]') as HTMLInputElement | null)?.click();
            }}
            onDragOver={(event) => {
                if (disabled) return;
                event.preventDefault();
                setDragActive(true);
            }}
            onDragLeave={() => setDragActive(false)}
            onDrop={(event) => {
                if (disabled) {
                    setDragActive(false);
                    return;
                }
                event.preventDefault();
                setDragActive(false);
                takeFiles(event.dataTransfer.files);
            }}
            className={cn(
                'flex flex-col items-center justify-center gap-3 rounded-xl border-2 border-dashed p-8 outline-none transition focus-visible:ring-2 focus-visible:ring-primary/40',
                disabled ? 'cursor-not-allowed opacity-50' : 'cursor-pointer hover:border-primary hover:bg-muted/40',
                !disabled && dragActive && 'border-primary bg-muted/40',
            )}
        >
            <Upload size={24} className="text-muted-foreground" />
            <div className="text-center">
                <p className="text-sm font-medium">{title}</p>
                {hint && <p className="mt-0.5 text-xs text-muted-foreground">{hint}</p>}
            </div>
            <input
                type="file"
                accept={accept}
                multiple={multiple}
                disabled={disabled}
                className="hidden"
                onChange={(event) => {
                    takeFiles(event.currentTarget.files);
                    event.currentTarget.value = '';
                }}
            />
        </label>
    );
}
