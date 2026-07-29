import React from 'react';
import { NavLink, Outlet } from 'react-router-dom';
import { LayoutDashboard, GitCompareArrows, FolderTree, Settings, Wrench, Loader2, Square } from 'lucide-react';
import { useStore } from '../store';
import { api } from '../api';

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
  const { compareStatus, fetchCompareStatus } = useStore();
  const [collapsed, setCollapsed] = React.useState(false);

  // Poll every 2 seconds for status — only when running
  React.useEffect(() => {
    // Always do an initial check
    fetchCompareStatus();

    const interval = setInterval(() => {
      // Poll when comparison is running OR if we don't know the state yet
      const st = useStore.getState().compareStatus;
      if (st?.running || st === null) {
        fetchCompareStatus();
      }
    }, 2000);

    return () => clearInterval(interval);
  }, [fetchCompareStatus]);

  if (!compareStatus?.running) return null;

  const progress = compareStatus.progress || 0;
  const processed = compareStatus.processed_files || 0;
  const total = compareStatus.total_files || 0;
  const elapsed = compareStatus.elapsed_time || '00:00:00';
  const eta = compareStatus.eta || '--:--:--';
  const matched = compareStatus.matched || 0;
  const different = compareStatus.different || 0;

  return (
    <div
      style={{
        flexShrink: 0,
        backgroundColor: 'var(--bg-elevated)',
        borderTop: '1px solid var(--border)',
        padding: collapsed ? '4px 12px' : '8px 12px',
        display: 'flex',
        alignItems: 'center',
        gap: '12px',
        fontSize: '11px',
        fontFamily: 'var(--font-mono)',
      }}
    >
      <Loader2 size={12} className="spin" color="var(--accent)" />
      <span style={{ color: 'var(--accent)', fontWeight: 600 }}>
        Comparing
      </span>
      <span style={{ color: 'var(--text-secondary)' }}>
        {processed} / {total} ({progress.toFixed(1)}%)
      </span>
      <div style={{ flex: 1, maxWidth: 200, height: 6, backgroundColor: 'var(--bg-secondary)', borderRadius: 3, overflow: 'hidden' }}>
        <div style={{ width: `${Math.min(100, progress)}%`, height: '100%', backgroundColor: 'var(--accent)', transition: 'width 0.3s ease' }} />
      </div>
      <span style={{ color: 'var(--text-muted)' }}>
        Elapsed: {elapsed}
      </span>
      <span style={{ color: 'var(--text-muted)' }}>
        ETA: {eta}
      </span>
      <span style={{ color: 'var(--success)' }}>
        ✓ {matched}
      </span>
      <span style={{ color: 'var(--warning)' }}>
        ≠ {different}
      </span>
      <button
        className="btn btn-danger"
        style={{ padding: '2px 8px', fontSize: 10, marginLeft: 'auto' }}
        onClick={async () => {
          try { await api.stopCompare(); } catch { /* ignore */ }
          fetchCompareStatus();
        }}
      >
        <Square size={10} />
        Stop
      </button>
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
