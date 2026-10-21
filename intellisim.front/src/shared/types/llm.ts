export type ConfigParamType =
  | 'double' | 'int' | 'uint' | 'bool' | 'string' | 'enum' | 'string_list'

export interface ConfigValue {
  double?: number
  int?: number
  uint?: number
  bool?: boolean
  string?: string
  list?: string[]
}

export type GenerationParams = Record<string, ConfigValue>

export interface ConfigParam {
  key: string
  type: ConfigParamType
  required: boolean
  label?: string
  description?: string
  min?: number
  max?: number
  enum_values?: string[]
  default?: ConfigValue
}

export interface ModelInfo {
  uid: string
  name: string
  capabilities: string[]
  efforts: string[]
  context_window: number
  max_output_tokens: number
  config_params: ConfigParam[]
  reasoning: boolean
  /** Max images this model accepts in one vision request (0/absent → treat as 1). */
  max_images?: number
}

// Execution LLM target for models/simulations (NOT chats).
export interface ExecutionLlmTarget {
  provider_uid: string
  model_uid: string
  effort?: string
  generation?: GenerationParams
  timeout_ms?: number
}

export interface ProviderInfo {
  uid: string
  name: string
  description: string
  version: string
  connected_at: string
  models: ModelInfo[]
}

export interface LLMProviderListResponse {
  list: ProviderInfo[]
}
