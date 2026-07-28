import { useState, useEffect } from 'react';
import { Settings as SettingsIcon, Save, CheckCircle, XCircle, Loader2, Heart } from 'lucide-react';
import { useStore } from '../store';
import type { SettingsResponse } from '../api';

const inputStyle: React.CSSProperties = {
  background: 'var(--bg-elevated)',
  border: '1px solid var(--border-light)',
  borderRadius: 6,
  padding: '8px 12px',
  color: 'var(--text-primary)',
  fontFamily: 'var(--font-mono)',
  fontSize: 13,
  width: '100%',
};

const labelStyle: React.CSSProperties = {
  fontSize: 12,
  fontWeight: 600,
  color: 'var(--text-secondary)',
  marginBottom: 6,
  display: 'block',
};

export default function Settings() {
  const { settings, fetchSettings, saveSettings, health, fetchHealth } = useStore();

  const [form, setForm] = useState<SettingsResponse>({
    template_folder: '',
    search_folder: '',
    output_folder: '',
    recursive: true,
    move_files: false,
    group_by_content: true,
  });
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [healthChecking, setHealthChecking] = useState(false);
  const [healthResult, setHealthResult] = useState<{ ok: boolean; version: string } | null>(null);

  useEffect(() => {
    fetchSettings();
    fetchHealth();
  }, [fetchSettings, fetchHealth]);

  useEffect(() => {
    if (settings) {
      setForm(settings);
    }
  }, [settings]);

  const handleSave = async () => {
    setSaving(true);
    setError(null);
    setSaved(false);
    try {
      await saveSettings(form);
      setSaved(true);
      setTimeout(() => setSaved(false), 3000);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save settings');
    } finally {
      setSaving(false);
    }
  };

  const handleHealthCheck = async () => {
    setHealthChecking(true);
    setHealthResult(null);
    try {
      const res = await fetch('/api/v1/health');
      if (res.ok) {
        const data = await res.json();
        setHealthResult({ ok: data.status === 'ok', version: data.version || 'unknown' });
      } else {
        setHealthResult({ ok: false, version: 'unknown' });
      }
    } catch {
      setHealthResult({ ok: false, version: 'unknown' });
    } finally {
      setHealthChecking(false);
    }
  };

  return (
    <div style={{ padding: '24px', maxWidth: '700px', margin: '0 auto' }}>
      <h1 style={{ fontSize: '24px', fontWeight: 700, marginBottom: '24px', display: 'flex', alignItems: 'center', gap: '8px' }}>
        <SettingsIcon size={24} color="var(--accent)" />
        Settings
      </h1>

      {/* Health Check */}
      <div className="card" style={{ marginBottom: '16px' }}>
        <h2 style={{ fontSize: '14px', fontWeight: 600, marginBottom: '12px', display: 'flex', alignItems: 'center', gap: '8px' }}>
          <Heart size={16} color="var(--accent)" />
          API Health Check
        </h2>
        <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
          <button className="btn btn-secondary" onClick={handleHealthCheck} disabled={healthChecking}>
            {healthChecking ? <Loader2 size={16} className="spin" /> : <Heart size={16} />}
            Check Health
          </button>
          {healthResult && (
            <span style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: 13 }}>
              {healthResult.ok ? (
                <><CheckCircle size={16} color="var(--success)" /><span style={{ color: 'var(--success)' }}>Online · v{healthResult.version}</span></>
              ) : (
                <><XCircle size={16} color="var(--error)" /><span style={{ color: 'var(--error)' }}>Offline</span></>
              )}
            </span>
          )}
          {health && !healthResult && (
            <span style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: 13, color: 'var(--text-muted)' }}>
              {health.status === 'ok' ? <CheckCircle size={16} color="var(--success)" /> : <XCircle size={16} color="var(--error)" />}
              {health.status === 'ok' ? `Online · v${health.version}` : 'Offline'}
            </span>
          )}
        </div>
      </div>

      {/* Default Folders */}
      <div className="card" style={{ marginBottom: '16px' }}>
        <h2 style={{ fontSize: '14px', fontWeight: 600, marginBottom: '16px' }}>Default Folders</h2>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
          <div>
            <label style={labelStyle}>Template Folder</label>
            <input
              type="text"
              value={form.template_folder}
              onChange={(e) => setForm({ ...form, template_folder: e.target.value })}
              placeholder="C:\path\to\templates"
              style={inputStyle}
            />
          </div>
          <div>
            <label style={labelStyle}>Search Folder</label>
            <input
              type="text"
              value={form.search_folder}
              onChange={(e) => setForm({ ...form, search_folder: e.target.value })}
              placeholder="C:\path\to\search"
              style={inputStyle}
            />
          </div>
          <div>
            <label style={labelStyle}>Output Folder</label>
            <input
              type="text"
              value={form.output_folder}
              onChange={(e) => setForm({ ...form, output_folder: e.target.value })}
              placeholder="C:\path\to\output"
              style={inputStyle}
            />
          </div>
        </div>
      </div>

      {/* Default Options */}
      <div className="card" style={{ marginBottom: '16px' }}>
        <h2 style={{ fontSize: '14px', fontWeight: 600, marginBottom: '16px' }}>Default Options</h2>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
          <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
            <input
              type="checkbox"
              checked={form.recursive}
              onChange={(e) => setForm({ ...form, recursive: e.target.checked })}
            />
            <span style={{ fontSize: 13, color: 'var(--text-primary)' }}>Recursive scan (search subdirectories)</span>
          </label>
          <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
            <input
              type="checkbox"
              checked={form.group_by_content}
              onChange={(e) => setForm({ ...form, group_by_content: e.target.checked })}
            />
            <span style={{ fontSize: 13, color: 'var(--text-primary)' }}>Group by content hash</span>
          </label>
          <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
            <input
              type="checkbox"
              checked={form.move_files}
              onChange={(e) => setForm({ ...form, move_files: e.target.checked })}
            />
            <span style={{ fontSize: 13, color: 'var(--text-primary)' }}>Move files to output folder</span>
          </label>
        </div>
      </div>

      {/* Save Button */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
        <button className="btn btn-primary" onClick={handleSave} disabled={saving}>
          {saving ? <Loader2 size={16} className="spin" /> : <Save size={16} />}
          Save Settings
        </button>
        {saved && (
          <span style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: 13, color: 'var(--success)' }}>
            <CheckCircle size={16} />
            Saved successfully
          </span>
        )}
        {error && (
          <span style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: 13, color: 'var(--error)' }}>
            <XCircle size={16} />
            {error}
          </span>
        )}
      </div>
    </div>
  );
}