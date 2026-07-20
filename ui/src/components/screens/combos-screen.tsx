import { useCallback, useEffect, useState } from "react";
import { GitBranch } from "lucide-react";
import { motion } from "framer-motion";
import { toast } from "sonner";
import { goApi } from "@/lib/go-api";

import CombosTab from "./routing/combos-tab";
import LogsViewerModal, { type LogFilter } from "../connections/LogsViewerModal";
import { RoutingErrorState } from "./routing/routing-error-state";
import type { ComboData, ConnectionOption, UiModel, RoutingLoadErrors } from "./routing/types";

const containerVariants = {
  hidden: { opacity: 0 },
  visible: { opacity: 1, transition: { staggerChildren: 0.06 } },
};

const itemVariants = {
  hidden: { opacity: 0, y: 16 },
  visible: { opacity: 1, y: 0, transition: { duration: 0.35 } },
};

export default function CombosScreen() {
  const [combos, setCombos] = useState<ComboData[]>([]);
  const [connections, setConnections] = useState<ConnectionOption[]>([]);
  const [models, setModels] = useState<UiModel[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadErrors, setLoadErrors] = useState<RoutingLoadErrors>({});

  const [logModal, setLogModal] = useState<{ 
    isOpen: boolean; 
    title: string; 
    filter: LogFilter 
  }>({ 
    isOpen: false, 
    title: "", 
    filter: {} 
  });

  const fetchAll = useCallback(async () => {
    setLoading(true);
    const [combosResult, connectionsResult, modelsResult] = await Promise.allSettled([
      goApi.getCombos(),
      goApi.getConnections(),
      goApi.getModels(),
    ]);

    const nextErrors: RoutingLoadErrors = {};

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

    if (modelsResult.status === "fulfilled" && Array.isArray(modelsResult.value)) {
      setModels(modelsResult.value);
    } else {
      setModels([]);
    }

    setLoadErrors(nextErrors);
    setLoading(false);

    if (nextErrors.combos) {
      toast.error("Failed to load combos");
    }
  }, []);

  useEffect(() => {
    fetchAll();
  }, [fetchAll]);

  const handleOpenLogModalForCombo = (comboName: string, allowedProviders: string[]) => {
    setLogModal({ 
      isOpen: true, 
      title: `Logs: ${comboName}`, 
      filter: { comboName, allowedProviders } 
    });
  };

  const onRefresh = fetchAll;

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
          <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-violet-500/10">
            <GitBranch className="h-5 w-5 text-violet-500" />
          </div>
          <div>
            <h1 className="text-2xl font-bold tracking-tight">
              Combos
            </h1>
            <p className="text-sm text-muted-foreground">
              Create intelligent fallback chains and round-robin strategies. 
              Supports pinned accounts (@connectionId).
            </p>
          </div>
        </div>
      </motion.div>

      <motion.div variants={itemVariants}>
        <RoutingErrorState errors={Object.values(loadErrors)} onRetry={fetchAll} />
        
        <CombosTab 
          combos={combos}
          connections={connections}
          models={models}
          loading={loading}
          hasLoadError={!!loadErrors.combos}
          onRefresh={onRefresh}
          onOpenLogModal={handleOpenLogModalForCombo}
        />
      </motion.div>

      <LogsViewerModal
        isOpen={logModal.isOpen}
        onClose={() => setLogModal((prev) => ({ ...prev, isOpen: false }))}
        title={logModal.title}
        filter={logModal.filter}
      />
    </motion.div>
  );
}
