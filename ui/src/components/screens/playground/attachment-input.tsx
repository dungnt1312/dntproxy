import { useCallback, useEffect, useRef, useState } from "react";
import { Image, Paperclip, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";

export interface Attachment {
  id: string;
  type: "image" | "file";
  name: string;
  size: number;
  dataUrl: string;
  mimeType: string;
}

interface AttachmentInputProps {
  attachments: Attachment[];
  onAttachmentsChange: (attachments: Attachment[]) => void;
  disabled?: boolean;
}

const MAX_FILE_SIZE = 10 * 1024 * 1024; // 10MB
const MAX_TOTAL_SIZE = 15 * 1024 * 1024; // 15MB
const MAX_ATTACHMENTS = 4;
const SUPPORTED_IMAGE_TYPES = ["image/png", "image/jpeg", "image/gif", "image/webp"];

export function AttachmentInput({
  attachments,
  onAttachmentsChange,
  disabled,
}: AttachmentInputProps) {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [isDragging, setIsDragging] = useState(false);

  const processFiles = useCallback(
    async (files: FileList | File[]) => {
      const fileArray = Array.from(files);
      const newAttachments: Attachment[] = [];
      let nextTotalSize = attachments.reduce((total, attachment) => total + attachment.size, 0);

      for (const file of fileArray) {
        if (attachments.length + newAttachments.length >= MAX_ATTACHMENTS) {
          toast.error(`Maximum ${MAX_ATTACHMENTS} images per request`);
          break;
        }

        if (file.size > MAX_FILE_SIZE) {
          toast.error(`File ${file.name} exceeds 10MB limit`);
          continue;
        }

        if (nextTotalSize + file.size > MAX_TOTAL_SIZE) {
          toast.error("Images exceed 15MB total limit");
          continue;
        }

        if (!SUPPORTED_IMAGE_TYPES.includes(file.type)) {
          toast.error(`File type ${file.type} not supported`);
          continue;
        }

        try {
          const dataUrl = await readFileAsDataURL(file);
          newAttachments.push({
            id: crypto.randomUUID(),
            type: "image",
            name: file.name,
            size: file.size,
            dataUrl,
            mimeType: file.type,
          });
          nextTotalSize += file.size;
        } catch (error) {
          toast.error(`Failed to read ${file.name}`);
        }
      }

      if (newAttachments.length > 0) {
        onAttachmentsChange([...attachments, ...newAttachments]);
        toast.success(`Added ${newAttachments.length} image(s)`);
      }
    },
    [attachments, onAttachmentsChange]
  );

  const readFileAsDataURL = (file: File): Promise<string> => {
    return new Promise((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => resolve(reader.result as string);
      reader.onerror = reject;
      reader.readAsDataURL(file);
    });
  };

  const handleFileSelect = useCallback(
    (event: React.ChangeEvent<HTMLInputElement>) => {
      const files = event.target.files;
      if (files && files.length > 0) {
        processFiles(files);
      }
      if (fileInputRef.current) {
        fileInputRef.current.value = "";
      }
    },
    [processFiles]
  );

  const handlePaste = useCallback(
    (event: ClipboardEvent) => {
      if (disabled) return;

      const items = event.clipboardData?.items;
      if (!items) return;

      const files: File[] = [];
      for (let i = 0; i < items.length; i++) {
        const item = items[i];
        if (item.kind === "file") {
          const file = item.getAsFile();
          if (file) files.push(file);
        }
      }

      if (files.length > 0) {
        event.preventDefault();
        processFiles(files);
      }
    },
    [disabled, processFiles]
  );

  const handleDragOver = useCallback((event: React.DragEvent) => {
    event.preventDefault();
    setIsDragging(true);
  }, []);

  const handleDragLeave = useCallback(() => {
    setIsDragging(false);
  }, []);

  const handleDrop = useCallback(
    (event: React.DragEvent) => {
      event.preventDefault();
      setIsDragging(false);

      if (disabled) return;

      const files = event.dataTransfer.files;
      if (files.length > 0) {
        processFiles(files);
      }
    },
    [disabled, processFiles]
  );

  const removeAttachment = useCallback(
    (id: string) => {
      onAttachmentsChange(attachments.filter((a) => a.id !== id));
    },
    [attachments, onAttachmentsChange]
  );

  // Register paste listener
  useEffect(() => {
    document.addEventListener("paste", handlePaste);
    return () => document.removeEventListener("paste", handlePaste);
  }, [handlePaste]);

  return (
    <div className="space-y-2">
      {/* Attachments preview */}
      {attachments.length > 0 && (
        <div className="flex flex-wrap gap-2">
          {attachments.map((attachment) => (
            <div
              key={attachment.id}
              className="relative group rounded-lg border bg-muted overflow-hidden"
            >
              <img
                src={attachment.dataUrl}
                alt={attachment.name}
                className="h-20 w-20 object-cover"
              />
              <Button
                variant="destructive"
                size="icon"
                aria-label={`Remove ${attachment.name}`}
                className="absolute top-1 right-1 h-5 w-5 opacity-100 transition-opacity md:opacity-0 md:group-hover:opacity-100 md:group-focus-within:opacity-100"
                onClick={() => removeAttachment(attachment.id)}
              >
                <X className="h-3 w-3" />
              </Button>
              <div className="absolute bottom-0 left-0 right-0 bg-black/60 text-white text-[10px] px-1 py-0.5 truncate">
                {attachment.name}
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Upload controls */}
      <div
        className={`flex items-center gap-2 rounded-lg border-2 border-dashed p-2 transition-colors ${
          isDragging
            ? "border-primary bg-primary/5"
            : "border-muted-foreground/25"
        }`}
        onDragOver={handleDragOver}
        onDragLeave={handleDragLeave}
        onDrop={handleDrop}
      >
        <input
          ref={fileInputRef}
          type="file"
          accept={SUPPORTED_IMAGE_TYPES.join(",")}
          multiple
          className="hidden"
          onChange={handleFileSelect}
          disabled={disabled}
        />

        <Button
          variant="ghost"
          size="sm"
          onClick={() => fileInputRef.current?.click()}
          disabled={disabled}
          className="text-xs gap-1"
        >
          <Image className="h-4 w-4" />
          Add Image
        </Button>

        <span className="text-xs text-muted-foreground">
          or paste (Ctrl+V) / drag & drop
        </span>
      </div>
    </div>
  );
}
