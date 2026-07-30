import { create } from 'zustand';
import { api, type HealthResponse, type CompareStatus, type ComparisonResult, type Project, type SettingsResponse } from './api';

interface StoreState {
  // Health
  health: HealthResponse | null;
  fetchHealth: () => Promise<void>;

  // Settings
  settings: SettingsResponse | null;
  fetchSettings: () => Promise<void>;
  saveSettings: (settings: SettingsResponse) => Promise<void>;

  // Projects
  projects: Project[];
  activeProject: Project | null;
  fetchProjects: () => Promise<void>;
  selectProject: (id: string) => Promise<void>;
  createProject: (data: { name: string; project_number: string; project_path: string; template_folder: string; search_folder: string; output_folder?: string; recursive?: boolean; group_by_content?: boolean }) => Promise<Project>;
  deleteProject: (id: string) => Promise<void>;

  // Compare
  compareStatus: CompareStatus | null;
  fetchCompareStatus: () => Promise<void>;
  results: ComparisonResult[];
  fetchResults: () => Promise<void>;

  // Templates
  templateCount: number;
  fetchTemplates: () => Promise<void>;
  scanTemplates: (folder: string, recursive: boolean) => Promise<void>;

  // Logs
  logs: string[];
}

export const useStore = create<StoreState>((set, get) => ({
  health: null,
  fetchHealth: async () => {
    try {
      const h = await api.health();
      set({ health: h });
    } catch { /* ignore */ }
  },

  settings: null,
  fetchSettings: async () => {
    try {
      const s = await api.getSettings();
      set({ settings: s });
    } catch { /* ignore */ }
  },
  saveSettings: async (settings: SettingsResponse) => {
    await api.saveSettings(settings);
    set({ settings });
  },

  projects: [],
  activeProject: null,
  fetchProjects: async () => {
    try {
      const resp = await api.getProjects();
      set({ projects: resp.projects });
      if (resp.active_id) {
        const active = resp.projects.find(p => p.id === resp.active_id);
        if (active) set({ activeProject: active });
      }
    } catch { /* ignore */ }
  },
  selectProject: async (id: string) => {
    try {
      const resp = await api.getProject(id);
      set({ activeProject: resp.project });
    } catch { /* ignore */ }
  },
  createProject: async (data) => {
    const resp = await api.createProject(data);
    set({ activeProject: resp.project });
    await get().fetchProjects();
    return resp.project;
  },
  deleteProject: async (id: string) => {
    await api.deleteProject(id);
    await get().fetchProjects();
  },

  compareStatus: null,
  fetchCompareStatus: async () => {
    try {
      const s = await api.getCompareStatus();
      set({ compareStatus: s, logs: s.recent_logs || [] });
    } catch { /* ignore */ }
  },

  results: [],
  fetchResults: async () => {
    try {
      const r = await api.getResults();
      set({ results: r.results || [] });
    } catch { /* ignore */ }
  },

  templateCount: 0,
  fetchTemplates: async () => {
    try {
      const t = await api.getTemplates();
      set({ templateCount: t.count });
    } catch { /* ignore */ }
  },
  scanTemplates: async (folder: string, recursive: boolean) => {
    const resp = await api.scanTemplates({ template_folder: folder, recursive });
    set({ templateCount: resp.count });
  },

  logs: [],
}));