import React, { useState } from "react";
import { ChevronRight, Copy, Check } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { motion, AnimatePresence } from "framer-motion";
import { ScrollArea } from "@/components/ui/scroll-area";

export function PayloadViewer({ label, rawContent, defaultOpen = true }: { label: string; rawContent: any; defaultOpen?: boolean }) {
  const [isOpen, setIsOpen] = useState(defaultOpen);
  const [viewMode, setViewMode] = useState<"formatted" | "raw">("formatted");
  const [copied, setCopied] = useState(false);

  const rawString = typeof rawContent === "string" ? rawContent : JSON.stringify(rawContent);

  if (!rawString || rawString === "{}" || rawString === "[]" || rawString === '""') return null;

  const handleCopy = (e: React.MouseEvent) => {
    e.stopPropagation();
    navigator.clipboard.writeText(rawString);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const getFormattedContent = () => {
    if (viewMode === "raw") return rawString;

    let content = rawString;

    // 1. Handle proxy stream tags like [tool:name] or [text]
    const hasTags = /^\[(text|tool:[^\]]+)\]/im.test(content);
    if (hasTags) {
       const lines = content.split('\n');
       const blocks: {tag: string, text: string}[] = [];
       
       for (let i = 0; i < lines.length; i++) {
           const line = lines[i];
           const match = line.match(/^\[(text|tool:[^\]]+)\] ?(.*)$/);
           if (match) {
               const tag = match[1];
               const text = match[2];
               if (blocks.length === 0 || blocks[blocks.length - 1].tag !== tag) {
                   blocks.push({ tag, text });
               } else {
                   blocks[blocks.length - 1].text += text;
               }
           } else {
               if (blocks.length > 0) {
                   blocks[blocks.length - 1].text += '\n' + line;
               } else {
                   blocks.push({ tag: 'unknown', text: line });
               }
           }
       }
       
       let finalContent = '';
       for (const block of blocks) {
           if (finalContent.length > 0) finalContent += '\n\n';
           
           if (block.tag.startsWith('tool:')) {
               try {
                   const parsed = JSON.parse(block.text);
                   finalContent += `// ${block.tag}\n` + JSON.stringify(parsed, null, 2);
               } catch (e) {
                   finalContent += `// ${block.tag} (RAW/INCOMPLETE)\n` + block.text.replace(/\\n/g, '\n').replace(/\\"/g, '"').replace(/\\\\/g, '\\');
               }
           } else if (block.tag === 'text') {
               finalContent += block.text.replace(/\\n/g, '\n');
           } else {
               finalContent += block.text;
           }
       }
       return finalContent;
    }

    // 2. Try JSON Parse for pretty printing standard payloads
    try {
      const parsed = JSON.parse(content);
      if (typeof parsed === "object" && parsed !== null) {
        return JSON.stringify(parsed, null, 2);
      }
    } catch {
      // 3. Fallback: Unescape newlines to make raw/truncated text readable
      content = content.replace(/\\n/g, '\n').replace(/\\"/g, '"').replace(/\\\\/g, '\\');
    }

    return content;
  };

  const formatted = getFormattedContent();

  return (
    <div className="mb-3 rounded-lg border bg-card overflow-hidden shadow-sm">
      <div 
        className="flex w-full items-center justify-between bg-muted/40 px-3 py-2 text-sm font-medium cursor-pointer hover:bg-muted/70 transition-colors"
        onClick={() => setIsOpen(!isOpen)}
      >
        <span className="flex items-center gap-2">
          <motion.div animate={{ rotate: isOpen ? 90 : 0 }} transition={{ duration: 0.2 }}>
            <ChevronRight className="h-4 w-4 text-muted-foreground" />
          </motion.div>
          {label}
        </span>
        
        {isOpen && (
          <div className="flex items-center gap-2 shrink-0" onClick={e => e.stopPropagation()}>
            <div className="flex items-center bg-background border rounded-md p-0.5">
              <button 
                onClick={() => setViewMode("formatted")}
                className={cn("px-2 py-1 text-[11px] font-medium rounded-sm transition-colors", viewMode === "formatted" ? "bg-muted shadow-sm" : "text-muted-foreground hover:text-foreground")}
              >
                Formatted
              </button>
              <button 
                onClick={() => setViewMode("raw")}
                className={cn("px-2 py-1 text-[11px] font-medium rounded-sm transition-colors", viewMode === "raw" ? "bg-muted shadow-sm" : "text-muted-foreground hover:text-foreground")}
              >
                Raw
              </button>
            </div>
            <Button variant="ghost" size="icon" className="h-7 w-7" onClick={handleCopy} title="Copy raw payload">
              {copied ? <Check className="h-3.5 w-3.5 text-green-500" /> : <Copy className="h-3.5 w-3.5 text-muted-foreground" />}
            </Button>
          </div>
        )}
      </div>

      <AnimatePresence initial={false}>
        {isOpen && (
          <motion.div
            initial={{ height: 0 }}
            animate={{ height: "auto" }}
            exit={{ height: 0 }}
            transition={{ duration: 0.2, ease: "easeInOut" }}
            className="overflow-hidden border-t"
          >
            <ScrollArea className="max-h-[450px]">
               <pre className="p-4 text-[13px] font-mono whitespace-pre-wrap break-words bg-[#1e1e1e] text-[#d4d4d4] dark:bg-[#0d0d0d] dark:text-[#c9d1d9] m-0 selection:bg-[#264f78]">
                 {formatted}
               </pre>
            </ScrollArea>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}
