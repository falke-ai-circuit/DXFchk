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
  auto_create_mod_templates: boolean;
  auto_apply_to_group: boolean;
  external_editor_path?: string;
}

export interface Project {
  id: string;
  name: string;
  project_number?: string;
  project_path?: string;
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
  elapsed_time: string;
  eta: string;
  matched: number;
  different: number;
  no_template: number;
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

export interface SessionData {
  project_id: string;
  search_folder: string;
  output_folder: string;
  template_folder: string;
  recursive: boolean;
  move_files: boolean;
  group_by_content: boolean;
  start_time: string;
  end_time?: string;
  total_files: number;
  processed_files: number;
  matched: number;
  different: number;
  no_template: number;
  status: string;
  paused: boolean;
}

export interface ModGroup {
  folder_name: string;
  folder_path: string;
  file_count: number;
  files: string[];
}

export interface TemplateGroup {
  template_name: string;
  template_path: string;
  mod_folders: ModGroup[];
  matched_count: number;
  total_files: number;
}

export interface FolderEntry {
  name: string;
  path: string;
  type: string; // "drive", "folder", "parent"
}

export interface DiffAttrib {
  tag: string;
  text: string;
  x: number;
  y: number;
  height: number;
  rotation: number;
  h_align: number;
  v_align: number;
}

export interface DiffEntity {
  type: string;
  status: string;
  coords: number[];
  coords_2d: number[][];
  block_name: string;
  layer: string;
  color: number;
  rotation: number;
  scale_x: number;
  scale_y: number;
  h_align: number;
  v_align: number;
  text_height: number;
  bulges: number[];
  closed: boolean;
  block_entities?: DiffEntity[];
  block_base_x?: number;
  block_base_y?: number;
  attribs?: DiffAttrib[];
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

export interface DXFRenderResponse {
  entities: DiffEntity[];
  count: number;
  bounding_box: [number, number, number, number];
  type_counts: Record<string, number>;
  layers: string[];
  layer_count: number;
  layer_colors: Record<string, number>;
  path: string;
  name: string;
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
  createProject: (data: { name: string; project_number: string; project_path: string; template_folder: string; search_folder: string; output_folder?: string; recursive?: boolean; group_by_content?: boolean }) =>
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
  getCompareStatus: (projectId?: string) =>
    apiFetch<CompareStatus>(`/api/v1/compare/status${projectId ? `?project_id=${encodeURIComponent(projectId)}` : ''}`),
  getCompareJobs: () => apiFetch<{ jobs: any[]; count: number; running: number }>('/api/v1/compare/jobs'),
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

  // Stop comparison
  stopCompare: () =>
    apiFetch<{ ok: boolean; message: string }>('/api/v1/compare/stop', {
      method: 'POST',
    }),

  // Resume comparison
  resumeCompare: () =>
    apiFetch<{ ok: boolean; message: string; skipped: number }>('/api/v1/compare/resume', {
      method: 'POST',
    }),

  // Session
  getSession: () =>
    apiFetch<{ session: SessionData | null; elapsed_time: string; eta: string }>('/api/v1/session'),
  clearSession: () =>
    apiFetch<{ ok: boolean; message: string }>('/api/v1/session', {
      method: 'DELETE',
    }),

  // Project import/export
  exportProject: (id: string) =>
    apiFetch<Project>(`/api/v1/project/export?id=${encodeURIComponent(id)}`),
  importProject: (project: Project, mode?: string) =>
    apiFetch<{ ok: boolean; project: Project }>(`/api/v1/project/import${mode ? `?mode=${mode}` : ''}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(project),
    }),

  // Template groups (fix 90 templates workflow)
  getTemplateGroups: (output?: string) =>
    apiFetch<{ groups: TemplateGroup[]; count: number }>(`/api/v1/template/groups${output ? `?output=${encodeURIComponent(output)}` : ''}`),
  applyTemplate: (req: { template_path: string; group_name: string; output_folder?: string }) =>
    apiFetch<{ ok: boolean; message: string; group: string; mod_folders: number; total_files: number; template_path: string }>('/api/v1/template/apply', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(req),
    }),

  // Log content (read .log file)
  getLogContent: (filePath: string) =>
    apiFetch<{ content: string; lines: number }>(`/api/v1/log?path=${encodeURIComponent(filePath)}`),

  // DXF raw content (read DXF file as text)
  getDXFContent: (filePath: string) =>
    apiFetch<{ content: string; lines: number }>(`/api/v1/dxf/content?path=${encodeURIComponent(filePath)}`),

  // DXF render data (entities for visual CAD rendering)
  getDXFRender: (filePath: string) =>
    apiFetch<DXFRenderResponse>(`/api/v1/dxf/render?path=${encodeURIComponent(filePath)}`),

  // Edit DXF file (save modified content)
  saveDXFContent: (filePath: string, content: string) =>
    apiFetch<{ ok: boolean; message: string }>(`/api/v1/dxf/content`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path: filePath, content }),
    }),

  // Apply edit script to template
  applyEditScript: (templatePath: string, editScript: string, groupName: string, outputFolder?: string) =>
    apiFetch<{ ok: boolean; message: string; modified: number }>('/api/v1/template/edit-script', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ template_path: templatePath, edit_script: editScript, group_name: groupName, output_folder: outputFolder }),
    }),

  // Get template group details (with file list for apply)
  getTemplateGroupDetail: (groupName: string, output?: string) =>
    apiFetch<{ group: TemplateGroup | null }>(`/api/v1/template/group?name=${encodeURIComponent(groupName)}${output ? `&output=${encodeURIComponent(output)}` : ''}`),

  // System folder browser
  browseSystem: (path?: string) =>
    apiFetch<{ entries: FolderEntry[]; current: string }>(`/api/v1/browse/system${path ? `?path=${encodeURIComponent(path)}` : ''}`),

  // Project zip export/import
  zipExportUrl: (id: string) => `/api/v1/project/zip-export?id=${encodeURIComponent(id)}`,
  zipImport: (file: File) => {
    const formData = new FormData();
    formData.append('file', file);
    return fetch('/api/v1/project/zip-import', { method: 'POST', body: formData })
      .then(r => r.json());
  },

  // Open DXF in external editor
  openExternal: (filePath: string, editorPath?: string) =>
    apiFetch<{ ok: boolean; launched: boolean; editor: string }>('/api/v1/open', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path: filePath, editor_path: editorPath }),
    }),
};