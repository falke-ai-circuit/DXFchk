import { useState, useEffect, useRef } from 'react';
import {
  FolderOpen, Play, Square, Loader2,
  Activity, FileCheck, FileDiff, FileQuestion,
  ScanLine, GitCompareArrows, RotateCcw,
} from 'lucide-react';
import { useStore } from '../store';
import { api, type CompareStatus } from '../api';

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

export default function Compare() {
  const {
    fetchSettings,
    compareStatus, fetchCompareStatus,
    fetchResults, fetchTemplates,
    scanTemplates,
    templateCount,
    logs,
    activeProject,
  } = useStore();

  const [templateFolder, setTemplateFolder] = useState('');
  const [searchFolder, setSearchFolder] = useState('');
  const [outputFolder, setOutputFolder] = useState('');
  const [recursive, setRecursive] = useState(true);
  const [groupByContent, setGroupByContent] = useState(true);
  const [moveFiles, setMoveFiles] = useState(false);
  const [scanning, setScanning] = useState(false);
  const [comparing, setComparing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [successMsg, setSuccessMsg] = useState<string | null>(null);

  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  // Load from active project or settings
  useEffect(() => {
    if (activeProject) {
      setTemplateFolder(activeProject.template_folder);
      setSearchFolder(activeProject.search_folder);
      setOutputFolder(activeProject.output_folder);
      setRecursive(activeProject.recursive);
      setMoveFiles(activeProject.move_files);
      setGroupByContent(activeProject.group_by_content);
    } else {
      fetchSettings().then(() => {
        const s = useStore.getState().settings;
        if (s) {
          setTemplateFolder(s.template_folder || '');
          setSearchFolder(s.search_folder || '');
          setOutputFolder(s.output_folder || '');
          setRecursive(s.recursive ?? true);
          setMoveFiles(s.move_files ?? false);
          setGroupByContent(s.group_by_content ?? true);
        }
      });
    }
    fetchCompareStatus();
    fetchTemplates();
  }, [activeProject, fetchSettings, fetchCompareStatus, fetchTemplates]);

  useEffect(() => {
    if (compareStatus?.running && !intervalRef.current) {
      intervalRef.current = setInterval(() => {
        fetchCompareStatus();
        fetchResults();
      }, 1000);
    } else if (!compareStatus?.running && intervalRef.current) {
      clearInterval(intervalRef.current);
      intervalRef.current = null;
      setComparing(false);
      fetchResults();
    }
  }, [compareStatus?.running, fetchCompareStatus, fetchResults]);

  useEffect(() => {
    return () => { if (intervalRef.current) clearInterval(intervalRef.current); };
  }, []);

  const handleStart = async () => {
    setError(null);
    setSuccessMsg(null);
    if (!templateFolder) { setError('Template folder is required'); return; }
    if (!searchFolder) { setError('Search folder is required'); return; }

    setScanning(true);
    try {
      await scanTemplates(templateFolder, recursive);
      setScanning(false);
      setComparing(true);
      const result = await api.startCompare({
        search_folder: searchFolder,
        recursive,
        move_files: moveFiles,
        group_by_content: groupByContent,
      });
      setSuccessMsg(result.message || 'Comparison started');
      if (intervalRef.current) clearInterval(intervalRef.current);
      intervalRef.current = setInterval(() => {
        fetchCompareStatus();
        fetchResults();
      }, 1000);
    } catch (err) {
      setScanning(false);
      setComparing(false);
      setError(err instanceof Error ? err.message : 'Failed to start comparison');
    }
  };

  const handleStop = () => {
    if (intervalRef.current) { clearInterval(intervalRef.current); intervalRef.current = null; }
    setComparing(false);
    setSuccessMsg('Comparison stopped by user');
    fetchCompareStatus();
  };

  const handleReset = () => {
    setComparing(false);
    setScanning(false);
    setError(null);
    setSuccessMsg(null);
    fetchCompareStatus();
    fetchResults();
  };

  const results = useStore.getState().results;
  const matched = results.filter(r => r.status === 'match').length;
  const different = results.filter(r => r.status === 'different').length;
  const noTemplate = results.filter(r => r.status === 'no_template').length;

  const cs: CompareStatus | null = compareStatus;
  const progress = cs?.progress ?? 0;

  return (
    <div style={{ padding: '24px', maxWidth: '900px', margin: '0 auto' }}>
      <h1 style={{ fontSize: '24px', fontWeight: 700, marginBottom: '24px' }}>Compare DXF Files</h1>

      {/* Active project indicator */}
      {activeProject && (
        <div style={{ marginBottom: '16px', padding: '8px 12px', background: 'rgba(0,138,0,0.08)', borderRadius: 6, fontSize: 12, color: 'var(--accent)', display: 'flex', alignItems: 'center', gap: '6px' }}>
          <FolderOpen size={14} />
          Project: <strong>{activeProject.name}</strong>
        </div>
      )}

      {error && (
        <div style={{ background: 'rgba(239,68,68,0.1)', border: '1px solid rgba(239,68,68,0.3)', borderRadius: 8, padding: '12px 16px', marginBottom: '16px', color: 'var(--error)', fontSize: 13 }}>
          {error}
        </div>
      )}
      {successMsg && !error && (
        <div style={{ background: 'rgba(16,185,129,0.1)', border: '1px solid rgba(16,185,129,0.3)', borderRadius: 8, padding: '12px 16px', marginBottom: '16px', color: 'var(--success)', fontSize: 13 }}>
          {successMsg}
        </div>
      )}

      {/* Folder Selection — matches Python GUI */}
      <div className="card" style={{ marginBottom: '16px' }}>
        <h2 style={{ fontSize: '14px', fontWeight: 600, marginBottom: '16px', display: 'flex', alignItems: 'center', gap: '8px' }}>
          <FolderOpen size={16} color="var(--accent)" />
          Folder Selection
        </h2>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
          <div>
            <label style={labelStyle}>Template Folder — contains original DXF templates</label>
            <input type="text" value={templateFolder} onChange={(e) => setTemplateFolder(e.target.value)} placeholder="C:\path\to\templates" style={inputStyle} />
          </div>
          <div>
            <label style={labelStyle}>Search Folder — contains DXF files to compare</label>
            <input type="text" value={searchFolder} onChange={(e) => setSearchFolder(e.target.value)} placeholder="C:\path\to\search" style={inputStyle} />
          </div>
          <div>
            <label style={labelStyle}>Output Folder — where results are saved</label>
            <div style={{ fontSize: 11, color: 'var(--text-muted)', marginBottom: 6, fontFamily: 'var(--font-mono)' }}>
              Identical files → template folders · Different files → _mod folders · No template → 'notemplate' folder
            </div>
            <input type="text" value={outputFolder} onChange={(e) => setOutputFolder(e.target.value)} placeholder="C:\path\to\output" style={inputStyle} />
          </div>
        </div>
      </div>

      {/* Options — matches Python GUI checkboxes */}
      <div className="card" style={{ marginBottom: '16px' }}>
        <h2 style={{ fontSize: '14px', fontWeight: 600, marginBottom: '16px' }}>Options</h2>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
          <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
            <input type="checkbox" checked={recursive} onChange={(e) => setRecursive(e.target.checked)} />
            <span style={{ fontSize: 13, color: 'var(--text-primary)' }}>Search Subdirectories</span>
            <span style={{ fontSize: 11, color: 'var(--text-muted)' }}>— search recursively in all subfolders</span>
          </label>
          <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
            <input type="checkbox" checked={groupByContent} onChange={(e) => setGroupByContent(e.target.checked)} />
            <span style={{ fontSize: 13, color: 'var(--text-primary)' }}>Group by Content Differences</span>
            <span style={{ fontSize: 11, color: 'var(--text-muted)' }}>— different files grouped into _modN folders by content hash</span>
          </label>
          <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
            <input type="checkbox" checked={moveFiles} onChange={(e) => setMoveFiles(e.target.checked)} />
            <span style={{ fontSize: 13, color: 'var(--text-primary)' }}>Move files to output</span>
            <span style={{ fontSize: 11, color: 'var(--text-muted)' }}>— original files in search folder are preserved when unchecked</span>
          </label>
        </div>
      </div>

      {/* Action Buttons — matches Python GUI layout */}
      <div style={{ display: 'flex', gap: '12px', marginBottom: '24px' }}>
        <button className="btn btn-primary" onClick={handleStart} disabled={scanning || comparing}>
          {scanning ? <Loader2 size={16} className="spin" /> : <Play size={16} />}
          {scanning ? 'Scanning Templates...' : comparing ? 'Comparing...' : 'Start'}
        </button>
        <button className="btn btn-danger" onClick={handleStop} disabled={!comparing}>
          <Square size={16} />
          Stop
        </button>
        <button className="btn btn-secondary" onClick={handleReset}>
          <RotateCcw size={16} />
          Reset Session
        </button>
      </div>

      {/* Progress Section — matches Python GUI */}
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px', marginBottom: '16px' }}>
        <div style={{ display: 'flex', gap: '16px', gridTemplateColumns: '1fr 1fr' }}>
          <ProgressCard title="Templates" icon={<ScanLine size={14} />} progress={scanning ? 50 : templateCount > 0 ? 100 : 0} label={scanning ? 'Scanning...' : `${templateCount} loaded`} active={scanning} />
          <ProgressCard title="Comparison" icon={<GitCompareArrows size={14} />} progress={progress} label={cs?.running ? `${cs.processed_files} / ${cs.total_files} (${progress.toFixed(1)}%)` : 'Idle'} active={cs?.running ?? false} />
        </div>
      </div>

      {/* Summary Stats */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: '12px', marginBottom: '16px' }}>
        <MiniStat icon={<FileCheck size={16} />} label="Matched" value={matched} color="var(--success)" />
        <MiniStat icon={<FileDiff size={16} />} label="Different" value={different} color="var(--warning)" />
        <MiniStat icon={<FileQuestion size={16} />} label="No Template" value={noTemplate} color="var(--text-muted)" />
      </div>

      {/* Live Log — matches Python GUI log section */}
      <div className="card" style={{ padding: 0, overflow: 'hidden' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '12px 16px', borderBottom: '1px solid var(--border)', backgroundColor: 'var(--bg-secondary)' }}>
          <Activity size={16} color="var(--accent)" />
          <h2 style={{ fontSize: '14px', fontWeight: 600 }}>Log</h2>
          <span style={{ fontSize: '11px', color: 'var(--text-muted)', marginLeft: 'auto', fontFamily: 'var(--font-mono)' }}>
            {logs.length} entries
          </span>
        </div>
        <div style={{ padding: '12px 16px', maxHeight: '350px', overflowY: 'auto', fontFamily: 'var(--font-mono)', fontSize: '12px', color: 'var(--text-secondary)', lineHeight: 1.6 }}>
          {logs.length === 0 ? (
            <span style={{ color: 'var(--text-muted)' }}>No logs yet. Start a comparison to see real-time output.</span>
          ) : (
            logs.map((line, i) => (
              <div key={i} style={{ marginBottom: '2px' }}>
                <span style={{ color: 'var(--text-muted)' }}>{String(i + 1).padStart(4, ' ')}</span>{' '}
                {line}
              </div>
            ))
          )}
        </div>
      </div>
    </div>
  );
}

function ProgressCard({ title, icon, progress, label, active }: { title: string; icon: React.ReactNode; progress: number; label: string; active: boolean }) {
  return (
    <div className="card" style={{ flex: 1 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '10px' }}>
        {active && <Loader2 size={12} className="spin" color="var(--accent)" />}
        {icon}
        <h3 style={{ fontSize: 13, fontWeight: 600 }}>{title}</h3>
        <span style={{ fontSize: 11, color: 'var(--text-muted)', marginLeft: 'auto', fontFamily: 'var(--font-mono)' }}>{label}</span>
      </div>
      <div className="progress-track">
        <div className="progress-fill" style={{ width: `${Math.min(100, progress)}%` }} />
      </div>
    </div>
  );
}

function MiniStat({ icon, label, value, color }: { icon: React.ReactNode; label: string; value: number; color: string }) {
  return (
    <div className="card" style={{ display: 'flex', alignItems: 'center', gap: '10px', padding: '12px' }}>
      <div style={{ color, display: 'flex', alignItems: 'center' }}>{icon}</div>
      <div>
        <div style={{ fontSize: '20px', fontWeight: 700 }}>{value}</div>
        <div style={{ fontSize: '11px', color: 'var(--text-muted)' }}>{label}</div>
      </div>
    </div>
  );
}