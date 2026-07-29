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

export interface Project {
  id: string;
  name: string;
  template_folder: string;
  search_folder: string;
  output_folder: string;
  created_at: string;
  last_used: string;
  recursive: boolean;
  group_by_content: boolean;
  move_files: boolean;
}

export interface ProjectsResponse {
  projects: Project[];
  active_id: string;
  count: number;
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

export interface TreeNode {
  name: string;
  path: string;
  is_dir: boolean;
  size: number;
  modified: string;
  children?: TreeNode[];
  is_template: boolean;
  is_mod: boolean;
  file_count: number;
  dxf_count: number;
}

export interface BrowseResponse {
  tree: TreeNode | null;
  empty: boolean;
}

export interface DiffEntity {
  type: string;
  status: string;
  coords: number[];
  coords_2d: number[][];
  block_name: string;
  layer: string;
}

export interface DiffResponse {
  template_file: string;
  module_file: string;
  template_entities: DiffEntity[];
  module_entities: DiffEntity[];
  added: DiffEntity[];
  removed: DiffEntity[];
  modified: DiffEntity[];
  bounding_box: [number, number, number, number];
  summary: {
    template_count: number;
    module_count: number;
    added_count: number;
    removed_count: number;
    modified_count: number;
  };
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

  // Projects
  getProjects: () => apiFetch<ProjectsResponse>('/api/v1/projects'),
  createProject: (data: { name: string; template_folder: string; search_folder: string; output_folder?: string; recursive?: boolean; group_by_content?: boolean }) =>
    apiFetch<{ ok: boolean; project: Project }>('/api/v1/projects', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    }),
  getProject: (id: string) =>
    apiFetch<{ project: Project }>(`/api/v1/project?id=${encodeURIComponent(id)}`),
  updateProject: (id: string, data: Partial<Project>) =>
    apiFetch<{ ok: boolean; project: Project }>(`/api/v1/project?id=${encodeURIComponent(id)}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    }),
  deleteProject: (id: string) =>
    apiFetch<{ ok: boolean }>(`/api/v1/project?id=${encodeURIComponent(id)}`, {
      method: 'DELETE',
    }),

  // Templates
  scanTemplates: (req: ScanTemplatesRequest) =>
    apiFetch<ScanTemplatesResponse>('/api/v1/templates/scan', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(req),
    }),
  getTemplates: () => apiFetch<TemplatesResponse>('/api/v1/templates'),

  // Compare
  startCompare: (req: CompareRequest) =>
    apiFetch<CompareResponse>('/api/v1/compare', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(req),
    }),
  getCompareStatus: () => apiFetch<CompareStatus>('/api/v1/compare/status'),
  getResults: () => apiFetch<ResultsResponse>('/api/v1/results'),

  // Browse
  browse: (path?: string) =>
    apiFetch<BrowseResponse>(`/api/v1/browse${path ? `?path=${encodeURIComponent(path)}` : ''}`),
  browseFolder: (path: string) =>
    apiFetch<{ children: TreeNode[] }>(`/api/v1/browse/folder?path=${encodeURIComponent(path)}`),

  // Diff
  diff: (templatePath: string, modulePath: string) =>
    apiFetch<DiffResponse>('/api/v1/diff', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ template_path: templatePath, module_path: modulePath }),
    }),

  // Create template
  createTemplate: (sourceFile: string, templateFolder?: string, templateName?: string) =>
    apiFetch<{ ok: boolean; message: string; path: string; name: string }>('/api/v1/template/create', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ source_file: sourceFile, template_folder: templateFolder, template_name: templateName }),
    }),
};