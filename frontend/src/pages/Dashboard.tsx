import { useEffect, useState, useRef } from 'react';
import {
  FolderOpen, Plus, Trash2, Folder, FileSearch, Files,
  FileCheck, FileDiff, FileQuestion, Activity, Loader2, FolderCog,
  Download, Upload,
} from 'lucide-react';
import { useStore } from '../store';
import { api, type Project } from '../api';

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

export default function Dashboard() {
  const {
    health, fetchHealth,
    projects, fetchProjects,
    activeProject, selectProject,
    createProject, deleteProject,
    compareStatus, fetchCompareStatus,
    results, fetchResults,
    templateCount, fetchTemplates,
    logs,
  } = useStore();

  const [showCreate, setShowCreate] = useState(false);
  const [name, setName] = useState('');
  const [templateFolder, setTemplateFolder] = useState('');
  const [searchFolder, setSearchFolder] = useState('');
  const [outputFolder, setOutputFolder] = useState('');
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    fetchHealth();
    fetchProjects();
    fetchCompareStatus();
    fetchResults();
    fetchTemplates();
  }, [fetchHealth, fetchProjects, fetchCompareStatus, fetchResults, fetchTemplates]);

  const handleCreate = async () => {
    setError(null);
    if (!name) { setError('Project name is required'); return; }
    if (!templateFolder) { setError('Template folder is required'); return; }
    if (!searchFolder) { setError('Search folder is required'); return; }

    setCreating(true);
    try {
      await createProject({
        name,
        template_folder: templateFolder,
        search_folder: searchFolder,
        output_folder: outputFolder || undefined,
      });
      setShowCreate(false);
      setName(''); setTemplateFolder(''); setSearchFolder(''); setOutputFolder('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create project');
    }
    setCreating(false);
  };

  const handleDelete = async (id: string, e: React.MouseEvent) => {
    e.stopPropagation();
    if (!confirm('Delete this project?')) return;
    await deleteProject(id);
  };

  const handleSelect = async (id: string) => {
    await selectProject(id);
  };

  const handleExport = async (id: string, e: React.MouseEvent) => {
    e.stopPropagation();
    try {
      const project = await api.exportProject(id);
      const blob = new Blob([JSON.stringify(project, null, 2)], { type: 'application/json' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `${id}.json`;
      a.click();
      URL.revokeObjectURL(url);
    } catch { /* ignore */ }
  };

  const handleImport = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    try {
      const text = await file.text();
      const project = JSON.parse(text) as Project;
      await api.importProject(project);
      await fetchProjects();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to import project');
    }
    if (fileInputRef.current) fileInputRef.current.value = '';
  };

  const matched = compareStatus?.matched ?? results.filter(r => r.status === 'match').length;
  const different = compareStatus?.different ?? results.filter(r => r.status === 'different').length;
  const noTemplate = compareStatus?.no_template ?? results.filter(r => r.status === 'no_template').length;
  const total = results.length;

  return (
    <div style={{ padding: '24px', maxWidth: '1200px', margin: '0 auto' }}>
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '12px', marginBottom: '24px' }}>
        <h1 style={{ fontSize: '24px', fontWeight: 700 }}>DXFchk Dashboard</h1>
        {health && (
          <span style={{ fontSize: '11px', color: 'var(--text-muted)', marginLeft: 'auto', fontFamily: 'var(--font-mono)' }}>
            {health.status === 'ok' ? '🟢' : '🔴'} v{health.version} {health.running ? '· running' : ''}
          </span>
        )}
      </div>

      {/* Active Project Card */}
      {activeProject ? (
        <div className="card" style={{ marginBottom: '16px', borderLeft: '3px solid var(--accent)' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '12px' }}>
            <FolderCog size={18} color="var(--accent)" />
            <h2 style={{ fontSize: '16px', fontWeight: 600 }}>{activeProject.name}</h2>
            <span className="badge badge-success" style={{ marginLeft: 'auto' }}>Active</span>
          </div>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: '12px' }}>
            <div>
              <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginBottom: '4px' }}>Template Folder</div>
              <div style={{ fontSize: '12px', fontFamily: 'var(--font-mono)', color: 'var(--text-secondary)', wordBreak: 'break-all' }}>{activeProject.template_folder}</div>
            </div>
            <div>
              <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginBottom: '4px' }}>Search Folder</div>
              <div style={{ fontSize: '12px', fontFamily: 'var(--font-mono)', color: 'var(--text-secondary)', wordBreak: 'break-all' }}>{activeProject.search_folder}</div>
            </div>
            <div>
              <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginBottom: '4px' }}>Output Folder</div>
              <div style={{ fontSize: '12px', fontFamily: 'var(--font-mono)', color: 'var(--text-secondary)', wordBreak: 'break-all' }}>{activeProject.output_folder}</div>
            </div>
          </div>
        </div>
      ) : (
        <div className="card" style={{ marginBottom: '16px', textAlign: 'center', padding: '32px' }}>
          <FolderOpen size={32} color="var(--text-muted)" style={{ marginBottom: '8px' }} />
          <p style={{ color: 'var(--text-muted)', fontSize: '14px' }}>No active project. Create one below or import an existing project.</p>
        </div>
      )}

      {/* Stats Cards */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))', gap: '16px', marginBottom: '24px' }}>
        <StatCard icon={<Files size={20} />} label="Total Files" value={total} color="var(--accent)" />
        <StatCard icon={<FileCheck size={20} />} label="Matched" value={matched} color="var(--success)" />
        <StatCard icon={<FileDiff size={20} />} label="Different" value={different} color="var(--warning)" />
        <StatCard icon={<FileQuestion size={20} />} label="No Template" value={noTemplate} color="var(--text-muted)" />
        <StatCard icon={<FileSearch size={20} />} label="Templates" value={templateCount} color="var(--info)" />
      </div>

      {/* Projects List */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '12px' }}>
        <h2 style={{ fontSize: '14px', fontWeight: 600 }}>Projects</h2>
        <button className="btn btn-primary" style={{ marginLeft: 'auto', padding: '6px 12px', fontSize: 12 }} onClick={() => setShowCreate(!showCreate)}>
          <Plus size={14} />
          New Project
        </button>
        <button className="btn btn-secondary" style={{ padding: '6px 12px', fontSize: 12 }} onClick={() => fileInputRef.current?.click()}>
          <Upload size={14} />
          Import
        </button>
        <input ref={fileInputRef} type="file" accept=".json" style={{ display: 'none' }} onChange={handleImport} />
      </div>

      {/* Create Project Form */}
      {showCreate && (
        <div className="card" style={{ marginBottom: '16px' }}>
          <h3 style={{ fontSize: '13px', fontWeight: 600, marginBottom: '12px' }}>Create New Project</h3>
          {error && (
            <div style={{ background: 'rgba(239,68,68,0.1)', border: '1px solid rgba(239,68,68,0.3)', borderRadius: 6, padding: '8px 12px', marginBottom: '12px', color: 'var(--error)', fontSize: 12 }}>
              {error}
            </div>
          )}
          <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
            <div>
              <label style={labelStyle}>Project Name</label>
              <input type="text" value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Eclipse Comparison" style={inputStyle} />
            </div>
            <div>
              <label style={labelStyle}>Template Folder (contains DXF templates)</label>
              <input type="text" value={templateFolder} onChange={(e) => setTemplateFolder(e.target.value)} placeholder="C:\path\to\templates" style={inputStyle} />
            </div>
            <div>
              <label style={labelStyle}>Search Folder (contains DXF files to compare)</label>
              <input type="text" value={searchFolder} onChange={(e) => setSearchFolder(e.target.value)} placeholder="C:\path\to\modules" style={inputStyle} />
            </div>
            <div>
              <label style={labelStyle}>Output Folder (optional — defaults to {searchFolder || '<search>'}/DXFchk_output)</label>
              <input type="text" value={outputFolder} onChange={(e) => setOutputFolder(e.target.value)} placeholder="C:\path\to\output" style={inputStyle} />
            </div>
            <div style={{ display: 'flex', gap: '8px' }}>
              <button className="btn btn-primary" onClick={handleCreate} disabled={creating}>
                {creating ? <Loader2 size={14} className="spin" /> : <Plus size={14} />}
                Create Project
              </button>
              <button className="btn btn-ghost" onClick={() => setShowCreate(false)}>Cancel</button>
            </div>
          </div>
        </div>
      )}

      {/* Project List */}
      {projects.length === 0 ? (
        <div className="card" style={{ textAlign: 'center', padding: '24px' }}>
          <p style={{ color: 'var(--text-muted)', fontSize: '13px' }}>No projects yet. Create one to start comparing DXF files.</p>
        </div>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
          {projects.map(p => (
            <div
              key={p.id}
              className="card"
              onClick={() => handleSelect(p.id)}
              style={{
                cursor: 'pointer',
                borderLeft: activeProject?.id === p.id ? '3px solid var(--accent)' : '3px solid transparent',
                display: 'flex',
                alignItems: 'center',
                gap: '12px',
                padding: '12px 16px',
              }}
            >
              <Folder size={20} color={activeProject?.id === p.id ? 'var(--accent)' : 'var(--text-muted)'} />
              <div style={{ flex: 1 }}>
                <div style={{ fontSize: '14px', fontWeight: 600 }}>{p.name}</div>
                <div style={{ fontSize: '11px', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>
                  {p.template_folder.split('\\').pop()} → {p.search_folder.split('\\').pop()}
                </div>
              </div>
              <span style={{ fontSize: '11px', color: 'var(--text-muted)' }}>
                {new Date(p.last_used).toLocaleDateString()}
              </span>
              <button className="btn btn-ghost" style={{ padding: '4px 8px', fontSize: 11 }} onClick={(e) => handleExport(p.id, e)} title="Export project">
                <Download size={12} />
              </button>
              <button className="btn btn-danger" style={{ padding: '4px 8px', fontSize: 11 }} onClick={(e) => handleDelete(p.id, e)}>
                <Trash2 size={12} />
              </button>
            </div>
          ))}
        </div>
      )}

      {/* Progress + Logs (if running) */}
      {compareStatus?.running && (
        <div style={{ marginTop: '24px' }}>
          <div className="card" style={{ marginBottom: '12px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '8px' }}>
              <Loader2 size={14} className="spin" color="var(--accent)" />
              <span style={{ fontSize: '13px', fontWeight: 600 }}>Comparison Running</span>
              <span style={{ fontSize: '11px', color: 'var(--text-muted)', marginLeft: 'auto', fontFamily: 'var(--font-mono)' }}>
                {compareStatus.processed_files} / {compareStatus.total_files} ({compareStatus.progress.toFixed(1)}%)
              </span>
              {compareStatus.elapsed_time && (
                <span style={{ fontSize: '11px', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>
                  · {compareStatus.elapsed_time} / ETA {compareStatus.eta}
                </span>
              )}
            </div>
            <div className="progress-track">
              <div className="progress-fill" style={{ width: `${compareStatus.progress}%` }} />
            </div>
          </div>
          <div className="card" style={{ padding: 0, overflow: 'hidden' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '8px 12px', borderBottom: '1px solid var(--border)', backgroundColor: 'var(--bg-secondary)' }}>
              <Activity size={14} color="var(--accent)" />
              <span style={{ fontSize: '12px', fontWeight: 600 }}>Live Log</span>
            </div>
            <div style={{ padding: '8px 12px', maxHeight: '200px', overflowY: 'auto', fontFamily: 'var(--font-mono)', fontSize: '11px', color: 'var(--text-secondary)', lineHeight: 1.5 }}>
              {logs.map((line, i) => <div key={i}>{line}</div>)}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function StatCard({ icon, label, value, color }: { icon: React.ReactNode; label: string; value: number | string; color: string }) {
  return (
    <div className="card" style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
      <div style={{ width: 44, height: 44, borderRadius: 8, display: 'flex', alignItems: 'center', justifyContent: 'center', background: `${color}15`, color }}>
        {icon}
      </div>
      <div>
        <div style={{ fontSize: '22px', fontWeight: 700, color: 'var(--text-primary)' }}>{value}</div>
        <div style={{ fontSize: '12px', color: 'var(--text-muted)' }}>{label}</div>
      </div>
    </div>
  );
}