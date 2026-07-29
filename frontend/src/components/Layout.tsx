import React from 'react';
import { NavLink, Outlet } from 'react-router-dom';
import { LayoutDashboard, GitCompareArrows, FolderTree, Settings, Loader2, Square } from 'lucide-react';
import { useStore } from '../store';

export default function Layout() {
  const fetchProjects = useStore(s => s.fetchProjects);

  // Fetch projects on app load so activeProject is populated on all pages
  React.useEffect(() => {
    fetchProjects();
  }, [fetchProjects]);

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
  const [jobs, setJobs] = React.useState<any[]>([]);
  const [health, setHealth] = React.useState<any>(null);

  // Poll every 2 seconds for jobs + health
  React.useEffect(() => {
    const fetchAll = async () => {
      try {
        const [jobsRes, healthRes] = await Promise.all([
          fetch('/api/v1/compare/jobs'),
          fetch('/api/v1/health'),
        ]);
        if (jobsRes.ok) {
          const data = await jobsRes.json();
          setJobs(data.jobs || []);
        }
        if (healthRes.ok) {
          setHealth(await healthRes.json());
        }
      } catch { /* ignore */ }
    };
    fetchAll();
    const interval = setInterval(fetchAll, 2000);
    return () => clearInterval(interval);
  }, []);

  const runningJobs = jobs.filter(j => j.running);
  const completedJobs = jobs.filter(j => !j.running && j.processed_files > 0);
  const hasContent = runningJobs.length > 0 || completedJobs.length > 0;

  if (!hasContent && !health?.running) {
    // Show idle status bar with health info
    return (
      <div style={{
        flexShrink: 0, backgroundColor: 'var(--bg-elevated)',
        borderTop: '1px solid var(--border)', padding: '4px 12px',
        display: 'flex', alignItems: 'center', gap: '8px',
        fontSize: '11px', fontFamily: 'var(--font-mono)',
      }}>
        <span style={{ color: 'var(--text-muted)' }}>
          {health?.running ? `Running: ${health.running_jobs} job(s)` : 'Idle'}
        </span>
        {completedJobs.length > 0 && (
          <span style={{ color: 'var(--text-muted)', marginLeft: 'auto' }}>
            Last: {completedJobs[0].project_name} — {completedJobs[0].matched} match, {completedJobs[0].different} diff, {completedJobs[0].no_template} no-tmpl
          </span>
        )}
      </div>
    );
  }

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
      {runningJobs.length > 0 ? (
        <>
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
              <span style={{ color: 'var(--text-muted)', fontSize: 10 }}>{job.elapsed_time} / ETA {job.eta}</span>
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
        </>
      ) : (
        // Completed jobs
        <>
          <span style={{ color: 'var(--text-muted)', fontWeight: 600 }}>Done</span>
          {completedJobs.map(job => (
            <span key={job.id} style={{ color: 'var(--text-muted)', flexShrink: 0 }}>
              {job.project_name}: {job.matched} match, {job.different} diff, {job.no_template} no-tmpl ({job.elapsed_time})
            </span>
          ))}
        </>
      )}
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
