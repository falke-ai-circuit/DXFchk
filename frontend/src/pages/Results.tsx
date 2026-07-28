import { useState, useEffect } from 'react';
import { ListCheck, Filter, FileCheck, FileDiff, FileQuestion, RefreshCw } from 'lucide-react';
import { useStore } from '../store';

type StatusFilter = 'all' | 'match' | 'different' | 'no_template';

export default function Results() {
  const { results, resultsLoading, fetchResults, compareStatus, fetchCompareStatus } = useStore();
  const [filter, setFilter] = useState<StatusFilter>('all');

  useEffect(() => {
    fetchResults();
    fetchCompareStatus();
  }, [fetchResults, fetchCompareStatus]);

  // Auto-refresh if comparison is running
  useEffect(() => {
    if (!compareStatus?.running) return;
    const interval = setInterval(() => {
      fetchResults();
    }, 2000);
    return () => clearInterval(interval);
  }, [compareStatus?.running, fetchResults]);

  const filtered = filter === 'all' ? results : results.filter(r => r.status === filter);

  const counts = {
    total: results.length,
    match: results.filter(r => r.status === 'match').length,
    different: results.filter(r => r.status === 'different').length,
    no_template: results.filter(r => r.status === 'no_template').length,
  };

  return (
    <div style={{ padding: '24px', maxWidth: '1200px', margin: '0 auto' }}>
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '12px', marginBottom: '24px' }}>
        <h1 style={{ fontSize: '24px', fontWeight: 700, display: 'flex', alignItems: 'center', gap: '8px' }}>
          <ListCheck size={24} color="var(--accent)" />
          Results
        </h1>
        <button
          className="btn btn-ghost"
          style={{ marginLeft: 'auto', fontSize: '12px', padding: '6px 12px' }}
          onClick={() => fetchResults()}
        >
          <RefreshCw size={14} />
          Refresh
        </button>
      </div>

      {/* Filter Bar */}
      <div style={{ display: 'flex', gap: '8px', marginBottom: '20px', alignItems: 'center' }}>
        <Filter size={16} color="var(--text-muted)" />
        <FilterButton label="All" count={counts.total} active={filter === 'all'} onClick={() => setFilter('all')} />
        <FilterButton label="Matched" count={counts.match} active={filter === 'match'} onClick={() => setFilter('match')} icon={<FileCheck size={14} />} color="var(--success)" />
        <FilterButton label="Different" count={counts.different} active={filter === 'different'} onClick={() => setFilter('different')} icon={<FileDiff size={14} />} color="var(--warning)" />
        <FilterButton label="No Template" count={counts.no_template} active={filter === 'no_template'} onClick={() => setFilter('no_template')} icon={<FileQuestion size={14} />} color="var(--text-muted)" />
      </div>

      {/* Results Table */}
      <div className="card" style={{ padding: 0, overflow: 'hidden' }}>
        {resultsLoading && results.length === 0 ? (
          <div style={{ padding: '48px', textAlign: 'center', color: 'var(--text-muted)' }}>
            Loading results...
          </div>
        ) : filtered.length === 0 ? (
          <div style={{ padding: '48px', textAlign: 'center', color: 'var(--text-muted)' }}>
            No results found. Run a comparison to see results here.
          </div>
        ) : (
          <table>
            <thead>
              <tr>
                <th style={{ width: '40px' }}>#</th>
                <th>File</th>
                <th>Status</th>
                <th>Template</th>
                <th>Mod Folder</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((r, i) => (
                <tr key={i}>
                  <td style={{ color: 'var(--text-muted)', fontFamily: 'var(--font-mono)', fontSize: 12 }}>{i + 1}</td>
                  <td style={{ fontFamily: 'var(--font-mono)', fontSize: 12 }}>{r.file_name || '-'}</td>
                  <td><StatusBadge status={r.status} /></td>
                  <td style={{ fontFamily: 'var(--font-mono)', fontSize: 12, color: 'var(--text-secondary)' }}>{r.template || '-'}</td>
                  <td style={{ fontFamily: 'var(--font-mono)', fontSize: 12, color: 'var(--text-secondary)' }}>{r.mod_folder || '-'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}

function FilterButton({ label, count, active, onClick, icon, color }: { label: string; count: number; active: boolean; onClick: () => void; icon?: React.ReactNode; color?: string }) {
  return (
    <button
      onClick={onClick}
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: '6px',
        padding: '6px 12px',
        borderRadius: 6,
        fontSize: 13,
        fontWeight: active ? 600 : 500,
        cursor: 'pointer',
        border: `1px solid ${active ? (color || 'var(--accent)') : 'var(--border)'}`,
        background: active ? `${(color || 'var(--accent)')}15` : 'transparent',
        color: active ? (color || 'var(--accent)') : 'var(--text-secondary)',
        transition: 'all 0.15s ease',
      }}
    >
      {icon}
      {label}
      <span style={{ fontSize: 11, opacity: 0.7 }}>({count})</span>
    </button>
  );
}

function StatusBadge({ status }: { status: string }) {
  const normalized = status.toLowerCase().replace(/\s+/g, '_');
  let className = 'badge badge-muted';
  let label = status;

  if (normalized === 'match') {
    className = 'badge badge-success';
    label = 'Match';
  } else if (normalized === 'different' || normalized === 'diff') {
    className = 'badge badge-warning';
    label = 'Different';
  } else if (normalized === 'no_template' || normalized === 'notemplate') {
    className = 'badge badge-muted';
    label = 'No Template';
  } else if (normalized === 'error') {
    className = 'badge badge-error';
    label = 'Error';
  }

  return <span className={className}>{label}</span>;
}