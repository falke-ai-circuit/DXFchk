import React from 'react';
import { NavLink, Outlet } from 'react-router-dom';
import { LayoutDashboard, GitCompareArrows, FolderTree, Settings, Wrench } from 'lucide-react';

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
          {/* Valmet-style logo: green circle with V */}
          <div style={{
            width: 24,
            height: 24,
            borderRadius: '50%',
            backgroundColor: 'var(--accent)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            fontSize: '14px',
            fontWeight: 900,
            color: '#fff',
            fontFamily: 'var(--font-sans)',
          }}>
            V
          </div>
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
