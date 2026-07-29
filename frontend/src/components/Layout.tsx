import React from 'react';
import { NavLink, Outlet } from 'react-router-dom';
import { LayoutDashboard, GitCompareArrows, FolderTree, Settings, Wrench, Loader2, Square } from 'lucide-react';

export default function Layout() {
  const tabStyle = (isActive: boolean): React.CSSProperties => ({
    display: 'flex',
    alignItems: 'center',
    gap: '6px',
    padding: '10px 16px',
    fontSize: '13px',
    fontWeight: isActive ? 700 : 500,
    color: isActive ? 'var(--accent)' : 'var(--text-secondary)',
    backgroundColor: isActive ? 'rgba(0,138,0,0.08)' : 'transparent',
    textDecoration: 'none',
    borderBottom: isActive ? '2px solid var(--accent)' : '2px solid transparent',
    transition: 'all 0.15s ease',
    cursor: 'pointer',
  });

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100vh' }}>
      <nav
        style={{
          display: 'flex',
          alignItems: 'center',
          backgroundColor: 'var(--bg-secondary)',
          borderBottom: '1px solid var(--border)',
          padding: '0 12px',
          height: '44px',
          flexShrink: 0,
        }}
      >
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: '8px',
            padding: '0 12px',
            marginRight: '8px',
          }}
        >
          <img src="/valmet-logo.png" alt="Valmet" style={{ height: 20, width: 'auto' }} />
          <span style={{ fontSize: '14px', fontWeight: 700, color: 'var(--accent)', fontFamily: 'var(--font-mono)' }}>
            DXFchk
          </span>
        </div>

        <NavLink to="/" end style={({ isActive }) => tabStyle(isActive)}>
          <LayoutDashboard size={14} />
          Dashboard
        </NavLink>

        <NavLink to="/compare" style={({ isActive }) => tabStyle(isActive)}>
          <GitCompareArrows size={14} />
          Compare
        </NavLink>

        <NavLink to="/browse" style={({ isActive }) => tabStyle(isActive)}>
          <FolderTree size={14} />
          Browse
        </NavLink>

        <NavLink to="/edit" style={({ isActive }) => tabStyle(isActive)}>
          <Wrench size={14} />
          Edit
        </NavLink>

        <NavLink to="/settings" style={({ isActive }) => tabStyle(isActive)}>
          <Settings size={14} />
          Settings
        </NavLink>

        <div style={{ marginLeft: 'auto', paddingRight: '12px' }}>
          <HealthIndicator />
        </div>
      </nav>

      <main
        style={{
          flex: 1,
          overflowY: 'auto',
          overflowX: 'hidden',
          backgroundColor: 'var(--bg-primary)',
        }}
      >
        <Outlet />
      </main>

      {/* Global comparison status bar — visible on all pages when running */}
      <GlobalStatusBar />
    </div>
  );
}

function GlobalStatusBar() {
  const [jobs, setJobs] = React.useState<any[]>([]); // running jobs from /api/v1/compare/jobs

  // Poll every 2 seconds for all running jobs
  React.useEffect(() => {
    const fetchJobs = async () => {
      try {
        const res = await fetch('/api/v1/compare/jobs');
        if (res.ok) {
          const data = await res.json();
          setJobs(data.jobs || []);
        }
      } catch { /* ignore */ }
    };
    fetchJobs();
    const interval = setInterval(fetchJobs, 2000);
    return () => clearInterval(interval);
  }, []);

  if (jobs.length === 0) return null;

  const runningJobs = jobs.filter(j => j.running);
  if (runningJobs.length === 0) return null;

  return (
    <div
      style={{
        flexShrink: 0,
        backgroundColor: 'var(--bg-elevated)',
        borderTop: '1px solid var(--border)',
        padding: '4px 12px',
        display: 'flex',
        alignItems: 'center',
        gap: '8px',
        fontSize: '11px',
        fontFamily: 'var(--font-mono)',
        maxWidth: '100%',
        overflowX: 'auto',
      }}
    >
      <Loader2 size={12} className="spin" color="var(--accent)" />
      <span style={{ color: 'var(--accent)', fontWeight: 600, flexShrink: 0 }}>
        {runningJobs.length > 1 ? `${runningJobs.length} jobs` : 'Comparing'}
      </span>
      {runningJobs.map(job => (
        <span key={job.id} style={{ display: 'flex', alignItems: 'center', gap: '4px', flexShrink: 0 }}>
          {runningJobs.length > 1 && job.project_name && (
            <span style={{ color: 'var(--text-muted)', fontSize: 10 }}>{job.project_name}:</span>
          )}
          <span style={{ color: 'var(--text-secondary)' }}>
            {job.processed_files}/{job.total_files} ({job.progress?.toFixed(1)}%)
          </span>
          <span style={{ color: 'var(--text-muted)', fontSize: 10 }}>ETA {job.eta}</span>
          <button
            className="btn btn-danger"
            style={{ padding: '1px 6px', fontSize: 9 }}
            onClick={async () => {
              try {
                await fetch('/api/v1/compare/stop', {
                  method: 'POST',
                  headers: { 'Content-Type': 'application/json' },
                  body: JSON.stringify({ project_id: job.id }),
                });
              } catch { /* ignore */ }
            }}
          >
            <Square size={8} />
          </button>
        </span>
      ))}
    </div>
  );
}

function HealthIndicator() {
  const [status, setStatus] = React.useState<'ok' | 'err' | 'loading'>('loading');

  React.useEffect(() => {
    let mounted = true;
    const check = async () => {
      try {
        const res = await fetch('/api/v1/health');
        if (res.ok) {
          const data = await res.json();
          if (mounted) setStatus(data.status === 'ok' ? 'ok' : 'err');
        } else {
          if (mounted) setStatus('err');
        }
      } catch {
        if (mounted) setStatus('err');
      }
    };
    check();
    const interval = setInterval(check, 5000);
    return () => { mounted = false; clearInterval(interval); };
  }, []);

  return (
    <span
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: '6px',
        fontSize: '11px',
        color: 'var(--text-muted)',
        fontFamily: 'var(--font-mono)',
      }}
    >
      <span
        style={{
          width: 8,
          height: 8,
          borderRadius: '50%',
          background: status === 'ok' ? 'var(--success)' : status === 'err' ? 'var(--error)' : 'var(--text-muted)',
        }}
      />
      {status === 'ok' ? 'Online' : status === 'err' ? 'Offline' : '...'}
    </span>
  );
}
