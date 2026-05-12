export interface UiModel {
  id: string;
  name: string;
  modelId?: string;
  displayName?: string;
  provider: string;
  connectionId?: string;
  connectionName?: string;
  isActive?: boolean;
  models?: string[];
  connections?: Array<{ id: string; name: string; provider: string }>;
}

export interface RoutingLoadErrors {
  models?: string;
  aliases?: string;
  combos?: string;
  connections?: string;
}

export type AliasMap = Record<string, string>;

export interface ComboData {
  id: string;
  name: string;
  models: string[];
  connectionIds?: string[];
  createdAt?: string;
  updatedAt?: string;
}

export interface ConnectionOption {
  id: string;
  name: string;
  provider: string;
}
