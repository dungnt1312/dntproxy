import { stripModelForConnection } from './BulkModelsModal';
import type { Connection } from '@/types/connections';

/**
 * Pure model-list operations shared by the migration wizard's dry-run
 * preview and its real apply loop.
 */

export interface ModelOps {
  add: string[];
  remove: string[];
  renames: Array<{ from: string; to: string }>;
}

function normalizeStored(models: string[], conn: Connection): string[] {
  return models.map((m) => stripModelForConnection(m, conn));
}

/** Apply add/remove/rename ops to one connection's stored (unprefixed) models. */
export function applyModelOps(
  storedModels: string[],
  ops: ModelOps,
  conn: Connection,
): { result: string[]; conflicts: string[] } {
  const conflicts: string[] = [];
  let models = normalizeStored(storedModels, conn);

  const removeSet = new Set(ops.remove.map((m) => stripModelForConnection(m, conn)));
  const renameFrom = new Map(
    ops.renames.map((r) => [stripModelForConnection(r.from, conn), stripModelForConnection(r.to, conn)]),
  );
  for (const r of ops.renames) {
    if (!models.includes(stripModelForConnection(r.from, conn)) && !removeSet.has(stripModelForConnection(r.from, conn))) {
      conflicts.push(`"${r.from}" is not in the current list`);
    }
  }

  models = models.filter((m) => !removeSet.has(m));
  models = models.map((m) => renameFrom.get(m) ?? m);
  for (const m of ops.add.map((a) => stripModelForConnection(a, conn))) {
    if (m && !models.includes(m)) models.push(m);
  }
  if (ops.add.some((a) => !a.trim())) conflicts.push('An added-model line is empty');

  return { result: [...new Set(models)], conflicts };
}
