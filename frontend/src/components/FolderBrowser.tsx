import { useState, useEffect, useCallback } from 'react';
import { Folder, HardDrive, ChevronUp, X, Check } from 'lucide-react';
import { api, type FolderEntry } from '../api';

export default function FolderBrowser({
  initialPath,
  onSelect,
  onClose,
}: {
  initialPath: string;
  onSelect: (path: string) => void;
  onClose: () => void;
}) {
  const [currentPath, setCurrentPath] = useState(initialPath || '');
  const [entries, setEntries] = useState<FolderEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [manualPath, setManualPath] = useState(initialPath || '');

  const loadFolder = useCallback(async (path: string) => {
    setLoading(true);
    try {
      const resp = await api.browseSystem(path || undefined);
      setEntries(resp.entries || []);
      setCurrentPath(resp.current || path);
      setManualPath(resp.current || path);
    } catch {
      setEntries([]);
    }
    setLoading(false);
  }, []);

  useEffect(() => {
    loadFolder(initialPath);
  }, [initialPath, loadFolder]);

  const handleEntryClick = (entry: FolderEntry) => {
    loadFolder(entry.path);
  };

  const handleManualGo = () => {
    if (manualPath) {
      loadFolder(manualPath);
    }
  };

  const handleSelect = () => {
    onSelect(currentPath);
    onClose();
  };

  return (
    <div
      style={{
        position: 'fixed',
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        background: 'rgba(0,0,0,0.6)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 1000,
      }}
      onClick={onClose}
    >
      <div
        style={{
          background: 'var(--bg-secondary)',
          border: '1px solid var(--border)',
          borderRadius: 12,
          width: '600px',
          maxHeight: '70vh',
          display: 'flex',
          flexDirection: 'column',
          boxShadow: '0 8px 32px rgba(0,0,0,0.3)',
        }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div style={{ padding: '16px', borderBottom: '1px solid var(--border)', display: 'flex', alignItems: 'center', gap: '8px' }}>
          <Folder size={18} color="var(--accent)" />
          <span style={{ fontSize: '15px', fontWeight: 600 }}>Browse for Folder</span>
          <button className="btn btn-ghost" style={{ marginLeft: 'auto', padding: '4px 8px' }} onClick={onClose}>
            <X size={16} />
          </button>
        </div>

        {/* Manual path input */}
        <div style={{ padding: '12px 16px', borderBottom: '1px solid var(--border)', display: 'flex', gap: '8px' }}>
          <input
            type="text"
            value={manualPath}
            onChange={(e) => setManualPath(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleManualGo()}
            placeholder="Enter path or browse below..."
            style={{
              flex: 1,
              background: 'var(--bg-elevated)',
              border: '1px solid var(--border-light)',
              borderRadius: 6,
              padding: '6px 10px',
              color: 'var(--text-primary)',
              fontFamily: 'var(--font-mono)',
              fontSize: 12,
            }}
          />
          <button className="btn btn-secondary" style={{ padding: '6px 12px', fontSize: 12 }} onClick={handleManualGo}>
            Go
          </button>
        </div>

        {/* Current path */}
        <div style={{ padding: '8px 16px', fontSize: '11px', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)', borderBottom: '1px solid var(--border)' }}>
          {currentPath || '(root)'}
        </div>

        {/* Folder list */}
        <div style={{ flex: 1, overflowY: 'auto', padding: '8px' }}>
          {loading ? (
            <div style={{ textAlign: 'center', padding: '24px', color: 'var(--text-muted)' }}>Loading...</div>
          ) : entries.length === 0 ? (
            <div style={{ textAlign: 'center', padding: '24px', color: 'var(--text-muted)', fontSize: 13 }}>
              No subfolders found. You can type a path manually above.
            </div>
          ) : (
            entries.map((entry, i) => (
              <div
                key={i}
                onClick={() => handleEntryClick(entry)}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: '8px',
                  padding: '6px 12px',
                  cursor: 'pointer',
                  fontSize: 13,
                  borderRadius: 4,
                  color: 'var(--text-primary)',
                }}
                onMouseEnter={(e) => (e.currentTarget.style.background = 'rgba(0,138,0,0.06)')}
                onMouseLeave={(e) => (e.currentTarget.style.background = 'transparent')}
              >
                {entry.type === 'parent' ? (
                  <ChevronUp size={14} color="var(--text-muted)" />
                ) : entry.type === 'drive' ? (
                  <HardDrive size={14} color="var(--accent)" />
                ) : (
                  <Folder size={14} color="var(--accent)" />
                )}
                <span>{entry.name}</span>
                {entry.type === 'parent' && <span style={{ color: 'var(--text-muted)', fontSize: 11 }}>Go up</span>}
              </div>
            ))
          )}
        </div>

        {/* Footer */}
        <div style={{ padding: '12px 16px', borderTop: '1px solid var(--border)', display: 'flex', justifyContent: 'flex-end', gap: '8px' }}>
          <button className="btn btn-ghost" onClick={onClose}>Cancel</button>
          <button className="btn btn-primary" onClick={handleSelect}>
            <Check size={14} />
            Select: {currentPath ? currentPath.split(/[/\\]/).pop() : 'root'}
          </button>
        </div>
      </div>
    </div>
  );
}