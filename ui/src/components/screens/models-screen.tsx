import { useCallback, useEffect, useMemo, useState } from "react";
import { Layers, Link2, GitBranch } from "lucide-react";
import { toast } from "sonner";
import { motion } from "framer-motion";
import { goApi } from "@/lib/go-api";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import LogsViewerModal, { LogFilter } from "../connections/LogsViewerModal";

import ModelsTab from "./routing/models-tab";
import AliasesTab from "./routing/aliases-tab";
import CombosTab from "./routing/combos-tab";
import { UiModel, AliasMap, ComboData, ConnectionOption, RoutingLoadErrors } from "./routing/types";
import { RoutingErrorState } from "./routing/routing-error-state";

const containerVariants = {
  hidden: { opacity: 0 },
  visible: { opacity: 1, transition: { staggerChildren: 0.06 } },
};

const itemVariants = {
  hidden: { opacity: 0, y: 16 },
  visible: { opacity: 1, y: 0, transition: { duration: 0.35 } },
};

export default function ModelsScreen() {
  const [models, setModels] = useState<UiModel[]>([]);
  const [aliases, setAliases] = useState<AliasMap>({});
  const [combos, setCombos] = useState<ComboData[]>([]);
  const [connections, setConnections] = useState<ConnectionOption[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadErrors, setLoadErrors] = useState<RoutingLoadErrors>({});

  const [logModal, setLogModal] = useState<{ isOpen: boolean; title: string; filter: LogFilter }>({ 
    isOpen: false, 
    title: "", 
    filter: {} 
  });

  const fetchAll = useCallback(async () => {
    setLoading(true);
    const [modelsResult, aliasesResult, combosResult, connectionsResult] = await Promise.allSettled([
      goApi.getModels(),
      goApi.getAliases(),
      goApi.getCombos(),
      goApi.getConnections(),
    ]);

    const nextErrors: RoutingLoadErrors = {};

    if (modelsResult.status === "fulfilled" && Array.isArray(modelsResult.value)) {
      setModels(modelsResult.value);
    } else {
      setModels([]);
      nextErrors.models = "Models unavailable.";
    }

    if (aliasesResult.status === "fulfilled") {
      setAliases(aliasesResult.value || {});
    } else {
      setAliases({});
      nextErrors.aliases = "Aliases unavailable.";
    }

    if (combosResult.status === "fulfilled" && Array.isArray(combosResult.value)) {
      setCombos(combosResult.value);
    } else {
      setCombos([]);
      nextErrors.combos = "Combos unavailable.";
    }

    if (connectionsResult.status === "fulfilled" && Array.isArray(connectionsResult.value)) {
      setConnections(connectionsResult.value);
    } else {
      setConnections([]);
      nextErrors.connections = "Connections unavailable.";
    }

    setLoadErrors(nextErrors);
    setLoading(false);

    if (Object.keys(nextErrors).length === 4) {
      toast.error("Failed to load routing data");
    }
  }, []);

  useEffect(() => {
    fetchAll();
  }, [fetchAll]);

  const handleOpenLogModalForModel = (modelId: string) => {
    setLogModal({ isOpen: true, title: `Logs: ${modelId}`, filter: { model: modelId } });
  };

  const handleOpenLogModalForAlias = (aliasName: string) => {
    setLogModal({ isOpen: true, title: `Logs: ${aliasName}`, filter: { aliasName } });
  };

  const handleOpenLogModalForCombo = (comboName: string, allowedProviders: string[]) => {
    setLogModal({ isOpen: true, title: `Logs: ${comboName}`, filter: { comboName, allowedProviders } });
  };

  // Stats
  const registryCount = useMemo(() => 
    models.filter(m => m.provider !== 'alias' && m.provider !== 'combo').length
  , [models]);
  const aliasCount = Object.keys(aliases).length;
  const comboCount = combos.length;

  return (
    <motion.div
      className="space-y-6"
      variants={containerVariants}
      initial="hidden"
      animate="visible"
    >
      {/* Header */}
      <motion.div variants={itemVariants} className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-3">
          <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-emerald-500/10">
            <Layers className="h-5 w-5 text-emerald-500" />
          </div>
          <div>
            <h1 className="text-2xl font-bold tracking-tight">
              Routing & Models
            </h1>
            <p className="text-sm text-muted-foreground">
              Manage your model registry, configure aliases, and define fallback combos.
            </p>
          </div>
        </div>
      </motion.div>

      {/* Tabs */}
      <motion.div variants={itemVariants}>
        <RoutingErrorState errors={Object.values(loadErrors)} onRetry={fetchAll} />
        <Tabs defaultValue="registry" className="w-full space-y-5">
          <TabsList className="bg-muted/50 p-1">
            <TabsTrigger value="registry" className="rounded-sm px-4 gap-1.5">
              <Layers className="h-3.5 w-3.5" />
              Registry
              {registryCount > 0 && (
                <span className="ml-1 rounded-full bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 px-2 py-0.5 text-[10px] font-medium">
                  {registryCount}
                </span>
              )}
            </TabsTrigger>
            <TabsTrigger value="aliases" className="rounded-sm px-4 gap-1.5">
              <Link2 className="h-3.5 w-3.5" />
              Aliases
              {aliasCount > 0 && (
                <span className="ml-1 rounded-full bg-blue-500/15 text-blue-600 dark:text-blue-400 px-2 py-0.5 text-[10px] font-medium">
                  {aliasCount}
                </span>
              )}
            </TabsTrigger>
            <TabsTrigger value="combos" className="rounded-sm px-4 gap-1.5">
              <GitBranch className="h-3.5 w-3.5" />
              Combos
              {comboCount > 0 && (
                <span className="ml-1 rounded-full bg-violet-500/15 text-violet-600 dark:text-violet-400 px-2 py-0.5 text-[10px] font-medium">
                  {comboCount}
                </span>
              )}
            </TabsTrigger>
          </TabsList>

          <TabsContent value="registry" className="space-y-4 outline-none">
            <ModelsTab
              models={models}
              loading={loading}
              hasLoadError={!!loadErrors.models}
              onOpenLogModal={handleOpenLogModalForModel}
            />
          </TabsContent>

          <TabsContent value="aliases" className="space-y-4 outline-none">
            <AliasesTab
              aliases={aliases}
              models={models}
              connections={connections}
              loading={loading}
              hasLoadError={!!loadErrors.aliases}
              onRefresh={fetchAll}
              onOpenLogModal={handleOpenLogModalForAlias}
            />
          </TabsContent>

          <TabsContent value="combos" className="outline-none">
            <CombosTab 
              combos={combos} 
              connections={connections} 
              models={models} 
              loading={loading}
              hasLoadError={!!loadErrors.combos}
              onRefresh={fetchAll}
              onOpenLogModal={handleOpenLogModalForCombo} 
            />
          </TabsContent>
        </Tabs>
      </motion.div>
      
      <LogsViewerModal
        isOpen={logModal.isOpen}
        onClose={() => setLogModal(prev => ({ ...prev, isOpen: false }))}
        title={logModal.title}
        filter={logModal.filter}
      />
    </motion.div>
  );
}
