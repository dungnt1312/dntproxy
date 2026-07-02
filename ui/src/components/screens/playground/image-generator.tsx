import { useEffect, useMemo, useRef, useState } from "react";
import { ImageIcon, Loader2, Sparkles, Download, Copy, Check, Upload, Wand2, X, AlertCircle } from "lucide-react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
import { ScrollArea } from "@/components/ui/scroll-area";
import { goApi } from "@/lib/go-api";

type ImageModel = {
  id: string;
  displayName: string;
  provider: string;
  capabilities: string[];
};

type ImageResult = {
  url?: string;
  b64_json?: string;
  revised_prompt?: string;
};

type ImageGenResponse = {
  created: number;
  data: ImageResult[];
};

const SIZES = ["1024x1024", "1792x1024", "1024x1792"];
const QUALITIES = ["standard", "hd"];
const FORMATS: Array<{ value: "url" | "b64_json"; label: string }> = [
  { value: "url", label: "URL" },
  { value: "b64_json", label: "Base64" },
];

interface ImageGeneratorProps {
  // No longer depends on parent models - loads its own
}

export function ImageGenerator({}: ImageGeneratorProps) {
  const [mode, setMode] = useState<"generate" | "edit">("generate");
  const [prompt, setPrompt] = useState("");
  const [model, setModel] = useState("");
  const [n, setN] = useState(1);
  const [size, setSize] = useState("1024x1024");
  const [quality, setQuality] = useState("standard");
  const [responseFormat, setResponseFormat] = useState<"url" | "b64_json">("url");
  const [generating, setGenerating] = useState(false);
  const [results, setResults] = useState<ImageResult[]>([]);
  const [copiedIdx, setCopiedIdx] = useState<number | null>(null);

  // Models loaded from /v1/models?type=image
  const [imageModels, setImageModels] = useState<ImageModel[]>([]);
  const [loadingModels, setLoadingModels] = useState(true);

  // Edit mode state
  const [editImages, setEditImages] = useState<{ dataUrl: string; name: string }[]>([]);
  const [editMask, setEditMask] = useState<{ dataUrl: string; name: string } | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const maskInputRef = useRef<HTMLInputElement>(null);

  // Load image models independently
  useEffect(() => {
    setLoadingModels(true);
    goApi.getImageModels()
      .then((models: ImageModel[]) => {
        setImageModels(models);
        if (models.length > 0 && !model) {
          setModel(models[0].id);
        }
      })
      .catch(() => toast.error("Failed to load image models"))
      .finally(() => setLoadingModels(false));
  }, []);

  const selectedModelDetails = useMemo(() => {
    return imageModels.find((m) => m.id === model);
  }, [imageModels, model]);

  const handleGenerate = async () => {
    if (!model || !prompt.trim()) {
      toast.error("Please select a model and enter a prompt");
      return;
    }

    if (mode === "edit" && editImages.length === 0) {
      toast.error("Please upload at least one image to edit");
      return;
    }

    setGenerating(true);
    setResults([]);

    try {
      if (mode === "generate") {
        const resp: ImageGenResponse = await goApi.generateImage({
          model,
          prompt,
          n,
          size,
          quality,
          response_format: responseFormat,
        });
        if (resp.data?.length > 0) {
          setResults(resp.data);
          toast.success(`Generated ${resp.data.length} image(s)`);
        } else {
          toast.error("No images returned");
        }
      } else {
        // Edit mode - use JSON body with image URLs
        const resp: ImageGenResponse = await goApi.editImage({
          model,
          prompt,
          images: editImages.map((img) => ({ image_url: img.dataUrl })),
          mask: editMask?.dataUrl,
          n,
          size,
          response_format: responseFormat,
        });
        if (resp.data?.length > 0) {
          setResults(resp.data);
          toast.success(`Edited ${resp.data.length} image(s)`);
        } else {
          toast.error("No images returned");
        }
      }
    } catch (err: any) {
      toast.error(err?.message || "Image generation failed");
    } finally {
      setGenerating(false);
    }
  };

  const getImageSrc = (result: ImageResult) => {
    if (result.url) return result.url;
    if (result.b64_json) return `data:image/png;base64,${result.b64_json}`;
    return "";
  };

  const handleDownload = async (result: ImageResult, index: number) => {
    const src = getImageSrc(result);
    if (!src) return;
    try {
      const resp = await fetch(src);
      const blob = await resp.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `generated-${index + 1}.png`;
      a.click();
      URL.revokeObjectURL(url);
    } catch {
      window.open(src, "_blank");
    }
  };

  const handleCopyBase64 = async (result: ImageResult, index: number) => {
    if (!result.b64_json) return;
    try {
      await navigator.clipboard.writeText(result.b64_json);
      setCopiedIdx(index);
      toast.success("Base64 copied to clipboard");
      setTimeout(() => setCopiedIdx(null), 2000);
    } catch {
      toast.error("Failed to copy");
    }
  };

  const handleFilePick = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files;
    if (!files) return;
    for (const file of Array.from(files)) {
      if (file.size > 10 * 1024 * 1024) {
        toast.error(`File ${file.name} exceeds 10MB limit`);
        continue;
      }
      const reader = new FileReader();
      reader.onload = () => {
        setEditImages((prev) => [...prev, { dataUrl: reader.result as string, name: file.name }]);
      };
      reader.readAsDataURL(file);
    }
  };

  const handleMaskPick = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    if (file.size > 10 * 1024 * 1024) {
      toast.error("Mask file exceeds 10MB limit");
      return;
    }
    const reader = new FileReader();
    reader.onload = () => {
      setEditMask({ dataUrl: reader.result as string, name: file.name });
    };
    reader.readAsDataURL(file);
  };

  const handlePaste = (e: React.ClipboardEvent) => {
    if (mode !== "edit") return;
    const items = e.clipboardData?.items;
    if (!items) return;
    for (const item of Array.from(items)) {
      if (item.type.startsWith("image/")) {
        const file = item.getAsFile();
        if (!file) continue;
        const reader = new FileReader();
        reader.onload = () => {
          setEditImages((prev) => [...prev, { dataUrl: reader.result as string, name: "pasted-image.png" }]);
        };
        reader.readAsDataURL(file);
      }
    }
  };

  const removeImage = (idx: number) => {
    setEditImages((prev) => prev.filter((_, i) => i !== idx));
  };

  const handleClear = () => {
    setPrompt("");
    setEditImages([]);
    setEditMask(null);
    setResults([]);
  };

  return (
    <div className="flex h-full flex-col" onPaste={handlePaste}>
      {/* Header */}
      <div className="flex flex-col gap-3 border-b bg-background/95 px-4 py-3 backdrop-blur-sm md:px-6">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <ImageIcon className="h-5 w-5 text-purple-600" />
            <div>
              <h1 className="text-lg font-semibold">Image Playground</h1>
              <p className="text-xs text-muted-foreground">Generate or edit images using AI models</p>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <Button variant="outline" size="sm" onClick={handleClear} disabled={generating}>
              Clear
            </Button>
            <Button onClick={handleGenerate} disabled={generating || !model || !prompt.trim() || (mode === "edit" && editImages.length === 0)} className="gap-1.5">
              {generating ? <Loader2 className="h-4 w-4 animate-spin" /> : mode === "edit" ? <Wand2 className="h-4 w-4" /> : <Sparkles className="h-4 w-4" />}
              {generating ? "Processing..." : mode === "edit" ? "Edit" : "Generate"}
            </Button>
          </div>
        </div>

        {/* Mode tabs */}
        <Tabs value={mode} onValueChange={(v) => { setMode(v as "generate" | "edit"); setResults([]); }}>
          <TabsList className="h-7 w-fit">
            <TabsTrigger value="generate" className="text-xs gap-1 h-6"><Sparkles className="h-3 w-3" />Generate</TabsTrigger>
            <TabsTrigger value="edit" className="text-xs gap-1 h-6"><Wand2 className="h-3 w-3" />Edit</TabsTrigger>
          </TabsList>
        </Tabs>

        {/* Controls */}
        <div className="flex flex-wrap gap-3">
          <div className="min-w-[200px]">
            <Label className="text-xs text-muted-foreground mb-1 block">Model</Label>
            <Select value={model} onValueChange={setModel} disabled={loadingModels || imageModels.length === 0}>
              <SelectTrigger className="h-8 text-xs">
                <SelectValue placeholder={loadingModels ? "Loading..." : "Select model"} />
              </SelectTrigger>
              <SelectContent>
                {imageModels.map((m) => (
                  <SelectItem key={m.id} value={m.id} className="text-xs">{m.displayName || m.id}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="min-w-[120px]">
            <Label className="text-xs text-muted-foreground mb-1 block">Size</Label>
            <Select value={size} onValueChange={setSize}>
              <SelectTrigger className="h-8 text-xs"><SelectValue /></SelectTrigger>
              <SelectContent>
                {SIZES.map((s) => (<SelectItem key={s} value={s} className="text-xs">{s}</SelectItem>))}
              </SelectContent>
            </Select>
          </div>

          <div className="min-w-[110px]">
            <Label className="text-xs text-muted-foreground mb-1 block">Quality</Label>
            <Select value={quality} onValueChange={setQuality}>
              <SelectTrigger className="h-8 text-xs"><SelectValue /></SelectTrigger>
              <SelectContent>
                {QUALITIES.map((q) => (<SelectItem key={q} value={q} className="text-xs capitalize">{q}</SelectItem>))}
              </SelectContent>
            </Select>
          </div>

          <div className="min-w-[80px]">
            <Label className="text-xs text-muted-foreground mb-1 block">Count</Label>
            <Select value={String(n)} onValueChange={(v) => setN(Number(v))}>
              <SelectTrigger className="h-8 text-xs"><SelectValue /></SelectTrigger>
              <SelectContent>
                {[1, 2, 3, 4].map((x) => (<SelectItem key={x} value={String(x)} className="text-xs">{x}</SelectItem>))}
              </SelectContent>
            </Select>
          </div>

          <div className="min-w-[100px]">
            <Label className="text-xs text-muted-foreground mb-1 block">Format</Label>
            <Select value={responseFormat} onValueChange={(v) => setResponseFormat(v as "url" | "b64_json")}>
              <SelectTrigger className="h-8 text-xs"><SelectValue /></SelectTrigger>
              <SelectContent>
                {FORMATS.map((f) => (<SelectItem key={f.value} value={f.value} className="text-xs">{f.label}</SelectItem>))}
              </SelectContent>
            </Select>
          </div>
        </div>
      </div>

      {/* Prompt + Edit upload area */}
      <div className="border-b bg-background/95 px-4 py-3 md:px-6 space-y-3">
        {/* Edit: image upload */}
        {mode === "edit" && (
          <div className="space-y-2">
            <Label className="text-xs text-muted-foreground">Upload Images to Edit</Label>
            <div className="flex flex-wrap gap-2">
              {editImages.map((img, idx) => (
                <div key={idx} className="relative group h-20 w-20 rounded-md border overflow-hidden bg-muted shrink-0">
                  <img src={img.dataUrl} alt={img.name} className="h-full w-full object-cover" />
                  <button onClick={() => removeImage(idx)} className="absolute top-0.5 right-0.5 bg-black/60 rounded-full p-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
                    <X className="h-3 w-3 text-white" />
                  </button>
                </div>
              ))}
              <button
                onClick={() => fileInputRef.current?.click()}
                className="h-20 w-20 rounded-md border-2 border-dashed border-muted-foreground/30 flex flex-col items-center justify-center gap-0.5 text-muted-foreground hover:border-purple-400 hover:text-purple-500 transition-colors shrink-0"
              >
                <Upload className="h-4 w-4" />
                <span className="text-[10px]">Upload</span>
              </button>
            </div>
            <input ref={fileInputRef} type="file" accept="image/*" multiple className="hidden" onChange={handleFilePick} />
            <p className="text-[10px] text-muted-foreground">Paste, drag & drop, or click to upload images (max 10MB each)</p>

            {/* Mask upload */}
            <div className="flex items-center gap-2 pt-1">
              <Label className="text-xs text-muted-foreground shrink-0">Mask (optional):</Label>
              {editMask ? (
                <div className="flex items-center gap-1.5">
                  <div className="relative h-8 w-8 rounded border overflow-hidden bg-muted">
                    <img src={editMask.dataUrl} alt="mask" className="h-full w-full object-cover" />
                  </div>
                  <span className="text-xs text-muted-foreground truncate max-w-[100px]">{editMask.name}</span>
                  <button onClick={() => setEditMask(null)} className="text-muted-foreground hover:text-red-500"><X className="h-3 w-3" /></button>
                </div>
              ) : (
                <button onClick={() => maskInputRef.current?.click()} className="text-xs text-purple-500 hover:text-purple-600">
                  Upload mask
                </button>
              )}
              <input ref={maskInputRef} type="file" accept="image/*" className="hidden" onChange={handleMaskPick} />
            </div>
          </div>
        )}

        {/* Prompt */}
        <div>
          <Label className="text-xs text-muted-foreground mb-1.5 block">
            {mode === "edit" ? "Edit prompt" : "Prompt"}
          </Label>
          <Textarea
            value={prompt}
            onChange={(e) => setPrompt(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); handleGenerate(); }
            }}
            placeholder={mode === "edit"
              ? "Describe how to edit the image... (e.g. Add sunglasses to the cat)"
              : "Describe the image you want to generate..."}
            className="min-h-[60px] resize-none"
            rows={2}
            disabled={generating}
          />
          {selectedModelDetails && (
            <div className="mt-1.5 flex items-center gap-2">
              <Badge variant="secondary" className="text-[10px]">{selectedModelDetails.displayName || selectedModelDetails.id}</Badge>
              {mode === "edit" && editImages.length > 0 && (
                <Badge variant="outline" className="text-[10px]">{editImages.length} image{editImages.length > 1 ? "s" : ""}</Badge>
              )}
            </div>
          )}
        </div>
      </div>

      {/* Results */}
      <ScrollArea className="flex-1">
        <div className="p-4 md:p-6">
          {generating && results.length === 0 ? (
            <div className="flex min-h-[40vh] flex-col items-center justify-center gap-4 text-center">
              <Loader2 className="h-10 w-10 animate-spin text-purple-500" />
              <p className="text-sm text-muted-foreground">{mode === "edit" ? "Editing" : "Generating"} image{n > 1 ? "s" : ""}...</p>
            </div>
          ) : results.length === 0 ? (
            <div className="flex min-h-[40vh] flex-col items-center justify-center gap-4 text-center">
              <div className="flex h-16 w-16 items-center justify-center rounded-full bg-purple-100 dark:bg-purple-950">
                <ImageIcon className="h-8 w-8 text-purple-600" />
              </div>
              <div>
                <h2 className="text-xl font-semibold">{mode === "edit" ? "Edit an image" : "Generate an image"}</h2>
                <p className="mt-2 max-w-md text-sm text-muted-foreground">
                  {imageModels.length === 0
                    ? "No image models found. Add a connection with image model support."
                    : mode === "edit"
                      ? "Upload an image, enter an edit prompt, and click Edit."
                      : "Select a model, enter a prompt, and click Generate."}
                </p>
              </div>
            </div>
          ) : (
            <div className="space-y-6">
              <div className={`grid gap-4 ${results.length === 1 ? "max-w-xl mx-auto" : results.length === 2 ? "grid-cols-1 md:grid-cols-2" : "grid-cols-1 md:grid-cols-2 lg:grid-cols-3"}`}>
                {results.map((result, idx) => {
                  const src = getImageSrc(result);
                  const aspect = size === "1024x1792" ? "9/16" : size === "1792x1024" ? "16/9" : "1/1";
                  return (
                    <div key={idx} className="group relative overflow-hidden rounded-lg border bg-card">
                      {src ? (
                        <img src={src} alt={result.revised_prompt || `Image ${idx + 1}`} className="w-full object-cover" style={{ aspectRatio: aspect }} />
                      ) : (
                        <div className="flex aspect-square items-center justify-center bg-muted text-muted-foreground text-sm">No image data</div>
                      )}
                      <div className="absolute inset-0 flex items-end gap-1.5 bg-gradient-to-t from-black/60 to-transparent p-2 opacity-0 transition-opacity group-hover:opacity-100">
                        <TooltipProvider>
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <Button size="icon" variant="secondary" className="h-7 w-7" onClick={() => handleDownload(result, idx)}>
                                <Download className="h-3.5 w-3.5" />
                              </Button>
                            </TooltipTrigger>
                            <TooltipContent><p>Download</p></TooltipContent>
                          </Tooltip>
                        </TooltipProvider>
                        {result.b64_json && (
                          <TooltipProvider>
                            <Tooltip>
                              <TooltipTrigger asChild>
                                <Button size="icon" variant="secondary" className="h-7 w-7" onClick={() => handleCopyBase64(result, idx)}>
                                  {copiedIdx === idx ? <Check className="h-3.5 w-3.5 text-green-500" /> : <Copy className="h-3.5 w-3.5" />}
                                </Button>
                              </TooltipTrigger>
                              <TooltipContent><p>Copy Base64</p></TooltipContent>
                            </Tooltip>
                          </TooltipProvider>
                        )}
                      </div>
                    </div>
                  );
                })}
              </div>

              {results.some((r) => r.revised_prompt) && (
                <div className="space-y-2">
                  <h3 className="text-sm font-medium">Revised Prompts</h3>
                  {results.map((result, idx) =>
                    result.revised_prompt ? (
                      <div key={idx} className="rounded-lg border bg-muted/50 p-3">
                        <p className="text-xs font-medium text-muted-foreground mb-1">Image {idx + 1}</p>
                        <p className="text-sm">{result.revised_prompt}</p>
                      </div>
                    ) : null,
                  )}
                </div>
              )}
            </div>
          )}
        </div>
      </ScrollArea>
    </div>
  );
}
