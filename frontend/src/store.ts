import { create } from 'zustand';
import type {
  HealthResponse,
  SettingsResponse,
  CompareStatus,
  ComparisonResult,
  TemplateMapping,
} from './api';
import { api } from './api';

interface DXFStore {
  // Health
  health: HealthResponse | null;
  healthLoading: boolean;

  // Settings
  settings: SettingsResponse | null;
  settingsLoading: boolean;

  // Templates
  templateCount: number;
  templateMapping: TemplateMapping;
  templatesLoading: boolean;

  // Compare state
  compareStatus: CompareStatus | null;
  compareRunning: boolean;

  // Results
  results: ComparisonResult[];
  resultsLoading: boolean;

  // Logs
  logs: string[];

  // Actions
  fetchHealth: () => Promise<void>;
  fetchSettings: () => Promise<void>;
  saveSettings: (s: SettingsResponse) => Promise<void>;
  scanTemplates: (templateFolder: string, recursive: boolean) => Promise<void>;
  fetchTemplates: () => Promise<void>;
  fetchCompareStatus: () => Promise<void>;
  fetchResults: () => Promise<void>;
  setLogs: (logs: string[]) => void;
  clearLogs: () => void;
}

export const useStore = create<DXFStore>((set) => ({
  health: null,
  healthLoading: false,
  settings: null,
  settingsLoading: false,
  templateCount: 0,
  templateMapping: {},
  templatesLoading: false,
  compareStatus: null,
  compareRunning: false,
  results: [],
  resultsLoading: false,
  logs: [],

  fetchHealth: async () => {
    set({ healthLoading: true });
    try {
      const h = await api.health();
      set({ health: h, healthLoading: false });
    } catch {
      set({ health: null, healthLoading: false });
    }
  },

  fetchSettings: async () => {
    set({ settingsLoading: true });
    try {
      const s = await api.getSettings();
      set({ settings: s, settingsLoading: false });
    } catch {
      set({ settings: null, settingsLoading: false });
    }
  },

  saveSettings: async (s: SettingsResponse) => {
    const saved = await api.saveSettings(s);
    set({ settings: saved });
  },

  scanTemplates: async (templateFolder: string, recursive: boolean) => {
    set({ templatesLoading: true });
    try {
      const r = await api.scanTemplates({ template_folder: templateFolder, recursive });
      set({ templateCount: r.count, templateMapping: r.mapping, templatesLoading: false });
    } catch {
      set({ templatesLoading: false });
      throw new Error('Failed to scan templates');
    }
  },

  fetchTemplates: async () => {
    set({ templatesLoading: true });
    try {
      const r = await api.getTemplates();
      set({ templateCount: r.count, templateMapping: r.mapping, templatesLoading: false });
    } catch {
      set({ templatesLoading: false });
    }
  },

  fetchCompareStatus: async () => {
    try {
      const s = await api.getCompareStatus();
      set({ compareStatus: s, compareRunning: s.running, logs: s.recent_logs || [] });
    } catch {
      // ignore
    }
  },

  fetchResults: async () => {
    set({ resultsLoading: true });
    try {
      const r = await api.getResults();
      set({ results: r.results || [], resultsLoading: false });
    } catch {
      set({ resultsLoading: false });
    }
  },

  setLogs: (logs: string[]) => set({ logs }),
  clearLogs: () => set({ logs: [] }),
}));