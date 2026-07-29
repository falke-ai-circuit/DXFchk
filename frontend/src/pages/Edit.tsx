import { useState, useEffect, useCallback } from 'react';
import {
  Wrench, FolderTree, Folder, FileText, ChevronRight, ChevronDown,
  RefreshCw, Loader2, Save, Code, Play, Layers, Eye,
} from 'lucide-react';
import { useStore } from '../store';
import { api, type TreeNode, type TemplateGroup, type DXFRenderResponse } from '../api';
import DXFViewer from '../components/DXFViewer';

type ViewMode = 'visual' | 'raw';

export default function Edit() {
  const { activeProject } = useStore();
  const [tree, setTree] = useState<TreeNode | null>(null);
  const [loading, setLoading] = useState(false);
  const [selectedFile, setSelectedFile] = useState<string | null>(null);
  const [viewMode, setViewMode] = useState<ViewMode>('visual');

  // Visual render data
  const [renderData, setRenderData] = useState<DXFRenderResponse | null>(null);
  const [renderLoading, setRenderLoading] = useState(false);

  // Raw DXF content
  const [dxfContent, setDxfContent] = useState<string>('');
  const [contentLoading, setContentLoading] = useState(false);

  const [saving, setSaving] = useState(false);
  const [saveMsg, setSaveMsg] = useState<string | null>(null);
  const [groups, setGroups] = useState<TemplateGroup[]>([]);
  const [showGroups, setShowGroups] = useState(false);
  const [selectedGroup, setSelectedGroup] = useState<string | null>(null);
  const [editOps, setEditOps] = useState<string>('[\n  {"find": "", "replace": ""}\n]');
  const [applying, setApplying] = useState(false);
  const [applyMsg, setApplyMsg] = useState<string | null>(null);

  const outputFolder = activeProject?.output_folder || '';

  const loadTree = useCallback(async () => {
    if (!outputFolder) return;
    setLoading(true);
    try {
      const resp = await api.browse(outputFolder);
      setTree(resp.tree);
    } catch {
      setTree(null);
    }
    setLoading(false);
  }, [outputFolder]);

  useEffect(() => {
    loadTree();
  }, [loadTree]);

  const loadGroups = async () => {
    if (!outputFolder) return;
    try {
      const resp = await api.getTemplateGroups(outputFolder);
      setGroups(resp.groups);
      setShowGroups(true);
    } catch { /* ignore */ }
  };

  const handleFileClick = async (filePath: string) => {
    setSelectedFile(filePath);
    setSaveMsg(null);
    setApplyMsg(null);
    setRenderData(null);
    setDxfContent('');

    if (filePath.toLowerCase().endsWith('.dxf')) {
      // Load visual render data (primary)
      setRenderLoading(true);
      try {
        const resp = await api.getDXFRender(filePath);
        setRenderData(resp);
      } catch (err) {
        setRenderData(null);
      }
      setRenderLoading(false);

      // Also load raw content for toggle
      setContentLoading(true);
      try {
        const resp = await api.getDXFContent(filePath);
        setDxfContent(resp.content);
      } catch {
        setDxfContent('');
      }
      setContentLoading(false);
    }
  };

  const handleSave = async () => {
    if (!selectedFile) return;
    setSaving(true);
    setSaveMsg(null);
    try {
      const resp = await api.saveDXFContent(selectedFile, dxfContent);
      setSaveMsg(resp.message);
    } catch (err) {
      setSaveMsg('Error: ' + (err instanceof Error ? err.message : 'unknown'));
    }
    setSaving(false);
  };

  const handleApplyEditScript = async () => {
    if (!selectedFile || !selectedGroup) return;
    setApplying(true);
    setApplyMsg(null);
    try {
      const resp = await api.applyEditScript(selectedFile, editOps, selectedGroup, outputFolder);
      setApplyMsg(resp.message);
      loadTree();
    } catch (err) {
      setApplyMsg('Error: ' + (err instanceof Error ? err.message : 'unknown'));
    }
    setApplying(false);
  };

  if (!activeProject) {
    return (
      <div style={{ padding: '24px', textAlign: 'center' }}>
        <Wrench size={32} color="var(--text-muted)" style={{ marginBottom: '8px' }} />
        <p style={{ color: 'var(--text-muted)' }}>Select a project on the Dashboard first.</p>
      </div>
    );
  }

  if (loading) {
    return (
      <div style={{ padding: '24px', textAlign: 'center' }}>
        <Loader2 size={24} className="spin" color="var(--accent)" />
        <p style={{ color: 'var(--text-muted)', marginTop: '8px' }}>Loading...</p>
      </div>
    );
  }

  return (
    <div style={{ display: 'flex', height: '100%', overflow: 'hidden' }}>
      {/* Left Panel — Tree View */}
      <div style={{ width: 280, flexShrink: 0, borderRight: '1px solid var(--border)', display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
        <div style={{ padding: '10px 14px', borderBottom: '1px solid var(--border)', backgroundColor: 'var(--bg-secondary)', display: 'flex', alignItems: 'center', gap: '6px' }}>
          <FolderTree size={16} color="var(--accent)" />
          <span style={{ fontSize: 13, fontWeight: 600 }}>Files</span>
          <button className="btn btn-ghost" style={{ marginLeft: 'auto', padding: '3px 6px' }} onClick={loadTree} title="Refresh">
            <RefreshCw size={14} />
          </button>
          <button className="btn btn-ghost" style={{ padding: '3px 6px' }} onClick={loadGroups} title="Template Groups">
            <Layers size={14} />
          </button>
        </div>

        {/* Template Groups Quick-Select */}
        {showGroups && groups.length > 0 && (
          <div style={{ maxHeight: 180, overflowY: 'auto', borderBottom: '2px solid var(--border)', backgroundColor: 'var(--bg-elevated)' }}>
            <div style={{ padding: '6px 10px', fontSize: 11, fontWeight: 600, color: 'var(--text-secondary)' }}>
              Template Groups (click to select)
            </div>
            {groups.filter(g => g.mod_folders.length > 0).map(g => (
              <div
                key={g.template_name}
                onClick={() => setSelectedGroup(g.template_name)}
                style={{
                  padding: '3px 10px', fontSize: 11, cursor: 'pointer',
                  background: selectedGroup === g.template_name ? 'rgba(0,138,0,0.08)' : 'transparent',
                  color: selectedGroup === g.template_name ? 'var(--accent)' : 'var(--text-primary)',
                  fontWeight: selectedGroup === g.template_name ? 600 : 400,
                }}
              >
                {g.template_name} — {g.total_files} files, {g.mod_folders.length} mod folders
              </div>
            ))}
          </div>
        )}

        <div style={{ flex: 1, overflowY: 'auto', padding: '6px' }}>
          {tree && <EditTreeView node={tree} level={0} selectedFile={selectedFile} onFileClick={handleFileClick} />}
        </div>
      </div>

      {/* Right Panel — DXF Viewer / Editor */}
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
        {selectedFile ? (
          <>
            {/* Header bar with view mode toggle */}
            <div style={{ padding: '8px 14px', borderBottom: '1px solid var(--border)', backgroundColor: 'var(--bg-secondary)', display: 'flex', alignItems: 'center', gap: '8px' }}>
              <FileText size={16} color="var(--accent)" />
              <span style={{ fontSize: 13, fontWeight: 600, fontFamily: 'var(--font-mono)' }}>
                {selectedFile.split('\\').pop()?.split('/').pop()}
              </span>

              {/* View mode toggle */}
              <div style={{ marginLeft: 'auto', display: 'flex', gap: 4, alignItems: 'center' }}>
                {renderData && (
                  <span style={{ fontSize: 10, color: 'var(--text-muted)', marginRight: 8 }}>
                    {renderData.count} entities · {renderData.layer_count} layers
                  </span>
                )}
                <button
                  className={`btn ${viewMode === 'visual' ? 'btn-primary' : 'btn-ghost'}`}
                  style={{ padding: '3px 10px', fontSize: 11, display: 'flex', alignItems: 'center', gap: 4 }}
                  onClick={() => setViewMode('visual')}
                >
                  <Eye size={12} /> Visual
                </button>
                <button
                  className={`btn ${viewMode === 'raw' ? 'btn-primary' : 'btn-ghost'}`}
                  style={{ padding: '3px 10px', fontSize: 11, display: 'flex', alignItems: 'center', gap: 4 }}
                  onClick={() => setViewMode('raw')}
                >
                  <Code size={12} /> Raw
                </button>
                {viewMode === 'raw' && (
                  <button className="btn btn-primary" style={{ padding: '3px 10px', fontSize: 11, marginLeft: 4 }} onClick={handleSave} disabled={saving}>
                    {saving ? <Loader2 size={12} className="spin" /> : <Save size={12} />}
                    Save
                  </button>
                )}
              </div>
            </div>

            {saveMsg && (
              <div style={{ padding: '6px 14px', background: 'rgba(0,138,0,0.08)', borderBottom: '1px solid var(--border)', fontSize: 12, color: 'var(--accent)' }}>
                {saveMsg}
              </div>
            )}

            {/* Edit Script Section */}
            <div style={{ padding: '10px 14px', borderBottom: '1px solid var(--border)', backgroundColor: 'var(--bg-elevated)' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '6px', marginBottom: '6px' }}>
                <Code size={14} color="var(--accent)" />
                <span style={{ fontSize: 12, fontWeight: 600 }}>Edit Script (find/replace as JSON)</span>
                {selectedGroup && (
                  <span style={{ fontSize: 11, color: 'var(--accent)', marginLeft: '6px' }}>
                    Target: <strong>{selectedGroup}</strong>
                  </span>
                )}
                <button className="btn btn-primary" style={{ marginLeft: 'auto', padding: '3px 10px', fontSize: 11 }} onClick={handleApplyEditScript} disabled={applying || !selectedGroup}>
                  {applying ? <Loader2 size={12} className="spin" /> : <Play size={12} />}
                  Apply to Template + Group
                </button>
              </div>
              <textarea
                value={editOps}
                onChange={(e) => setEditOps(e.target.value)}
                style={{
                  width: '100%', height: 60, background: 'var(--bg-primary)',
                  border: '1px solid var(--border-light)', borderRadius: 6,
                  padding: '6px', color: 'var(--text-primary)',
                  fontFamily: 'var(--font-mono)', fontSize: 12, resize: 'vertical',
                }}
                placeholder='[{"find": "old_text", "replace": "new_text"}]'
              />
              {applyMsg && (
                <div style={{ marginTop: '6px', padding: '4px 10px', background: 'rgba(0,138,0,0.08)', borderRadius: 4, fontSize: 12, color: 'var(--accent)' }}>
                  {applyMsg}
                </div>
              )}
            </div>

            {/* Main content area — visual or raw */}
            <div style={{ flex: 1, overflow: 'hidden', display: 'flex', flexDirection: 'column' }}>
              {viewMode === 'visual' ? (
                renderLoading ? (
                  <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                    <Loader2 size={32} className="spin" color="var(--accent)" />
                    <span style={{ marginLeft: 12, color: 'var(--text-muted)' }}>Rendering DXF...</span>
                  </div>
                ) : renderData ? (
                  <DXFViewer
                    entities={renderData.entities}
                    boundingBox={renderData.bounding_box}
                    layers={renderData.layers}
                  />
                ) : (
                  <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'var(--text-muted)' }}>
                    Failed to render DXF file
                  </div>
                )
              ) : (
                /* Raw DXF text editor */
                contentLoading ? (
                  <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                    <Loader2 size={24} className="spin" color="var(--accent)" />
                  </div>
                ) : (
                  <textarea
                    value={dxfContent}
                    onChange={(e) => setDxfContent(e.target.value)}
                    style={{
                      width: '100%', height: '100%', background: 'var(--bg-primary)',
                      border: 'none', padding: '12px 16px', color: 'var(--text-primary)',
                      fontFamily: 'var(--font-mono)', fontSize: 11, resize: 'none',
                      outline: 'none', lineHeight: 1.5,
                    }}
                    spellCheck={false}
                  />
                )
              )}
            </div>
          </>
        ) : (
          <div style={{ flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', padding: '24px' }}>
            <Wrench size={32} color="var(--text-muted)" style={{ marginBottom: '8px' }} />
            <p style={{ color: 'var(--text-muted)', fontSize: 14 }}>Select a DXF file to view</p>
            <p style={{ color: 'var(--text-muted)', fontSize: 12, marginTop: 4 }}>
              Visual CAD rendering with layer colors, pan/zoom, and entity info.
            </p>
            <button className="btn btn-secondary" style={{ marginTop: 16 }} onClick={loadGroups}>
              <Layers size={14} />
              View Template Groups
            </button>
          </div>
        )}
      </div>
    </div>
  );
}

// EditTreeView — tree showing DXF files
function EditTreeView({ node, level, selectedFile, onFileClick }: {
  node: TreeNode;
  level: number;
  selectedFile: string | null;
  onFileClick: (filePath: string) => void;
}) {
  const [expanded, setExpanded] = useState(level < 2);

  if (!node.is_dir) {
    const isDXF = node.name.toLowerCase().endsWith('.dxf');
    if (!isDXF) return null;

    return (
      <div
        onClick={() => onFileClick(node.path)}
        style={{
          display: 'flex', alignItems: 'center', gap: '6px',
          padding: '3px 8px', marginLeft: level * 16, cursor: 'pointer',
          fontSize: 12, fontFamily: 'var(--font-mono)',
          color: selectedFile === node.path ? 'var(--accent)' : 'var(--text-primary)',
          background: selectedFile === node.path ? 'rgba(0,138,0,0.08)' : 'transparent',
          borderRadius: 4,
        }}
      >
        <FileText size={12} color="var(--accent)" />
        <span>{node.name}</span>
      </div>
    );
  }

  const hasDXF = node.children?.some(c => c.is_dir || c.name.toLowerCase().endsWith('.dxf'));
  if (!hasDXF && level > 0) return null;

  return (
    <div>
      <div
        onClick={() => setExpanded(!expanded)}
        style={{
          display: 'flex', alignItems: 'center', gap: '4px',
          padding: '3px 8px', marginLeft: level * 16, cursor: 'pointer',
          fontSize: 12, fontFamily: 'var(--font-mono)',
          color: node.is_mod ? 'var(--warning)' : 'var(--text-primary)',
          borderRadius: 4,
        }}
      >
        {expanded ? <ChevronDown size={12} color="var(--text-muted)" /> : <ChevronRight size={12} color="var(--text-muted)" />}
        <Folder size={14} color={node.is_mod ? 'var(--warning)' : 'var(--accent)'} />
        <span>{node.name}</span>
        {node.is_mod && <span className="badge badge-warning" style={{ fontSize: 9, padding: '1px 4px' }}>mod</span>}
      </div>
      {expanded && node.children && (
        <div>
          {node.children.map((child, i) => (
            <EditTreeView key={i} node={child} level={level + 1} selectedFile={selectedFile} onFileClick={onFileClick} />
          ))}
        </div>
      )}
    </div>
  );
}