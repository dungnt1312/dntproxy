export type FormFieldType = 'text' | 'password' | 'textarea' | 'select' | 'number' | 'url';

export interface FormFieldMeta {
  name: string;
  label: string;
  type: FormFieldType;
  required: boolean;
  placeholder?: string;
  defaultValue?: string;
  secret?: boolean;
  helpText?: string;
  options?: string[];
}

export interface ProviderUIMeta {
  category: string;
  description: string;
  docsUrl?: string;
  showBaseUrlField: boolean;
  baseUrlLabel?: string;
  baseUrlPlaceholder?: string;
  preferredAuthMethod?: string;
  authFlows: string[];
  formFields: FormFieldMeta[];
  supportsModelSelect: boolean;
  defaultTestModel?: string;
}

export interface ProviderConfigMeta {
  id: string;
  name: string;
  icon: string;
  authMethods: string[];
  defaultBaseUrl: string;
  recommendedModels: string[];
  format: string;
  supportsQuota: boolean;
  ui: ProviderUIMeta;
}

export interface CreateConnectionPayload {
  provider: string;
  name?: string;
  apiKey?: string;
  baseUrl?: string;
  routePrefix?: string;
  modelPrefix?: string;
  supportedModels?: string[];
}