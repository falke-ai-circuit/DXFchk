import { useState, useEffect, useRef } from 'react';
import {
  FolderOpen, Play, Square, Loader2,
  Activity, FileCheck, FileDiff, FileQuestion,
  ScanLine, GitCompareArrows, RotateCcw, Clock, Timer, Pause, Search,
} from 'lucide-react';
import { useStore } from '../store';
import { api, type CompareStatus } from '../api';
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
  const [stopping, setStopping] = useState(false);
  const [resuming, setResuming] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [successMsg, setSuccessMsg] = useState<string | null>(null);
  const [hasSession, setHasSession] = useState(false);
  const [browserTarget, setBrowserTarget] = useState<'template' | 'search' | 'output' | null>(null);

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
    // Check for existing session
    api.getSession().then(resp => {
      if (resp.session && resp.session.status !== 'completed') {
        setHasSession(true);
      }
    }).catch(() => {});
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
      setStopping(false);
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
      setHasSession(true);
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

  const handleStop = async () => {
    setStopping(true);
    try {
      await api.stopCompare();
      setSuccessMsg('Stop signal sent — comparison will halt after current file');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to stop');
    }
    setStopping(false);
  };

  const handleResume = async () => {
    setResuming(true);
    setError(null);
    try {
      const result = await api.resumeCompare();
      setSuccessMsg(`Comparison resumed (${result.skipped} files skipped)`);
      setComparing(true);
      if (intervalRef.current) clearInterval(intervalRef.current);
      intervalRef.current = setInterval(() => {
        fetchCompareStatus();
        fetchResults();
      }, 1000);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to resume');
    }
    setResuming(false);
  };

  const handleReset = async () => {
    setComparing(false);
    setScanning(false);
    setError(null);
    setSuccessMsg(null);
    setHasSession(false);
    try {
      await api.clearSession();
    } catch { /* ignore */ }
    fetchCompareStatus();
    fetchResults();
  };

  const results = useStore.getState().results;
  const matched = compareStatus?.matched ?? results.filter(r => r.status === 'match').length;
  const different = compareStatus?.different ?? results.filter(r => r.status === 'different').length;
  const noTemplate = compareStatus?.no_template ?? results.filter(r => r.status === 'no_template').length;

  const cs: CompareStatus | null = compareStatus;
  const progress = cs?.progress ?? 0;
  const elapsedTime = cs?.elapsed_time || '00:00:00';
  const eta = cs?.eta || '--:--:--';

  return (
    <div style={{ padding: '24px', maxWidth: '1400px', margin: '0 auto' }}>
      <h1 style={{ fontSize: '24px', fontWeight: 700, marginBottom: '16px' }}>Compare DXF Files</h1>

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

      {/* Two-column layout: controls left, measurements+log right */}
      <div style={{ display: 'flex', gap: '16px', alignItems: 'flex-start' }}>
        {/* Left Column — Controls */}
        <div style={{ flex: '0 0 520px', display: 'flex', flexDirection: 'column', gap: '16px' }}>
          {/* Folder Selection */}
          <div className="card">
            <h2 style={{ fontSize: '14px', fontWeight: 600, marginBottom: '16px', display: 'flex', alignItems: 'center', gap: '8px' }}>
              <FolderOpen size={16} color="var(--accent)" />
              Folder Selection
            </h2>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
              <div>
                <label style={labelStyle}>Template Folder — contains original DXF templates</label>
                <div style={{ display: 'flex', gap: '8px' }}>
                  <input type="text" value={templateFolder} onChange={(e) => setTemplateFolder(e.target.value)} placeholder="C:\path\to\templates" style={{ ...inputStyle, flex: 1 }} />
                  <button className="btn btn-secondary" style={{ padding: '8px 12px', fontSize: 12, whiteSpace: 'nowrap' }} onClick={() => setBrowserTarget('template')}>
                    <Search size={14} /> Browse
                  </button>
                </div>
              </div>
              <div>
                <label style={labelStyle}>Search Folder — contains DXF files to compare</label>
                <div style={{ display: 'flex', gap: '8px' }}>
                  <input type="text" value={searchFolder} onChange={(e) => setSearchFolder(e.target.value)} placeholder="C:\path\to\search" style={{ ...inputStyle, flex: 1 }} />
                  <button className="btn btn-secondary" style={{ padding: '8px 12px', fontSize: 12, whiteSpace: 'nowrap' }} onClick={() => setBrowserTarget('search')}>
                    <Search size={14} /> Browse
                  </button>
                </div>
              </div>
              <div>
                <label style={labelStyle}>Output Folder — where results are saved</label>
                <div style={{ fontSize: 11, color: 'var(--text-muted)', marginBottom: 6, fontFamily: 'var(--font-mono)' }}>
                  Identical → template folders · Different → _mod folders · No template → 'notemplate'
                </div>
                <div style={{ display: 'flex', gap: '8px' }}>
                  <input type="text" value={outputFolder} onChange={(e) => setOutputFolder(e.target.value)} placeholder="C:\path\to\output" style={{ ...inputStyle, flex: 1 }} />
                  <button className="btn btn-secondary" style={{ padding: '8px 12px', fontSize: 12, whiteSpace: 'nowrap' }} onClick={() => setBrowserTarget('output')}>
                    <Search size={14} /> Browse
                  </button>
                </div>
              </div>
            </div>
          </div>

          {/* Options */}
          <div className="card">
            <h2 style={{ fontSize: '14px', fontWeight: 600, marginBottom: '16px' }}>Options</h2>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
              <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
                <input type="checkbox" checked={recursive} onChange={(e) => setRecursive(e.target.checked)} />
                <span style={{ fontSize: 13, color: 'var(--text-primary)' }}>Search Subdirectories</span>
              </label>
              <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
                <input type="checkbox" checked={groupByContent} onChange={(e) => setGroupByContent(e.target.checked)} />
                <span style={{ fontSize: 13, color: 'var(--text-primary)' }}>Group by Content Differences</span>
              </label>
              <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
                <input type="checkbox" checked={moveFiles} onChange={(e) => setMoveFiles(e.target.checked)} />
                <span style={{ fontSize: 13, color: 'var(--text-primary)' }}>Move files to output</span>
              </label>
            </div>
          </div>

          {/* Action Buttons */}
          <div style={{ display: 'flex', gap: '12px', flexWrap: 'wrap' }}>
            <button className="btn btn-primary" onClick={handleStart} disabled={scanning || comparing || stopping}>
              {scanning ? <Loader2 size={16} className="spin" /> : <Play size={16} />}
              {scanning ? 'Scanning...' : comparing ? 'Comparing...' : 'Start'}
            </button>
            <button className="btn btn-danger" onClick={handleStop} disabled={!comparing || stopping}>
              {stopping ? <Loader2 size={16} className="spin" /> : <Square size={16} />}
              {stopping ? 'Stopping...' : 'Stop'}
            </button>
            {hasSession && !comparing && (
              <button className="btn btn-primary" onClick={handleResume} disabled={resuming}>
                {resuming ? <Loader2 size={16} className="spin" /> : <Pause size={16} />}
                {resuming ? 'Resuming...' : 'Resume'}
              </button>
            )}
            <button className="btn btn-secondary" onClick={handleReset} disabled={comparing || stopping}>
              <RotateCcw size={16} />
              Reset
            </button>
          </div>
        </div>

        {/* Right Column — Measurements + Live Log */}
        <div style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: '16px', minWidth: 0 }}>
          {/* Timing + Templates row */}
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: '12px' }}>
            <TimingCard icon={<Clock size={14} />} label="Elapsed" value={elapsedTime} active={cs?.running ?? false} />
            <TimingCard icon={<Timer size={14} />} label="ETA" value={eta} active={cs?.running ?? false} />
            <div className="card" style={{ display: 'flex', alignItems: 'center', gap: '10px', padding: '12px' }}>
              <div style={{ color: 'var(--accent)', display: 'flex', alignItems: 'center' }}><ScanLine size={16} /></div>
              <div>
                <div style={{ fontSize: '18px', fontWeight: 700 }}>{templateCount}</div>
                <div style={{ fontSize: '11px', color: 'var(--text-muted)' }}>Templates</div>
              </div>
            </div>
          </div>

          {/* Progress bar */}
          <ProgressCard title="Comparison" icon={<GitCompareArrows size={14} />} progress={progress} label={cs?.running ? `${cs.processed_files} / ${cs.total_files} (${progress.toFixed(1)}%)` : 'Idle'} active={cs?.running ?? false} />

          {/* Summary Stats */}
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: '12px' }}>
            <MiniStat icon={<FileCheck size={16} />} label="Matched" value={matched} color="var(--success)" />
            <MiniStat icon={<FileDiff size={16} />} label="Different" value={different} color="var(--warning)" />
            <MiniStat icon={<FileQuestion size={16} />} label="No Template" value={noTemplate} color="var(--text-muted)" />
          </div>

          {/* Live Log */}
          <div className="card" style={{ padding: 0, overflow: 'hidden', flex: 1, minHeight: '300px', display: 'flex', flexDirection: 'column' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '12px 16px', borderBottom: '1px solid var(--border)', backgroundColor: 'var(--bg-secondary)', flexShrink: 0 }}>
              <Activity size={16} color="var(--accent)" />
              <h2 style={{ fontSize: '14px', fontWeight: 600 }}>Live Log</h2>
              <span style={{ fontSize: '11px', color: 'var(--text-muted)', marginLeft: 'auto', fontFamily: 'var(--font-mono)' }}>
                {logs.length} entries
              </span>
            </div>
            <div style={{ padding: '12px 16px', flex: 1, overflowY: 'auto', fontFamily: 'var(--font-mono)', fontSize: '12px', color: 'var(--text-secondary)', lineHeight: 1.6, maxHeight: '400px' }}>
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
      </div>

      {/* Folder Browser Modal */}
      {browserTarget && (
        <FolderBrowser
          initialPath={browserTarget === 'template' ? templateFolder : browserTarget === 'search' ? searchFolder : outputFolder}
          onSelect={(path) => {
            if (browserTarget === 'template') setTemplateFolder(path);
            else if (browserTarget === 'search') setSearchFolder(path);
            else if (browserTarget === 'output') setOutputFolder(path);
            setBrowserTarget(null);
          }}
          onClose={() => setBrowserTarget(null)}
        />
      )}
    </div>
  );
}

function TimingCard({ icon, label, value, active }: { icon: React.ReactNode; label: string; value: string; active: boolean }) {
  return (
    <div className="card" style={{ display: 'flex', alignItems: 'center', gap: '10px', padding: '12px' }}>
      <div style={{ color: active ? 'var(--accent)' : 'var(--text-muted)', display: 'flex', alignItems: 'center' }}>{icon}</div>
      <div>
        <div style={{ fontSize: '18px', fontWeight: 700, fontFamily: 'var(--font-mono)' }}>{value}</div>
        <div style={{ fontSize: '11px', color: 'var(--text-muted)' }}>{label}</div>
      </div>
    </div>
  );
}

function ProgressCard({ title, icon, progress, label, active }: { title: string; icon: React.ReactNode; progress: number; label: string; active: boolean }) {
  return (
    <div className="card">
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