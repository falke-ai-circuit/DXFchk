import { Routes, Route } from 'react-router-dom';
import Layout from './components/Layout';
import Dashboard from './pages/Dashboard';
import Compare from './pages/Compare';
import Results from './pages/Results';
import Settings from './pages/Settings';

function NotFound() {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: '100%', padding: '48px', textAlign: 'center' }}>
      <h1 style={{ fontSize: '64px', fontWeight: 700, color: 'var(--accent)', fontFamily: 'var(--font-mono)', marginBottom: '8px' }}>404</h1>
      <p style={{ color: 'var(--text-secondary)', fontSize: '16px', marginBottom: '24px' }}>The page you're looking for doesn't exist.</p>
      <a href="/" className="btn btn-primary">Back to Dashboard</a>
    </div>
  );
}

export default function App() {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100vh', backgroundColor: 'var(--bg-primary)', color: 'var(--text-primary)' }}>
      <div style={{ flex: 1, overflow: 'hidden' }}>
        <Routes>
          <Route element={<Layout />}>
            <Route path="/" element={<Dashboard />} />
            <Route path="/compare" element={<Compare />} />
            <Route path="/results" element={<Results />} />
            <Route path="/settings" element={<Settings />} />
            <Route path="*" element={<NotFound />} />
          </Route>
        </Routes>
      </div>
    </div>
  );
}