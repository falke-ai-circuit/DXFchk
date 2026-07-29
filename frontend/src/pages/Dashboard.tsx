import { useEffect, useState, useRef } from 'react';
import {
  FolderOpen, Plus, Trash2, Folder, FileSearch, Files,
  FileCheck, FileDiff, FileQuestion, Activity, Loader2, FolderCog,
  Download, Upload, Save, X, Package,
} from 'lucide-react';
import { useStore } from '../store';
import { api, type Project } from '../api';
import FolderBrowser from '../components/FolderBrowser';

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
  const [editingProject, setEditingProject] = useState<Project | null>(null);
  const [name, setName] = useState('');
  const [templateFolder, setTemplateFolder] = useState('');
  const [searchFolder, setSearchFolder] = useState('');
  const [outputFolder, setOutputFolder] = useState('');
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [browserTarget, setBrowserTarget] = useState<'template' | 'search' | 'output' | null>(null);
  const fileImportRef = useRef<HTMLInputElement>(null);
  const zipImportRef = useRef<HTMLInputElement>(null);

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
    // Also open edit modal so user can adjust folders
    const proj = projects.find(p => p.id === id);
    if (proj) {
      setEditingProject({ ...proj });
    }
  };

  const handleExport = (id: string, e: React.MouseEvent) => {
    e.stopPropagation();
    // Download project config as JSON
    api.exportProject(id).then(project => {
      const blob = new Blob([JSON.stringify(project, null, 2)], { type: 'application/json' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `${id}.json`;
      a.click();
      URL.revokeObjectURL(url);
    }).catch(() => {});
  };

  const handleZipExport = (id: string, e: React.MouseEvent) => {
    e.stopPropagation();
    // Download full project as zip
    const url = api.zipExportUrl(id);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${id}.zip`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
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
    if (fileImportRef.current) fileImportRef.current.value = '';
  };

  const handleZipImport = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    try {
      await api.zipImport(file);
      await fetchProjects();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to import zip');
    }
    if (zipImportRef.current) zipImportRef.current.value = '';
  };

  const handleEditProject = (project: Project, e: React.MouseEvent) => {
    e.stopPropagation();
    setEditingProject({ ...project });
  };

  const handleSaveEdit = async () => {
    if (!editingProject) return;
    try {
      await api.updateProject(editingProject.id, {
        name: editingProject.name,
        template_folder: editingProject.template_folder,
        search_folder: editingProject.search_folder,
        output_folder: editingProject.output_folder,
        recursive: editingProject.recursive,
        group_by_content: editingProject.group_by_content,
        move_files: editingProject.move_files,
      });
      await fetchProjects();
      setEditingProject(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save');
    }
  };

  const handleBrowserSelect = (path: string) => {
    if (browserTarget === 'template') setTemplateFolder(path);
    else if (browserTarget === 'search') setSearchFolder(path);
    else if (browserTarget === 'output') setOutputFolder(path);
    setBrowserTarget(null);
  };

  const handleEditBrowserSelect = (path: string) => {
    if (!editingProject) return;
    if (browserTarget === 'template') setEditingProject({ ...editingProject, template_folder: path });
    else if (browserTarget === 'search') setEditingProject({ ...editingProject, search_folder: path });
    else if (browserTarget === 'output') setEditingProject({ ...editingProject, output_folder: path });
    setBrowserTarget(null);
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
          <p style={{ color: 'var(--text-muted)', fontSize: '14px' }}>No active project. Create one below, import a config, or import a zip.</p>
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
      <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '12px', flexWrap: 'wrap' }}>
        <h2 style={{ fontSize: '14px', fontWeight: 600 }}>Projects</h2>
        <button className="btn btn-primary" style={{ marginLeft: 'auto', padding: '6px 12px', fontSize: 12 }} onClick={() => setShowCreate(!showCreate)}>
          <Plus size={14} /> New Project
        </button>
        <button className="btn btn-secondary" style={{ padding: '6px 12px', fontSize: 12 }} onClick={() => fileImportRef.current?.click()} title="Import project config JSON">
          <Upload size={14} /> Import Config
        </button>
        <button className="btn btn-secondary" style={{ padding: '6px 12px', fontSize: 12 }} onClick={() => zipImportRef.current?.click()} title="Import full project zip (folders + files)">
          <Package size={14} /> Import ZIP
        </button>
        <input ref={fileImportRef} type="file" accept=".json" style={{ display: 'none' }} onChange={handleImport} />
        <input ref={zipImportRef} type="file" accept=".zip" style={{ display: 'none' }} onChange={handleZipImport} />
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
            <FolderInput label="Template Folder (contains DXF templates)" value={templateFolder} onChange={setTemplateFolder} onBrowse={() => setBrowserTarget('template')} />
            <FolderInput label="Search Folder (contains DXF files to compare)" value={searchFolder} onChange={setSearchFolder} onBrowse={() => setBrowserTarget('search')} />
            <FolderInput label="Output Folder (optional — defaults to {search}/DXFchk_output)" value={outputFolder} onChange={setOutputFolder} onBrowse={() => setBrowserTarget('output')} />
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
              <button className="btn btn-ghost" style={{ padding: '4px 8px', fontSize: 11 }} onClick={(e) => handleEditProject(p, e)} title="Edit project details">
                <FolderCog size={12} />
              </button>
              <button className="btn btn-ghost" style={{ padding: '4px 8px', fontSize: 11 }} onClick={(e) => handleExport(p.id, e)} title="Export config JSON">
                <Download size={12} />
              </button>
              <button className="btn btn-ghost" style={{ padding: '4px 8px', fontSize: 11 }} onClick={(e) => handleZipExport(p.id, e)} title="Export full project as ZIP">
                <Package size={12} />
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

      {/* Folder Browser Modal */}
      {browserTarget && (
        <FolderBrowser
          initialPath={browserTarget === 'template' ? templateFolder : browserTarget === 'search' ? searchFolder : outputFolder}
          onSelect={editingProject ? handleEditBrowserSelect : handleBrowserSelect}
          onClose={() => setBrowserTarget(null)}
        />
      )}

      {/* Edit Project Modal */}
      {editingProject && (
        <div
          style={{ position: 'fixed', top: 0, left: 0, right: 0, bottom: 0, background: 'rgba(0,0,0,0.6)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1000 }}
          onClick={() => setEditingProject(null)}
        >
          <div
            style={{ background: 'var(--bg-secondary)', border: '1px solid var(--border)', borderRadius: 12, width: '600px', maxHeight: '80vh', overflow: 'auto', padding: '24px' }}
            onClick={(e) => e.stopPropagation()}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '20px' }}>
              <FolderCog size={20} color="var(--accent)" />
              <h2 style={{ fontSize: '18px', fontWeight: 700 }}>Edit Project</h2>
              <button className="btn btn-ghost" style={{ marginLeft: 'auto', padding: '4px 8px' }} onClick={() => setEditingProject(null)}>
                <X size={16} />
              </button>
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
              <div>
                <label style={labelStyle}>Project Name</label>
                <input type="text" value={editingProject.name} onChange={(e) => setEditingProject({ ...editingProject, name: e.target.value })} style={inputStyle} />
              </div>
              <FolderInput label="Template Folder" value={editingProject.template_folder} onChange={(v) => setEditingProject({ ...editingProject, template_folder: v })} onBrowse={() => setBrowserTarget('template')} />
              <FolderInput label="Search Folder" value={editingProject.search_folder} onChange={(v) => setEditingProject({ ...editingProject, search_folder: v })} onBrowse={() => setBrowserTarget('search')} />
              <FolderInput label="Output Folder" value={editingProject.output_folder} onChange={(v) => setEditingProject({ ...editingProject, output_folder: v })} onBrowse={() => setBrowserTarget('output')} />
              <div style={{ display: 'flex', gap: '12px', marginTop: '8px' }}>
                <label style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: 12 }}>
                  <input type="checkbox" checked={editingProject.recursive} onChange={(e) => setEditingProject({ ...editingProject, recursive: e.target.checked })} />
                  Recursive
                </label>
                <label style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: 12 }}>
                  <input type="checkbox" checked={editingProject.group_by_content} onChange={(e) => setEditingProject({ ...editingProject, group_by_content: e.target.checked })} />
                  Group by content
                </label>
                <label style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: 12 }}>
                  <input type="checkbox" checked={editingProject.move_files} onChange={(e) => setEditingProject({ ...editingProject, move_files: e.target.checked })} />
                  Move files
                </label>
              </div>
              <div style={{ display: 'flex', gap: '8px', marginTop: '16px' }}>
                <button className="btn btn-primary" onClick={handleSaveEdit}>
                  <Save size={14} /> Save Changes
                </button>
                <button className="btn btn-ghost" onClick={() => setEditingProject(null)}>Cancel</button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

// FolderInput — text input with browse button
function FolderInput({ label, value, onChange, onBrowse }: { label: string; value: string; onChange: (v: string) => void; onBrowse: () => void }) {
  return (
    <div>
      <label style={labelStyle}>{label}</label>
      <div style={{ display: 'flex', gap: '8px' }}>
        <input type="text" value={value} onChange={(e) => onChange(e.target.value)} placeholder="C:\path\to\folder" style={{ ...inputStyle, flex: 1 }} />
        <button className="btn btn-secondary" style={{ padding: '8px 12px', fontSize: 12, whiteSpace: 'nowrap' }} onClick={onBrowse}>
          <FolderOpen size={14} /> Browse
        </button>
      </div>
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