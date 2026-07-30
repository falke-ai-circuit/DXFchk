import { useState, useEffect, useRef, useCallback } from 'react';
import {
  FolderOpen, Play, Square, Loader2,
  Activity, FileCheck, FileDiff, FileQuestion,
  ScanLine, GitCompareArrows, RotateCcw, Clock, Timer, Pause, Search,
  Layers,
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
    fetchResults, fetchTemplates,
    templateCount,
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

  // Multi-job switcher state (local — not in store)
  const [activeJobId, setActiveJobId] = useState<string | null>(null);
  const [allJobs, setAllJobs] = useState<any[]>([]);
  const [localStatus, setLocalStatus] = useState<CompareStatus | null>(null);
  const [localLogs, setLocalLogs] = useState<string[]>([]);

  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const jobsIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const logEndRef = useRef<HTMLDivElement | null>(null);
  const activeJobIdRef = useRef<string | null>(null);
  activeJobIdRef.current = activeJobId;

  // Auto-scroll log to bottom
  useEffect(() => {
    if (logEndRef.current) {
      logEndRef.current.scrollIntoView({ behavior: 'smooth' });
    }
  }, [localLogs]);

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
    fetchTemplates();
    api.getSession().then(resp => {
      if (resp.session && resp.session.status !== 'completed') {
        setHasSession(true);
      }
    }).catch(() => {});
  }, [activeProject, fetchSettings, fetchTemplates]);

  // Fetch all jobs on mount + every 3s
  const fetchJobs = useCallback(async () => {
    try {
      const resp = await api.getCompareJobs();
      setAllJobs(resp.jobs || []);
      // Auto-select: if activeProject changed and has a job, switch to it
      if (activeProject) {
        const projJob = (resp.jobs || []).find((j: any) => j.project_id === activeProject.id || j.project_name === activeProject.name);
        if (projJob) {
          setActiveJobId(prev => prev !== projJob.id ? projJob.id : prev);
          return;
        }
      }
      // If no activeJobId yet, pick first running job, or first job
      if (!activeJobIdRef.current && (resp.jobs || []).length > 0) {
        const running = (resp.jobs || []).find((j: any) => j.running);
        setActiveJobId(running ? running.id : resp.jobs[0].id);
      }
    } catch { /* ignore */ }
  }, [activeProject]);

  useEffect(() => {
    fetchJobs();
    jobsIntervalRef.current = setInterval(fetchJobs, 3000);
    return () => { if (jobsIntervalRef.current) clearInterval(jobsIntervalRef.current); };
  }, [fetchJobs]);

  // Fetch status for the active job
  const fetchJobStatus = useCallback(async () => {
    const jobId = activeJobIdRef.current;
    try {
      const s = await api.getCompareStatus(jobId || undefined);
      setLocalStatus(s);
      setLocalLogs(s.recent_logs || []);
    } catch { /* ignore */ }
  }, []);

  useEffect(() => {
    fetchJobStatus();
  }, [activeJobId, fetchJobStatus]);

  // Poll status every 1s when running
  useEffect(() => {
    if (localStatus?.running && !intervalRef.current) {
      intervalRef.current = setInterval(() => {
        fetchJobStatus();
        fetchResults();
      }, 1000);
    } else if (!localStatus?.running && intervalRef.current) {
      clearInterval(intervalRef.current);
      intervalRef.current = null;
      setComparing(false);
      setStopping(false);
      fetchResults();
    }
  }, [localStatus?.running, fetchJobStatus, fetchResults]);

  useEffect(() => {
    return () => { if (intervalRef.current) clearInterval(intervalRef.current); };
  }, []);

  const handleStart = async () => {
    setError(null);
    setSuccessMsg(null);
    if (!templateFolder) { setError('Template folder is required'); return; }
    if (!searchFolder) { setError('Search folder is required'); return; }

    setComparing(true);
    setSuccessMsg('Starting comparison...');
    try {
      const result = await api.startCompare({
        search_folder: searchFolder,
        recursive,
        move_files: moveFiles,
        group_by_content: groupByContent,
        template_folder: templateFolder,
        output_folder: outputFolder,
        project_id: activeProject?.id,
        project_name: activeProject?.name,
      } as any);
      setSuccessMsg(result.message || 'Comparison started');
      setHasSession(true);
      if (intervalRef.current) clearInterval(intervalRef.current);
      intervalRef.current = setInterval(() => {
        fetchJobStatus();
        fetchResults();
      }, 1000);
    } catch (err) {
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
        fetchJobStatus();
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
    fetchJobStatus();
    fetchResults();
  };

  const results = useStore.getState().results;
  const cs: CompareStatus | null = localStatus;
  const matched = cs?.matched ?? results.filter(r => r.status === 'match').length;
  const different = cs?.different ?? results.filter(r => r.status === 'different').length;
  const noTemplate = cs?.no_template ?? results.filter(r => r.status === 'no_template').length;

  const progress = cs?.progress ?? 0;
  const elapsedTime = cs?.elapsed_time || '00:00:00';
  const eta = cs?.eta || '--:--:--';

  return (
    <div style={{ display: 'flex', height: '100%', overflow: 'hidden' }}>
      {/* LEFT SIDE — All controls, stats, ETA, actions */}
      <div style={{ flex: '0 0 600px', flexShrink: 0, display: 'flex', flexDirection: 'column', overflowY: 'auto', padding: '16px', borderRight: '1px solid var(--border)' }}>
        <h1 style={{ fontSize: '20px', fontWeight: 700, marginBottom: '12px' }}>Compare DXF Files</h1>

        {/* Active project indicator */}
        {activeProject && (
          <div style={{ marginBottom: '12px', padding: '6px 10px', background: 'rgba(0,138,0,0.08)', borderRadius: 6, fontSize: 12, color: 'var(--accent)', display: 'flex', alignItems: 'center', gap: '6px' }}>
            <FolderOpen size={14} />
            Project: <strong>{activeProject.name}</strong>
          </div>
        )}

        {/* Multi-job switcher */}
        {allJobs.length > 0 && (
          <div style={{ marginBottom: '12px' }}>
            <div style={{ fontSize: 11, fontWeight: 600, color: 'var(--text-secondary)', marginBottom: 6, display: 'flex', alignItems: 'center', gap: '4px' }}>
              <Layers size={12} /> Comparison Jobs {allJobs.length > 1 && `(${allJobs.length})`}
            </div>
            <div style={{ display: 'flex', gap: '4px', flexWrap: 'wrap' }}>
              {allJobs.map((job: any) => {
                const isActive = job.id === activeJobId;
                return (
                  <button
                    key={job.id}
                    onClick={() => setActiveJobId(job.id)}
                    style={{
                      padding: '4px 10px',
                      fontSize: 11,
                      borderRadius: 6,
                      border: `1px solid ${isActive ? 'var(--accent)' : 'var(--border-light)'}`,
                      background: isActive ? 'rgba(0,138,0,0.12)' : 'var(--bg-elevated)',
                      color: isActive ? 'var(--accent)' : 'var(--text-secondary)',
                      cursor: 'pointer',
                      fontWeight: isActive ? 600 : 400,
                      fontFamily: 'var(--font-mono)',
                      display: 'flex',
                      alignItems: 'center',
                      gap: '4px',
                    }}
                  >
                    {job.running && <Loader2 size={10} className="spin" />}
                    {job.project_name || job.id.slice(0, 8)}
                    {job.progress !== undefined && (
                      <span style={{ fontSize: 9, opacity: 0.7 }}>{job.progress.toFixed(0)}%</span>
                    )}
                  </button>
                );
              })}
            </div>
          </div>
        )}

        {error && (
          <div style={{ background: 'rgba(239,68,68,0.1)', border: '1px solid rgba(239,68,68,0.3)', borderRadius: 6, padding: '8px 12px', marginBottom: '12px', color: 'var(--error)', fontSize: 12 }}>
            {error}
          </div>
        )}
        {successMsg && !error && (
          <div style={{ background: 'rgba(16,185,129,0.1)', border: '1px solid rgba(16,185,129,0.3)', borderRadius: 6, padding: '8px 12px', marginBottom: '12px', color: 'var(--success)', fontSize: 12 }}>
            {successMsg}
          </div>
        )}

        {/* Folder Selection */}
        <div className="card" style={{ marginBottom: '12px' }}>
          <h2 style={{ fontSize: '13px', fontWeight: 600, marginBottom: '12px', display: 'flex', alignItems: 'center', gap: '8px' }}>
            <FolderOpen size={15} color="var(--accent)" />
            Folder Selection
          </h2>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
            <div>
              <label style={{ ...labelStyle, fontSize: 11 }}>Template Folder</label>
              <div style={{ display: 'flex', gap: '6px' }}>
                <input type="text" value={templateFolder} onChange={(e) => setTemplateFolder(e.target.value)} placeholder="C:\path\to\templates" style={{ ...inputStyle, flex: 1, fontSize: 12, padding: '6px 10px' }} />
                <button className="btn btn-secondary" style={{ padding: '6px 10px', fontSize: 11, whiteSpace: 'nowrap' }} onClick={() => setBrowserTarget('template')}>
                  <Search size={13} />
                </button>
              </div>
            </div>
            <div>
              <label style={{ ...labelStyle, fontSize: 11 }}>Search Folder</label>
              <div style={{ display: 'flex', gap: '6px' }}>
                <input type="text" value={searchFolder} onChange={(e) => setSearchFolder(e.target.value)} placeholder="C:\path\to\search" style={{ ...inputStyle, flex: 1, fontSize: 12, padding: '6px 10px' }} />
                <button className="btn btn-secondary" style={{ padding: '6px 10px', fontSize: 11, whiteSpace: 'nowrap' }} onClick={() => setBrowserTarget('search')}>
                  <Search size={13} />
                </button>
              </div>
            </div>
            <div>
              <label style={{ ...labelStyle, fontSize: 11 }}>Output Folder</label>
              <div style={{ fontSize: 10, color: 'var(--text-muted)', marginBottom: 4, fontFamily: 'var(--font-mono)' }}>
                Identical → template folders · Different → _mod folders · No template → 'notemplate'
              </div>
              <div style={{ display: 'flex', gap: '6px' }}>
                <input type="text" value={outputFolder} onChange={(e) => setOutputFolder(e.target.value)} placeholder="C:\path\to\output" style={{ ...inputStyle, flex: 1, fontSize: 12, padding: '6px 10px' }} />
                <button className="btn btn-secondary" style={{ padding: '6px 10px', fontSize: 11, whiteSpace: 'nowrap' }} onClick={() => setBrowserTarget('output')}>
                  <Search size={13} />
                </button>
              </div>
            </div>
          </div>
        </div>

        {/* Options */}
        <div className="card" style={{ marginBottom: '12px' }}>
          <h2 style={{ fontSize: '13px', fontWeight: 600, marginBottom: '10px' }}>Options</h2>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
              <input type="checkbox" checked={recursive} onChange={(e) => setRecursive(e.target.checked)} />
              <span style={{ fontSize: 12 }}>Search Subdirectories</span>
            </label>
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
              <input type="checkbox" checked={groupByContent} onChange={(e) => setGroupByContent(e.target.checked)} />
              <span style={{ fontSize: 12 }}>Group by Content Differences</span>
            </label>
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
              <input type="checkbox" checked={moveFiles} onChange={(e) => setMoveFiles(e.target.checked)} />
              <span style={{ fontSize: 12 }}>Move files to output</span>
            </label>
          </div>
        </div>

        {/* Action Buttons */}
        <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap', marginBottom: '12px' }}>
          <button className="btn btn-primary" onClick={handleStart} disabled={scanning || comparing || stopping} style={{ fontSize: 12, padding: '6px 14px' }}>
            {scanning ? <Loader2 size={14} className="spin" /> : <Play size={14} />}
            {scanning ? 'Scanning...' : comparing ? 'Comparing...' : 'Start'}
          </button>
          <button className="btn btn-danger" onClick={handleStop} disabled={!comparing || stopping} style={{ fontSize: 12, padding: '6px 14px' }}>
            {stopping ? <Loader2 size={14} className="spin" /> : <Square size={14} />}
            {stopping ? 'Stopping...' : 'Stop'}
          </button>
          {hasSession && !comparing && (
            <button className="btn btn-primary" onClick={handleResume} disabled={resuming} style={{ fontSize: 12, padding: '6px 14px' }}>
              {resuming ? <Loader2 size={14} className="spin" /> : <Pause size={14} />}
              {resuming ? 'Resuming...' : 'Resume'}
            </button>
          )}
          <button className="btn btn-secondary" onClick={handleReset} disabled={comparing || stopping} style={{ fontSize: 12, padding: '6px 14px' }}>
            <RotateCcw size={14} />
            Reset
          </button>
        </div>

        {/* Timing + Templates */}
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: '8px', marginBottom: '10px' }}>
          <TimingCard icon={<Clock size={13} />} label="Elapsed" value={elapsedTime} active={cs?.running ?? false} />
          <TimingCard icon={<Timer size={13} />} label="ETA" value={eta} active={cs?.running ?? false} />
          <div className="card" style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '10px' }}>
            <div style={{ color: 'var(--accent)', display: 'flex', alignItems: 'center' }}><ScanLine size={15} /></div>
            <div>
              <div style={{ fontSize: '16px', fontWeight: 700 }}>{templateCount}</div>
              <div style={{ fontSize: '10px', color: 'var(--text-muted)' }}>Templates</div>
            </div>
          </div>
        </div>

        {/* Progress bar */}
        <ProgressCard title="Comparison" icon={<GitCompareArrows size={13} />} progress={progress} label={cs?.running ? `${cs.processed_files} / ${cs.total_files} (${progress.toFixed(1)}%)` : 'Idle'} active={cs?.running ?? false} />

        {/* Summary Stats */}
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: '8px', marginTop: '10px' }}>
          <MiniStat icon={<FileCheck size={15} />} label="Matched" value={matched} color="var(--success)" />
          <MiniStat icon={<FileDiff size={15} />} label="Different" value={different} color="var(--warning)" />
          <MiniStat icon={<FileQuestion size={15} />} label="No Template" value={noTemplate} color="var(--text-muted)" />
        </div>
      </div>

      {/* RIGHT SIDE — Live Log only, full height */}
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '12px 16px', borderBottom: '1px solid var(--border)', backgroundColor: 'var(--bg-secondary)', flexShrink: 0 }}>
          <Activity size={16} color="var(--accent)" />
          <h2 style={{ fontSize: '14px', fontWeight: 600 }}>Live Log</h2>
          {activeJobId && allJobs.find(j => j.id === activeJobId) && (
            <span style={{ fontSize: 11, color: 'var(--accent)', fontFamily: 'var(--font-mono)' }}>
              · {allJobs.find(j => j.id === activeJobId)?.project_name || activeJobId.slice(0, 8)}
            </span>
          )}
          <span style={{ fontSize: '11px', color: 'var(--text-muted)', marginLeft: 'auto', fontFamily: 'var(--font-mono)' }}>
            {localLogs.length} entries
          </span>
        </div>
        <div style={{ flex: 1, overflowY: 'auto', padding: '12px 16px', fontFamily: 'var(--font-mono)', fontSize: 12, color: 'var(--text-secondary)', lineHeight: 1.6 }}>
          {localLogs.length === 0 ? (
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%', color: 'var(--text-muted)' }}>
              No logs yet. Start a comparison to see real-time output.
            </div>
          ) : (
            localLogs.map((line, i) => (
              <div key={i} style={{ marginBottom: '2px' }}>
                <span style={{ color: 'var(--text-muted)' }}>{String(i + 1).padStart(4, ' ')}</span>{' '}
                {line}
              </div>
            ))
          )}
          <div ref={logEndRef} />
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
    <div className="card" style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '10px' }}>
      <div style={{ color: active ? 'var(--accent)' : 'var(--text-muted)', display: 'flex', alignItems: 'center' }}>{icon}</div>
      <div>
        <div style={{ fontSize: '16px', fontWeight: 700, fontFamily: 'var(--font-mono)' }}>{value}</div>
        <div style={{ fontSize: '10px', color: 'var(--text-muted)' }}>{label}</div>
      </div>
    </div>
  );
}

function ProgressCard({ title, icon, progress, label, active }: { title: string; icon: React.ReactNode; progress: number; label: string; active: boolean }) {
  return (
    <div className="card">
      <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '8px' }}>
        {active && <Loader2 size={12} className="spin" color="var(--accent)" />}
        {icon}
        <h3 style={{ fontSize: 12, fontWeight: 600 }}>{title}</h3>
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
    <div className="card" style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '10px' }}>
      <div style={{ color, display: 'flex', alignItems: 'center' }}>{icon}</div>
      <div>
        <div style={{ fontSize: '18px', fontWeight: 700 }}>{value}</div>
        <div style={{ fontSize: '10px', color: 'var(--text-muted)' }}>{label}</div>
      </div>
    </div>
  );
}