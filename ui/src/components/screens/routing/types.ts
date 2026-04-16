export interface UiModel {
  id: string;
  name: string;
  displayName?: string;
  provider: string;
  isActive?: boolean;
  models?: string[];
  connections?: Array<{ id: string; name: string; provider: string }>;
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
