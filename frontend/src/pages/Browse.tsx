import { useState, useEffect, useRef, useCallback } from 'react';
import {
  FolderTree, Folder, FileText, ChevronRight, ChevronDown,
  RefreshCw, Loader2, GitCompare, Plus, AlertCircle,
} from 'lucide-react';
import { useStore } from '../store';
import { api, type TreeNode, type DiffResponse, type DiffEntity } from '../api';

export default function Browse() {
  const { activeProject } = useStore();
  const [tree, setTree] = useState<TreeNode | null>(null);
  const [loading, setLoading] = useState(false);
  const [empty, setEmpty] = useState(false);
  const [selectedFile, setSelectedFile] = useState<string | null>(null);
  const [diff, setDiff] = useState<DiffResponse | null>(null);
  const [diffLoading, setDiffLoading] = useState(false);
  const [templatePath, setTemplatePath] = useState<string | null>(null);
  const [createTemplateMsg, setCreateTemplateMsg] = useState<string | null>(null);

  const outputFolder = activeProject?.output_folder || '';

  const loadTree = useCallback(async () => {
    if (!outputFolder) return;
    setLoading(true);
    try {
      const resp = await api.browse(outputFolder);
      setTree(resp.tree);
      setEmpty(resp.empty);
    } catch {
      setTree(null);
      setEmpty(true);
    }
    setLoading(false);
  }, [outputFolder]);

  useEffect(() => {
    loadTree();
  }, [loadTree]);

  // Find template file for a given module file
  const findTemplateForFile = async (filePath: string) => {
    if (!activeProject?.template_folder) return null;

    const fileName = filePath.split('\\').pop() || '';

    try {
      const resp = await api.browseFolder(activeProject.template_folder);
      const found = resp.children?.find(c => c.name === fileName);
      if (found) return found.path;
    } catch { /* ignore */ }

    return null;
  };

  const handleFileClick = async (filePath: string, _folderPath: string) => {
    setSelectedFile(filePath);
    setDiff(null);
    setCreateTemplateMsg(null);

    // Try to find corresponding template
    const tmpl = await findTemplateForFile(filePath);
    setTemplatePath(tmpl);

    if (tmpl) {
      setDiffLoading(true);
      try {
        const d = await api.diff(tmpl, filePath);
        setDiff(d);
      } catch (err) {
        // Diff failed — maybe files too different or parse error
        console.error('Diff failed:', err);
      }
      setDiffLoading(false);
    }
  };

  const handleCreateTemplate = async () => {
    if (!selectedFile) return;
    setCreateTemplateMsg('Creating template...');
    try {
      const resp = await api.createTemplate(selectedFile, activeProject?.template_folder);
      setCreateTemplateMsg(`✓ Template created: ${resp.name}`);
    } catch (err) {
      setCreateTemplateMsg(`✗ Failed: ${err instanceof Error ? err.message : 'unknown error'}`);
    }
  };

  if (!activeProject) {
    return (
      <div style={{ padding: '24px', textAlign: 'center' }}>
        <FolderTree size={32} color="var(--text-muted)" style={{ marginBottom: '8px' }} />
        <p style={{ color: 'var(--text-muted)' }}>Select a project on the Dashboard first.</p>
      </div>
    );
  }

  if (loading) {
    return (
      <div style={{ padding: '24px', textAlign: 'center' }}>
        <Loader2 size={24} className="spin" color="var(--accent)" />
        <p style={{ color: 'var(--text-muted)', marginTop: '8px' }}>Loading output folder...</p>
      </div>
    );
  }

  if (empty || !tree) {
    return (
      <div style={{ padding: '24px', textAlign: 'center' }}>
        <p style={{ color: 'var(--text-muted)' }}>Output folder is empty. Run a comparison first.</p>
        <button className="btn btn-primary" style={{ marginTop: '12px' }} onClick={loadTree}>
          <RefreshCw size={14} />
          Refresh
        </button>
      </div>
    );
  }

  return (
    <div style={{ display: 'flex', height: '100%', overflow: 'hidden' }}>
      {/* Left Panel — Tree View */}
      <div style={{ width: '380px', flexShrink: 0, borderRight: '1px solid var(--border)', display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
        <div style={{ padding: '12px 16px', borderBottom: '1px solid var(--border)', backgroundColor: 'var(--bg-secondary)', display: 'flex', alignItems: 'center', gap: '8px' }}>
          <FolderTree size={16} color="var(--accent)" />
          <span style={{ fontSize: '13px', fontWeight: 600 }}>Output Structure</span>
          <button className="btn btn-ghost" style={{ marginLeft: 'auto', padding: '4px 8px' }} onClick={loadTree}>
            <RefreshCw size={14} />
          </button>
        </div>
        <div style={{ flex: 1, overflowY: 'auto', padding: '8px' }}>
          <TreeView node={tree} level={0} selectedFile={selectedFile} onFileClick={handleFileClick} />
        </div>
      </div>

      {/* Right Panel — DXF Visual Diff */}
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
        {selectedFile ? (
          <>
            <div style={{ padding: '12px 16px', borderBottom: '1px solid var(--border)', backgroundColor: 'var(--bg-secondary)', display: 'flex', alignItems: 'center', gap: '8px' }}>
              <FileText size={16} color="var(--accent)" />
              <span style={{ fontSize: '13px', fontWeight: 600, fontFamily: 'var(--font-mono)' }}>
                {selectedFile.split('\\').pop()}
              </span>
              {templatePath && (
                <span style={{ fontSize: '11px', color: 'var(--text-muted)', marginLeft: '8px' }}>
                  vs {templatePath.split('\\').pop()}
                </span>
              )}
              {diff && diff.summary.added_count > 0 && (
                <button className="btn btn-primary" style={{ marginLeft: 'auto', padding: '4px 12px', fontSize: 11 }} onClick={handleCreateTemplate}>
                  <Plus size={12} />
                  Create Template from this file
                </button>
              )}
            </div>

            {/* Diff Summary Bar */}
            {diff && (
              <div style={{ padding: '8px 16px', borderBottom: '1px solid var(--border)', display: 'flex', gap: '16px', fontSize: '12px' }}>
                <span style={{ color: 'var(--text-muted)' }}>Template: <strong style={{ color: 'var(--text-primary)' }}>{diff.summary.template_count}</strong> entities</span>
                <span style={{ color: 'var(--text-muted)' }}>Module: <strong style={{ color: 'var(--text-primary)' }}>{diff.summary.module_count}</strong> entities</span>
                <span style={{ color: 'var(--error)' }}>Added: <strong>{diff.summary.added_count}</strong></span>
                <span style={{ color: 'var(--warning)' }}>Removed: <strong>{diff.summary.removed_count}</strong></span>
              </div>
            )}

            {/* Create template message */}
            {createTemplateMsg && (
              <div style={{ padding: '8px 16px', background: 'rgba(0,138,0,0.08)', borderBottom: '1px solid var(--border)', fontSize: '12px', color: 'var(--accent)' }}>
                {createTemplateMsg}
              </div>
            )}

            {/* DXF Canvas View */}
            <div style={{ flex: 1, overflow: 'auto', display: 'flex', backgroundColor: 'var(--bg-primary)' }}>
              {diffLoading ? (
                <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                  <Loader2 size={24} className="spin" color="var(--accent)" />
                </div>
              ) : diff ? (
                <div style={{ display: 'flex', width: '100%', height: '100%' }}>
                  {/* Template View */}
                  <div style={{ flex: 1, display: 'flex', flexDirection: 'column', borderRight: '1px solid var(--border)' }}>
                    <div style={{ padding: '8px 12px', borderBottom: '1px solid var(--border)', fontSize: '12px', fontWeight: 600, color: 'var(--text-secondary)', backgroundColor: 'var(--bg-secondary)' }}>
                      Template
                    </div>
                    <DXFCanvas entities={diff.template_entities} added={[]} removed={diff.removed} bbox={diff.bounding_box} />
                  </div>
                  {/* Module View */}
                  <div style={{ flex: 1, display: 'flex', flexDirection: 'column' }}>
                    <div style={{ padding: '8px 12px', borderBottom: '1px solid var(--border)', fontSize: '12px', fontWeight: 600, color: 'var(--text-secondary)', backgroundColor: 'var(--bg-secondary)' }}>
                      Module (with differences highlighted)
                    </div>
                    <DXFCanvas entities={diff.module_entities} added={diff.added} removed={[]} bbox={diff.bounding_box} highlightDiffs={true} />
                  </div>
                </div>
              ) : templatePath === null ? (
                <div style={{ flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', padding: '24px' }}>
                  <AlertCircle size={32} color="var(--warning)" style={{ marginBottom: '8px' }} />
                  <p style={{ color: 'var(--text-secondary)', fontSize: '14px', textAlign: 'center', marginBottom: '12px' }}>
                    No matching template found for this file.
                  </p>
                  <p style={{ color: 'var(--text-muted)', fontSize: '12px', marginBottom: '16px' }}>
                    This file is in a _modN folder, meaning it differs from the template.
                    You can create a new template from this file.
                  </p>
                  <button className="btn btn-primary" onClick={handleCreateTemplate}>
                    <Plus size={14} />
                    Create Template from this file
                  </button>
                </div>
              ) : (
                <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                  <p style={{ color: 'var(--text-muted)' }}>No diff data available.</p>
                </div>
              )}
            </div>
          </>
        ) : (
          <div style={{ flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', padding: '24px' }}>
            <GitCompare size={32} color="var(--text-muted)" style={{ marginBottom: '8px' }} />
            <p style={{ color: 'var(--text-muted)', fontSize: '14px' }}>Select a DXF file from the tree to view the visual diff.</p>
            <p style={{ color: 'var(--text-muted)', fontSize: '12px', marginTop: '4px' }}>
              Template vs module comparison with highlighted differences.
            </p>
          </div>
        )}
      </div>
    </div>
  );
}

// TreeView renders a tree node recursively
function TreeView({ node, level, selectedFile, onFileClick }: {
  node: TreeNode;
  level: number;
  selectedFile: string | null;
  onFileClick: (filePath: string, folderPath: string) => void;
}) {
  const [expanded, setExpanded] = useState(level === 0);

  const isDXF = !node.is_dir && node.name.toLowerCase().endsWith('.dxf');
  const isLog = !node.is_dir && node.name.toLowerCase().endsWith('.log');

  if (!node.is_dir) {
    // File node
    return (
      <div
        onClick={() => isDXF && onFileClick(node.path, '')}
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: '6px',
          padding: '3px 8px',
          marginLeft: level * 16,
          cursor: isDXF ? 'pointer' : 'default',
          fontSize: 12,
          fontFamily: 'var(--font-mono)',
          color: selectedFile === node.path ? 'var(--accent)' : isDXF ? 'var(--text-primary)' : 'var(--text-muted)',
          background: selectedFile === node.path ? 'rgba(0,138,0,0.08)' : 'transparent',
          borderRadius: 4,
        }}
      >
        {isDXF ? <FileText size={12} color="var(--accent)" /> : <FileText size={12} color="var(--text-muted)" />}
        <span>{node.name}</span>
        {isDXF && <span style={{ fontSize: 10, color: 'var(--text-muted)' }}>📐</span>}
        {isLog && <span style={{ fontSize: 10, color: 'var(--text-muted)' }}>📝</span>}
        <span style={{ fontSize: 10, color: 'var(--text-muted)', marginLeft: 'auto' }}>{formatSize(node.size)}</span>
      </div>
    );
  }

  // Directory node
  return (
    <div>
      <div
        onClick={() => setExpanded(!expanded)}
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: '4px',
          padding: '3px 8px',
          marginLeft: level * 16,
          cursor: 'pointer',
          fontSize: 12,
          fontFamily: 'var(--font-mono)',
          color: node.is_mod ? 'var(--warning)' : 'var(--text-primary)',
          borderRadius: 4,
        }}
      >
        {expanded ? <ChevronDown size={12} color="var(--text-muted)" /> : <ChevronRight size={12} color="var(--text-muted)" />}
        <Folder size={14} color={node.is_mod ? 'var(--warning)' : 'var(--accent)'} />
        <span>{node.name}</span>
        {node.is_template && <span style={{ fontSize: 10, color: 'var(--text-muted)' }}> [✓]</span>}
        {node.is_mod && <span className="badge badge-warning" style={{ fontSize: 9, padding: '1px 4px' }}>mod</span>}
        {node.dxf_count > 0 && <span style={{ fontSize: 10, color: 'var(--text-muted)', marginLeft: 'auto' }}>{node.dxf_count} DXF</span>}
      </div>
      {expanded && node.children && (
        <div>
          {node.children.map((child, i) => (
            <TreeView key={i} node={child} level={level + 1} selectedFile={selectedFile} onFileClick={onFileClick} />
          ))}
        </div>
      )}
    </div>
  );
}

// DXFCanvas renders DXF entities on an SVG canvas with diff highlighting
function DXFCanvas({ entities, added, removed, bbox, highlightDiffs }: {
  entities: DiffEntity[];
  added: DiffEntity[];
  removed: DiffEntity[];
  bbox: [number, number, number, number];
  highlightDiffs?: boolean;
}) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [size, setSize] = useState({ w: 600, h: 400 });

  useEffect(() => {
    if (containerRef.current) {
      const rect = containerRef.current.getBoundingClientRect();
      setSize({ w: rect.width, h: rect.height });
    }
  }, []);

  const [minX, minY, maxX, maxY] = bbox;
  const bw = maxX - minX || 100;
  const bh = maxY - minY || 100;

  // Scale to fit canvas with padding
  const padding = 40;
  const scale = Math.min((size.w - padding * 2) / bw, (size.h - padding * 2) / bh);
  const offsetX = (size.w - bw * scale) / 2 - minX * scale;
  const offsetY = (size.h - bh * scale) / 2 + maxY * scale; // flip Y

  const tx = (x: number) => x * scale + offsetX;
  const ty = (y: number) => -y * scale + offsetY; // flip Y for DXF coordinate system

  // Build added entity lookup set
  const addedSet = new Set(added.map(e => entityKey(e)));

  return (
    <div ref={containerRef} style={{ flex: 1, overflow: 'hidden', position: 'relative', backgroundColor: 'var(--bg-primary)' }}>
      <svg width={size.w} height={size.h} style={{ display: 'block' }}>
        {/* Grid lines (light) */}
        <defs>
          <pattern id="grid" width="20" height="20" patternUnits="userSpaceOnUse">
            <path d="M 20 0 L 0 0 0 20" fill="none" stroke="var(--border)" strokeWidth="0.5" />
          </pattern>
        </defs>
        <rect width={size.w} height={size.h} fill="url(#grid)" opacity="0.3" />

        {/* Render entities */}
        {entities.map((e, i) => {
          const isAdded = highlightDiffs && addedSet.has(entityKey(e));
          const stroke = isAdded ? 'var(--error)' : 'var(--accent)';
          const strokeWidth = isAdded ? 2 : 1;
          const opacity = isAdded ? 1 : 0.6;

          switch (e.type) {
            case 'line':
              if (e.coords.length >= 4) {
                return (
                  <g key={i}>
                    <line x1={tx(e.coords[0])} y1={ty(e.coords[1])} x2={tx(e.coords[2])} y2={ty(e.coords[3])}
                      stroke={stroke} strokeWidth={strokeWidth} opacity={opacity} />
                    {isAdded && <DiffCircle cx={tx((e.coords[0] + e.coords[2]) / 2)} cy={ty((e.coords[1] + e.coords[3]) / 2)} />}
                  </g>
                );
              }
              return null;
            case 'lwpolyline':
            case 'polyline':
              if (e.coords_2d.length >= 2) {
                const points = e.coords_2d.map(p => `${tx(p[0])},${ty(p[1])}`).join(' ');
                return (
                  <g key={i}>
                    <polyline points={points} fill="none" stroke={stroke} strokeWidth={strokeWidth} opacity={opacity} />
                    {isAdded && <DiffCircle cx={tx(e.coords_2d[0][0])} cy={ty(e.coords_2d[0][1])} />}
                  </g>
                );
              }
              return null;
            case 'insert':
              if (e.coords.length >= 2) {
                return (
                  <g key={i}>
                    <rect x={tx(e.coords[0]) - 4} y={ty(e.coords[1]) - 4} width="8" height="8"
                      fill="none" stroke={stroke} strokeWidth={strokeWidth} opacity={opacity} />
                    {isAdded && <DiffCircle cx={tx(e.coords[0])} cy={ty(e.coords[1])} />}
                  </g>
                );
              }
              return null;
            default:
              return null;
          }
        })}

        {/* Render removed entities (only on template side) */}
        {removed.map((e, i) => {
          const stroke = 'var(--warning)';
          switch (e.type) {
            case 'line':
              if (e.coords.length >= 4) {
                return <line key={`r${i}`} x1={tx(e.coords[0])} y1={ty(e.coords[1])} x2={tx(e.coords[2])} y2={ty(e.coords[3])}
                  stroke={stroke} strokeWidth={2} strokeDasharray="4,2" opacity={0.5} />;
              }
              return null;
            case 'lwpolyline':
            case 'polyline':
              if (e.coords_2d.length >= 2) {
                const points = e.coords_2d.map(p => `${tx(p[0])},${ty(p[1])}`).join(' ');
                return <polyline key={`r${i}`} points={points} fill="none" stroke={stroke} strokeWidth={2} strokeDasharray="4,2" opacity={0.5} />;
              }
              return null;
            case 'insert':
              if (e.coords.length >= 2) {
                return <rect key={`r${i}`} x={tx(e.coords[0]) - 4} y={ty(e.coords[1]) - 4} width="8" height="8"
                  fill="none" stroke={stroke} strokeWidth={2} strokeDasharray="2,2" opacity={0.5} />;
              }
              return null;
            default:
              return null;
          }
        })}
      </svg>
    </div>
  );
}

// DiffCircle draws a red circle highlight around a difference
function DiffCircle({ cx, cy }: { cx: number; cy: number }) {
  return (
    <g>
      <circle cx={cx} cy={cy} r="12" fill="none" stroke="var(--error)" strokeWidth="2" opacity="0.8">
        <animate attributeName="r" values="8;16;8" dur="2s" repeatCount="indefinite" />
        <animate attributeName="opacity" values="0.8;0.3;0.8" dur="2s" repeatCount="indefinite" />
      </circle>
    </g>
  );
}

// Helper: entity key for matching
function entityKey(e: DiffEntity): string {
  const coords = e.coords.map(c => c.toFixed(2)).join(',');
  return `${e.type}:${e.block_name}:${coords}`;
}

// Helper: format file size
function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}