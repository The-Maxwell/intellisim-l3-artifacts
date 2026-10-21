import type { ExecutionLlmTarget } from './llm'
import type { TokenUsageSummary } from './chat'

export type AnalysisStatus = 'pending' | 'running' | 'completed' | 'failed'
export type AnalysisLanguage = 'en' | 'uk'
export type AnalysisFocusArea =
  | 'population' | 'properties' | 'spatial' | 'interactions' | 'events' | 'rules'

export interface AnalysisSelection {
  step_from?: number
  step_to?: number
  sampling?: number
  agent_uids?: string[]
  property_uids?: string[]
  focus_areas?: AnalysisFocusArea[]
}

export interface AnalyzeRequest {
  name?: string
  context_message: string
  language: AnalysisLanguage
  llm_target: ExecutionLlmTarget
  selection: AnalysisSelection
}

export interface AnalysisProgress { stage: string; percent: number }
export interface AnalysisStepsCovered { from: number; to: number; sampling: number; count: number }
export interface AnalysisFile { uid: string; name: string; mime_type: string; size: number; created_at: string }
export interface AnalysisSection { key: string; title: string; narrative: string; key_findings?: string[]; causal_links?: string[]; emergent_phenomena?: string[] }
export interface AnalysisReport { sections?: AnalysisSection[]; executive_summary: string; conclusions?: string[]; recommendations?: string[] }
export interface AnalysisWarning { stage: string; code: string; message: string }

export interface Analysis {
  id: string
  simulation_id: string
  name: string
  context_message: string
  language: AnalysisLanguage
  llm_target: ExecutionLlmTarget
  selection: AnalysisSelection
  status: AnalysisStatus
  status_text: string
  progress: AnalysisProgress
  simulation_status_at_analysis: string
  steps_covered: AnalysisStepsCovered
  report?: AnalysisReport | null
  file?: AnalysisFile | null
  token_usage?: TokenUsageSummary | null
  requests: number
  warnings?: AnalysisWarning[]
  error?: string
  created_at: string
  updated_at?: string
  created_by: string
  started_at?: string | null
  finished_at?: string | null
}

export interface AnalysisSummary {
  id: string
  name: string
  status: AnalysisStatus
  language: AnalysisLanguage
  model_label: string
  simulation_status_at_analysis: string
  has_file: boolean
  created_at: string
}

export interface ListAnalysesResponse { items: AnalysisSummary[] }

export interface ResultsSummary {
  step_range: { min: number; max: number }
  agent_types: { uid: string; name: string; type: string }[]
  properties: { uid: string; name: string; type: string; entity_type: 'agent' | 'shape' }[]
  focus_areas: string[]
  has_results: boolean
}

export interface ResultsSeries {
  property_uid?: string
  population?: { step: number; count: Record<string, number> }[]
  series?: { step: number; value: unknown }[]
}

export interface AnalysisProgressPayload { analysis_id: string; stage: string; percent: number }
export interface AnalysisCompletedPayload { analysis_id: string; status: string; file_uid?: string; file_name?: string; file_size?: number }
export interface AnalysisFailedPayload { analysis_id: string; error: string }
