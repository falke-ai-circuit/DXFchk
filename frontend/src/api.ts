const BASE = '';

export interface HealthResponse {
  status: string;
  version: string;
  running: boolean;
}

export interface SettingsResponse {
  template_folder: string;
  search_folder: string;
  output_folder: string;
  recursive: boolean;
  move_files: boolean;
  group_by_content: boolean;
}

export interface TemplateMapping {
  [key: string]: string;
}

export interface TemplatesResponse {
  count: number;
  mapping: TemplateMapping;
}

export interface ScanTemplatesRequest {
  template_folder: string;
  recursive: boolean;
}

export interface ScanTemplatesResponse {
  count: number;
  mapping: TemplateMapping;
}

export interface CompareRequest {
  search_folder: string;
  recursive: boolean;
  move_files: boolean;
  group_by_content: boolean;
}

export interface CompareResponse {
  ok: boolean;
  message: string;
  output: string;
}

export interface CompareStatus {
  running: boolean;
  total_files: number;
  processed_files: number;
  progress: number;
  log_count: number;
  recent_logs: string[];
  results_count: number;
}

export interface ComparisonResult {
  file_name: string;
  status: string;
  template?: string;
  content_hash?: string;
  mod_folder?: string;
}

export interface ResultsResponse {
  results: ComparisonResult[];
  count: number;
}

export async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, init);
  if (!res.ok) {
    const text = await res.text().catch(() => `HTTP ${res.status}`);
    throw new Error(text);
  }
  if (res.status === 204) return undefined as T;
  return res.json();
}

// API client
export const api = {
  health: () => apiFetch<HealthResponse>('/api/v1/health'),

  getSettings: () => apiFetch<SettingsResponse>('/api/v1/settings'),

  saveSettings: (settings: SettingsResponse) =>
    apiFetch<SettingsResponse>('/api/v1/settings', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(settings),
    }),

  scanTemplates: (req: ScanTemplatesRequest) =>
    apiFetch<ScanTemplatesResponse>('/api/v1/templates/scan', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(req),
    }),

  getTemplates: () => apiFetch<TemplatesResponse>('/api/v1/templates'),

  startCompare: (req: CompareRequest) =>
    apiFetch<CompareResponse>('/api/v1/compare', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(req),
    }),

  getCompareStatus: () => apiFetch<CompareStatus>('/api/v1/compare/status'),

  getResults: () => apiFetch<ResultsResponse>('/api/v1/results'),
};