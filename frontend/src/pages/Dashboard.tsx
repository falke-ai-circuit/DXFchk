import { useEffect, useState, useRef } from 'react';
import {
  FolderOpen, Plus, Trash2, Folder, FileSearch, Files,
  FileCheck, FileDiff, FileQuestion, Activity, Loader2, FolderCog,
  Download, Upload, Save, Package, ChevronDown
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
  const { health, fetchHealth,
    projects, fetchProjects,
    selectProject,
    createProject, deleteProject,
    compareStatus, fetchCompareStatus,
    results, fetchResults,
    templateCount, fetchTemplates,
    logs,
    settings, fetchSettings,
  } = useStore();

  const [showCreate, setShowCreate] = useState(false);
  const [editingProject, setEditingProject] = useState<Project | null>(null);
  const [name, setName] = useState('');
  const [projectNumber, setProjectNumber] = useState('');
  const [projectPath, setProjectPath] = useState('');
  const [templateFolder, setTemplateFolder] = useState('');
  const [searchFolder, setSearchFolder] = useState('');
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [editError, setEditError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);
  const [browserTarget, setBrowserTarget] = useState<'projectPath' | 'template' | 'search' | 'output' | null>(null);
  const [editBrowserTarget, setEditBrowserTarget] = useState<'template' | 'search' | 'output' | null>(null);
  const [copyStatus, setCopyStatus] = useState<any>(null);
  const [copyPolling, setCopyPolling] = useState(false);
  const fileImportRef = useRef<HTMLInputElement>(null);
  const zipImportRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    fetchHealth();
    fetchProjects();
    fetchCompareStatus();
    fetchResults();
    fetchTemplates();
    fetchSettings();
  }, [fetchHealth, fetchProjects, fetchCompareStatus, fetchResults, fetchTemplates, fetchSettings]);

  const handleCreate = async () => {
    setError(null);
    if (!name) { setError('Project name is required'); return; }
    if (!projectNumber) { setError('Project number is required'); return; }
    if (!projectPath) { setError('Project path (base directory) is required'); return; }
    if (!templateFolder) { setError('Source template folder is required'); return; }
    if (!searchFolder) { setError('Source unchecked folder is required'); return; }
    setCreating(true);
    try {
      const resp = await createProject({ name, project_number: projectNumber, project_path: projectPath, template_folder: templateFolder, search_folder: searchFolder });
      setShowCreate(false);
      setName(''); setProjectNumber(''); setProjectPath(''); setTemplateFolder(''); setSearchFolder('');
      // Start polling copy status
      if (resp?.id) {
        setCopyPolling(true);
        setCopyStatus({ done: false, phase: 'scanning', total_files: 0, copied_files: 0, current_file: '', elapsed: '', eta: '' });
        pollCopyStatus(resp.id);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create project');
    }
    setCreating(false);
  };

  const pollCopyStatus = (projectId: string) => {
    const poll = async () => {
      try {
        const status = await api.getCopyStatus(projectId);
        setCopyStatus(status);
        if (!status.done) {
          setTimeout(poll, 1000);
        } else {
          setCopyPolling(false);
          await fetchProjects();
          // Auto-dismiss "done" card after 5 seconds
          setTimeout(() => setCopyStatus(null), 5000);
        }
      } catch {
        setTimeout(poll, 2000);
      }
    };
    poll();
  };

  const handleDelete = async (id: string, e: React.MouseEvent) => {
    e.stopPropagation();
    if (!confirm('Delete this project?')) return;
    await deleteProject(id);
    if (editingProject?.id === id) setEditingProject(null);
  };

  const handleSelect = async (id: string) => {
    await selectProject(id);
    const proj = projects.find(p => p.id === id);
    if (proj) {
      setEditingProject({ ...proj });
      setEditError(null);
      setSaved(false);
    }
  };

  const handleSaveEdit = async () => {
    if (!editingProject) return;
    setEditError(null);
    setSaved(false);
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
      await selectProject(editingProject.id);
      setSaved(true);
      setTimeout(() => setSaved(false), 3000);
    } catch (err) {
      setEditError(err instanceof Error ? err.message : 'Failed to save');
    }
  };

  // Inline settings handlers
  const handleBrowserSelect = (path: string) => {
    if (browserTarget === 'projectPath') setProjectPath(path);
    else if (browserTarget === 'template') setTemplateFolder(path);
    else if (browserTarget === 'search') setSearchFolder(path);
    setBrowserTarget(null);
  };
  const handleEditBrowserSelect = (path: string) => {
    if (!editingProject) return;
    if (editBrowserTarget === 'template') setEditingProject({ ...editingProject, template_folder: path });
    else if (editBrowserTarget === 'search') setEditingProject({ ...editingProject, search_folder: path });
    else if (editBrowserTarget === 'output') setEditingProject({ ...editingProject, output_folder: path });
    setEditBrowserTarget(null);
  };

  const handleExport = (id: string, e: React.MouseEvent) => {
    e.stopPropagation();
    api.exportProject(id).then(project => {
      const blob = new Blob([JSON.stringify(project, null, 2)], { type: 'application/json' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a'); a.href = url; a.download = `${id}.json`; a.click();
      URL.revokeObjectURL(url);
    }).catch(() => {});
  };

  const handleZipExport = (id: string, e: React.MouseEvent) => {
    e.stopPropagation();
    const url = api.zipExportUrl(id);
    const a = document.createElement('a'); a.href = url; a.download = `${id}.zip`;
    document.body.appendChild(a); a.click(); document.body.removeChild(a);
  };

  const handleImport = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]; if (!file) return;
    try {
      const text = await file.text();
      await api.importProject(JSON.parse(text) as Project);
      await fetchProjects();
    } catch (err) { setError(err instanceof Error ? err.message : 'Import failed'); }
    if (fileImportRef.current) fileImportRef.current.value = '';
  };

  const handleZipImport = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]; if (!file) return;
    try { await api.zipImport(file); await fetchProjects(); }
    catch (err) { setError(err instanceof Error ? err.message : 'Zip import failed'); }
    if (zipImportRef.current) zipImportRef.current.value = '';
  };

  const matched = compareStatus?.matched ?? results.filter(r => r.status === 'match').length;
  const different = compareStatus?.different ?? results.filter(r => r.status === 'different').length;
  const noTemplate = compareStatus?.no_template ?? results.filter(r => r.status === 'no_template').length;
  const total = results.length;

  return (
    <div style={{ padding: '24px', maxWidth: '900px', margin: '0 auto' }}>
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '12px', marginBottom: '24px' }}>
        <h1 style={{ fontSize: '24px', fontWeight: 700 }}>DXFchk Dashboard</h1>
        {health && (
          <span style={{ fontSize: '11px', color: 'var(--text-muted)', marginLeft: 'auto', fontFamily: 'var(--font-mono)' }}>
            {health.status === 'ok' ? '🟢' : '🔴'} {health.version} {health.running ? '· running' : ''}
          </span>
        )}
      </div>

      {/* Stats Cards */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(160px, 1fr))', gap: '12px', marginBottom: '24px' }}>
        <StatCard icon={<Files size={20} />} label="Total Files" value={total} color="var(--accent)" />
        <StatCard icon={<FileCheck size={20} />} label="Matched" value={matched} color="var(--success)" />
        <StatCard icon={<FileDiff size={20} />} label="Different" value={different} color="var(--warning)" />
        <StatCard icon={<FileQuestion size={20} />} label="No Template" value={noTemplate} color="var(--text-muted)" />
        <StatCard icon={<FileSearch size={20} />} label="Templates" value={templateCount} color="var(--info)" />
      </div>

      {/* Active Project Card (LOGReport style — expanded inline) */}
      {editingProject && (
        <div className="card" style={{ marginBottom: '16px', borderLeft: '3px solid var(--accent)' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '16px' }}>
            <ChevronDown size={18} color="var(--accent)" style={{ transition: 'transform 0.2s', transform: 'rotate(0deg)' }} />
            <FolderCog size={18} color="var(--accent)" />
            <span style={{ fontSize: '16px', fontWeight: 600 }}>
              {editingProject.project_number ? editingProject.project_number.toUpperCase() : editingProject.id.toUpperCase()} — {editingProject.name}
            </span>
            <span className="badge badge-success" style={{ marginLeft: 'auto' }}>Active</span>
          </div>

          {/* Metadata grid */}
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px', marginBottom: '16px' }}>
            <div>
              <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginBottom: '4px' }}>ID</div>
              <div style={{ fontSize: '12px', fontFamily: 'var(--font-mono)' }}>{editingProject.id}</div>
            </div>
            <div>
              <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginBottom: '4px' }}>Last Used</div>
              <div style={{ fontSize: '12px', fontFamily: 'var(--font-mono)' }}>{new Date(editingProject.last_used).toLocaleString()}</div>
            </div>
            {editingProject.project_path && (
              <div style={{ gridColumn: '1 / -1' }}>
                <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginBottom: '4px' }}>Project Path</div>
                <div style={{ fontSize: '12px', fontFamily: 'var(--font-mono)' }}>{editingProject.project_path}</div>
              </div>
            )}
          </div>

          {/* Editable settings */}
          {editError && (
            <div style={{ background: 'rgba(239,68,68,0.1)', border: '1px solid rgba(239,68,68,0.3)', borderRadius: 6, padding: '8px 12px', marginBottom: '12px', color: 'var(--error)', fontSize: 12 }}>
              {editError}
            </div>
          )}
          {saved && (
            <div style={{ background: 'rgba(0,138,0,0.1)', border: '1px solid rgba(0,138,0,0.3)', borderRadius: 6, padding: '8px 12px', marginBottom: '12px', color: 'var(--success)', fontSize: 12 }}>
              ✓ Settings saved
            </div>
          )}

          <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
            <div>
              <label style={labelStyle}>Project Name</label>
              <input type="text" value={editingProject.name} onChange={(e) => setEditingProject({ ...editingProject, name: e.target.value })} style={inputStyle} />
            </div>
            <FolderInput label="Template Folder" value={editingProject.template_folder} onChange={(v) => setEditingProject({ ...editingProject, template_folder: v })} onBrowse={() => setEditBrowserTarget('template')} />
            <FolderInput label="Search Folder" value={editingProject.search_folder} onChange={(v) => setEditingProject({ ...editingProject, search_folder: v })} onBrowse={() => setEditBrowserTarget('search')} />
            <FolderInput label="Output Folder" value={editingProject.output_folder} onChange={(v) => setEditingProject({ ...editingProject, output_folder: v })} onBrowse={() => setEditBrowserTarget('output')} />

            <div style={{ display: 'flex', gap: '16px', marginTop: '4px' }}>
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

            <div style={{ display: 'flex', gap: '8px', marginTop: '8px' }}>
              <button className="btn btn-primary" onClick={handleSaveEdit}>
                <Save size={14} /> Save Changes
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Projects List Header */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '12px', flexWrap: 'wrap' }}>
        <h2 style={{ fontSize: '14px', fontWeight: 600 }}>Projects</h2>
        <button className="btn btn-primary" style={{ marginLeft: 'auto', padding: '6px 12px', fontSize: 12 }} onClick={() => setShowCreate(!showCreate)}>
          <Plus size={14} /> New Project
        </button>
        <button className="btn btn-secondary" style={{ padding: '6px 12px', fontSize: 12 }} onClick={() => fileImportRef.current?.click()} title="Import config JSON">
          <Upload size={14} /> Import
        </button>
        <button className="btn btn-secondary" style={{ padding: '6px 12px', fontSize: 12 }} onClick={() => zipImportRef.current?.click()} title="Import ZIP">
          <Package size={14} /> ZIP
        </button>
        <input ref={fileImportRef} type="file" accept=".json" style={{ display: 'none' }} onChange={handleImport} />
        <input ref={zipImportRef} type="file" accept=".zip" style={{ display: 'none' }} onChange={handleZipImport} />
      </div>

      {/* Create Form */}
      {showCreate && (
        <div className="card" style={{ marginBottom: '16px' }}>
          <h3 style={{ fontSize: '13px', fontWeight: 600, marginBottom: '12px' }}>Create New Project</h3>
          {error && (
            <div style={{ background: 'rgba(239,68,68,0.1)', border: '1px solid rgba(239,68,68,0.3)', borderRadius: 6, padding: '8px 12px', marginBottom: '12px', color: 'var(--error)', fontSize: 12 }}>
              {error}
            </div>
          )}
          <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '10px' }}>
              <div>
                <label style={labelStyle}>Project Number</label>
                <input type="text" value={projectNumber} onChange={(e) => setProjectNumber(e.target.value)} placeholder="e.g. ECLIPSE-V04-TEST" style={inputStyle} />
              </div>
              <div>
                <label style={labelStyle}>Project Name</label>
                <input type="text" value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Eclipse v04 Test" style={inputStyle} />
              </div>
            </div>
            <FolderInput label="Project Path (base directory, . = dxfchk location)" value={projectPath} onChange={setProjectPath} onBrowse={() => setBrowserTarget('projectPath')} />
            <div style={{ fontSize: 11, color: 'var(--text-muted)', padding: '6px 0', fontFamily: 'var(--font-mono)' }}>
              Creates: {projectPath || '{project_path}'} \ {projectNumber || '{project_number}'}_{name || '{name}'} \ with {settings?.folder_templates || 'templates'}/, {settings?.folder_unchecked || 'unchecked'}/, {settings?.folder_output || 'output'}/ subfolders
            </div>
            <FolderInput label="Template Source Folder (DXF templates to copy)" value={templateFolder} onChange={setTemplateFolder} onBrowse={() => setBrowserTarget('template')} />
            <FolderInput label="Unchecked Source Folder (DXF files to check)" value={searchFolder} onChange={setSearchFolder} onBrowse={() => setBrowserTarget('search')} />
            <div style={{ display: 'flex', gap: '8px' }}>
              <button className="btn btn-primary" onClick={handleCreate} disabled={creating}>
                {creating ? <Loader2 size={14} className="spin" /> : <Plus size={14} />} Create
              </button>
              <button className="btn btn-ghost" onClick={() => setShowCreate(false)}>Cancel</button>
            </div>
          </div>
        </div>
      )}

      {/* Copy Progress Card */}
      {copyPolling && copyStatus && !copyStatus.done && (
        <div className="card" style={{ marginBottom: '16px', borderLeft: '3px solid var(--accent)' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '12px' }}>
            <Loader2 size={16} className="spin" color="var(--accent)" />
            <span style={{ fontSize: '14px', fontWeight: 600 }}>
              {copyStatus.phase === 'scanning' ? 'Scanning DXF files...' :
               copyStatus.phase === 'copying_templates' ? 'Copying template files...' :
               copyStatus.phase === 'copying_unchecked' ? 'Copying unchecked files...' :
               'Setting up project...'}
            </span>
            <span style={{ marginLeft: 'auto', fontSize: '11px', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>
              {copyStatus.copied_files} / {copyStatus.total_files || '?'}
              {copyStatus.total_files > 0 && ` (${(copyStatus.copied_files / copyStatus.total_files * 100).toFixed(1)}%)`}
            </span>
          </div>
          {copyStatus.total_files > 0 && (
            <div className="progress-track" style={{ marginBottom: '8px' }}>
              <div className="progress-fill" style={{ width: `${(copyStatus.copied_files / copyStatus.total_files * 100).toFixed(1)}%` }} />
            </div>
          )}
          <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '11px', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>
            <span>Current: {copyStatus.current_file || '...'}</span>
            <span>Elapsed: {copyStatus.elapsed || '00:00:00'} · ETA: {copyStatus.eta || '...'}</span>
          </div>
        </div>
      )}

      {/* Copy Done Card */}
      {copyStatus?.done && copyStatus.phase === 'done' && (
        <div className="card" style={{ marginBottom: '16px', borderLeft: '3px solid var(--success)' }} onClick={() => setCopyStatus(null)}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <FileCheck size={16} color="var(--success)" />
            <span style={{ fontSize: '14px', fontWeight: 600 }}>Project ready — {copyStatus.copied_files} files copied in {copyStatus.elapsed}</span>
          </div>
        </div>
      )}

      {/* Project List */}
      {projects.length === 0 ? (
        <div className="card" style={{ textAlign: 'center', padding: '24px' }}>
          <FolderOpen size={32} color="var(--text-muted)" style={{ marginBottom: '8px' }} />
          <p style={{ color: 'var(--text-muted)', fontSize: '13px' }}>No projects yet. Create one to start.</p>
        </div>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
          {projects.map(p => {
            const isExpanded = editingProject?.id === p.id;
            return (
            <div
              key={p.id}
              className="card"
              onClick={() => handleSelect(p.id)}
              style={{
                cursor: 'pointer',
                borderLeft: isExpanded ? '3px solid var(--accent)' : '3px solid transparent',
                background: isExpanded ? 'rgba(0,138,0,0.06)' : undefined,
                display: 'flex', alignItems: 'center', gap: '12px', padding: '12px 16px',
                transition: 'background 0.15s',
              }}
            >
              <ChevronDown
                size={16}
                color={isExpanded ? 'var(--accent)' : 'var(--text-muted)'}
                style={{ transition: 'transform 0.2s', transform: isExpanded ? 'rotate(0deg)' : 'rotate(-90deg)', flexShrink: 0 }}
              />
              <Folder size={20} color={isExpanded ? 'var(--accent)' : 'var(--text-muted)'} />
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                  <span style={{ fontSize: 14, fontWeight: 600 }}>
                    {p.project_number ? p.project_number.toUpperCase() : p.id.toUpperCase()} — {p.name}
                  </span>
                  {isExpanded && <span className="badge badge-success" style={{ fontSize: 9, padding: '1px 6px' }}>Active</span>}
                </div>
                <div style={{ fontSize: 11, color: 'var(--text-muted)', fontFamily: 'var(--font-mono)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {p.project_path ? p.project_path.split('\\\\\\\\').pop() : `${p.template_folder.split('\\\\\\\\').pop()} → ${p.search_folder.split('\\\\\\\\').pop()}`}
                </div>
              </div>
              <span style={{ fontSize: 11, color: 'var(--text-muted)', flexShrink: 0 }}>{new Date(p.last_used).toLocaleDateString()}</span>
              <button className="btn btn-ghost" style={{ padding: '4px 8px', fontSize: 11, flexShrink: 0 }} onClick={(e) => handleExport(p.id, e)} title="Export config"><Download size={12} /></button>
              <button className="btn btn-ghost" style={{ padding: '4px 8px', fontSize: 11, flexShrink: 0 }} onClick={(e) => handleZipExport(p.id, e)} title="Export ZIP"><Package size={12} /></button>
              <button className="btn btn-danger" style={{ padding: '4px 8px', fontSize: 11, flexShrink: 0 }} onClick={(e) => handleDelete(p.id, e)}><Trash2 size={12} /></button>
            </div>
            );
          })}
        </div>
      )}

      {/* Progress + Logs */}
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
                <span style={{ fontSize: '11px', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>· {compareStatus.elapsed_time} / ETA {compareStatus.eta}</span>
              )}
            </div>
            <div className="progress-track"><div className="progress-fill" style={{ width: `${compareStatus.progress}%` }} /></div>
          </div>
          <div className="card" style={{ padding: 0, overflow: 'hidden' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '8px 12px', borderBottom: '1px solid var(--border)', backgroundColor: 'var(--bg-secondary)' }}>
              <Activity size={14} color="var(--accent)" /><span style={{ fontSize: '12px', fontWeight: 600 }}>Live Log</span>
            </div>
            <div style={{ padding: '8px 12px', maxHeight: '200px', overflowY: 'auto', fontFamily: 'var(--font-mono)', fontSize: '11px', color: 'var(--text-secondary)', lineHeight: 1.5 }}>
              {logs.map((line, i) => <div key={i}>{line}</div>)}
            </div>
          </div>
        </div>
      )}

      {/* Folder Browser Modals */}
      {browserTarget && (
        <FolderBrowser
          initialPath={browserTarget === 'projectPath' ? projectPath : browserTarget === 'template' ? templateFolder : searchFolder}
          onSelect={handleBrowserSelect} onClose={() => setBrowserTarget(null)}
        />
      )}
      {editBrowserTarget && (
        <FolderBrowser
          initialPath={editBrowserTarget === 'template' ? editingProject?.template_folder || '' : editBrowserTarget === 'search' ? editingProject?.search_folder || '' : editingProject?.output_folder || ''}
          onSelect={handleEditBrowserSelect} onClose={() => setEditBrowserTarget(null)}
        />
      )}
    </div>
  );
}

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
      <div style={{ width: 44, height: 44, borderRadius: 8, display: 'flex', alignItems: 'center', justifyContent: 'center', background: `${color}15`, color }}>{icon}</div>
      <div>
        <div style={{ fontSize: '22px', fontWeight: 700, color: 'var(--text-primary)' }}>{value}</div>
        <div style={{ fontSize: '12px', color: 'var(--text-muted)' }}>{label}</div>
      </div>
    </div>
  );
}