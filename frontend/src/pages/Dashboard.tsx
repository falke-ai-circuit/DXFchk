import { useEffect, useRef } from 'react';
import {
  FileCheck, FileDiff, FileQuestion, Files,
  Activity, Loader2,
} from 'lucide-react';
import { useStore } from '../store';

export default function Dashboard() {
  const {
    health, fetchHealth,
    compareStatus, fetchCompareStatus,
    results, fetchResults,
    templateCount, fetchTemplates,
    logs,
  } = useStore();

  const pollingRef = useRef(false);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  useEffect(() => {
    fetchHealth();
    fetchCompareStatus();
    fetchResults();
    fetchTemplates();
  }, [fetchHealth, fetchCompareStatus, fetchResults, fetchTemplates]);

  // Poll compare status when running
  useEffect(() => {
    if (compareStatus?.running && !pollingRef.current) {
      pollingRef.current = true;
      intervalRef.current = setInterval(() => {
        fetchCompareStatus();
        fetchResults();
      }, 1000);
    } else if (!compareStatus?.running && pollingRef.current) {
      pollingRef.current = false;
      if (intervalRef.current) {
        clearInterval(intervalRef.current);
        intervalRef.current = null;
      }
    }
  }, [compareStatus?.running, fetchCompareStatus, fetchResults]);

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
    };
  }, []);

  const matched = results.filter(r => r.status === 'match').length;
  const different = results.filter(r => r.status === 'different').length;
  const noTemplate = results.filter(r => r.status === 'no_template').length;
  const total = results.length;

  const templateProgress = compareStatus ? (compareStatus.total_files > 0 ? (compareStatus.processed_files / compareStatus.total_files) * 100 : 0) : 0;

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

      {/* Stats Cards */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: '16px', marginBottom: '24px' }}>
        <StatCard icon={<Files size={20} />} label="Total Files" value={total} color="var(--accent)" />
        <StatCard icon={<FileCheck size={20} />} label="Matched" value={matched} color="var(--success)" />
        <StatCard icon={<FileDiff size={20} />} label="Different" value={different} color="var(--warning)" />
        <StatCard icon={<FileQuestion size={20} />} label="No Template" value={noTemplate} color="var(--text-muted)" />
      </div>

      {/* Progress Section */}
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px', marginBottom: '24px' }}>
        <ProgressCard
          title="Template Scanning"
          progress={compareStatus?.running ? templateProgress : 100}
          label={compareStatus?.running ? `${compareStatus.processed_files} / ${compareStatus.total_files} files` : `${templateCount} templates loaded`}
          active={compareStatus?.running ?? false}
        />
        <ProgressCard
          title="Comparison Progress"
          progress={compareStatus?.progress ?? 0}
          label={compareStatus?.running ? `${compareStatus.processed_files} / ${compareStatus.total_files} (${(compareStatus.progress ?? 0).toFixed(1)}%)` : 'Idle'}
          active={compareStatus?.running ?? false}
        />
      </div>

      {/* Log Output */}
      <div className="card" style={{ padding: 0, overflow: 'hidden' }}>
        <div style={{
          display: 'flex',
          alignItems: 'center',
          gap: '8px',
          padding: '12px 16px',
          borderBottom: '1px solid var(--border)',
          backgroundColor: 'var(--bg-secondary)',
        }}>
          <Activity size={16} color="var(--accent)" />
          <h2 style={{ fontSize: '14px', fontWeight: 600 }}>Log Output</h2>
          <span style={{ fontSize: '11px', color: 'var(--text-muted)', marginLeft: 'auto', fontFamily: 'var(--font-mono)' }}>
            {logs.length} entries
          </span>
        </div>
        <div style={{
          padding: '12px 16px',
          maxHeight: '300px',
          overflowY: 'auto',
          fontFamily: 'var(--font-mono)',
          fontSize: '12px',
          color: 'var(--text-secondary)',
          lineHeight: 1.6,
        }}>
          {logs.length === 0 ? (
            <span style={{ color: 'var(--text-muted)' }}>No logs yet. Start a comparison to see output.</span>
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

function StatCard({ icon, label, value, color }: { icon: React.ReactNode; label: string; value: number | string; color: string }) {
  return (
    <div className="card" style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
      <div style={{
        width: 44, height: 44, borderRadius: 8,
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        background: `${color}15`, color,
      }}>
        {icon}
      </div>
      <div>
        <div style={{ fontSize: '22px', fontWeight: 700, color: 'var(--text-primary)' }}>{value}</div>
        <div style={{ fontSize: '12px', color: 'var(--text-muted)' }}>{label}</div>
      </div>
    </div>
  );
}

function ProgressCard({ title, progress, label, active }: { title: string; progress: number; label: string; active: boolean }) {
  return (
    <div className="card">
      <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '12px' }}>
        {active && <Loader2 size={14} className="spin" color="var(--accent)" />}
        <h3 style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text-primary)' }}>{title}</h3>
        <span style={{ fontSize: '11px', color: 'var(--text-muted)', marginLeft: 'auto', fontFamily: 'var(--font-mono)' }}>{label}</span>
      </div>
      <div className="progress-track">
        <div className="progress-fill" style={{ width: `${Math.min(100, progress)}%` }} />
      </div>
    </div>
  );
}