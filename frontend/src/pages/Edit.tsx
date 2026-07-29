import { useState, useEffect, useCallback } from 'react';
import {
  Wrench, FolderTree, Folder, FileText, ChevronRight, ChevronDown,
  RefreshCw, Loader2, Save, Code, Play, Layers,
} from 'lucide-react';
import { useStore } from '../store';
import { api, type TreeNode, type TemplateGroup } from '../api';

export default function Edit() {
  const { activeProject } = useStore();
  const [tree, setTree] = useState<TreeNode | null>(null);
  const [loading, setLoading] = useState(false);
  const [selectedFile, setSelectedFile] = useState<string | null>(null);
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

    // Check if it's a DXF file
    if (filePath.toLowerCase().endsWith('.dxf')) {
      setContentLoading(true);
      try {
        const resp = await api.getDXFContent(filePath);
        setDxfContent(resp.content);
      } catch (err) {
        setDxfContent('Error loading file: ' + (err instanceof Error ? err.message : 'unknown'));
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
      <div style={{ width: '320px', flexShrink: 0, borderRight: '1px solid var(--border)', display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
        <div style={{ padding: '12px 16px', borderBottom: '1px solid var(--border)', backgroundColor: 'var(--bg-secondary)', display: 'flex', alignItems: 'center', gap: '8px' }}>
          <FolderTree size={16} color="var(--accent)" />
          <span style={{ fontSize: '13px', fontWeight: 600 }}>Files</span>
          <button className="btn btn-ghost" style={{ marginLeft: 'auto', padding: '4px 8px' }} onClick={loadTree} title="Refresh">
            <RefreshCw size={14} />
          </button>
          <button className="btn btn-ghost" style={{ padding: '4px 8px' }} onClick={loadGroups} title="Template Groups">
            <Layers size={14} />
          </button>
        </div>

        {/* Template Groups Quick-Select */}
        {showGroups && groups.length > 0 && (
          <div style={{ maxHeight: '200px', overflowY: 'auto', borderBottom: '2px solid var(--border)', backgroundColor: 'var(--bg-elevated)' }}>
            <div style={{ padding: '8px 12px', fontSize: '11px', fontWeight: 600, color: 'var(--text-secondary)' }}>
              Template Groups (click to select)
            </div>
            {groups.filter(g => g.mod_folders.length > 0).map(g => (
              <div
                key={g.template_name}
                onClick={() => setSelectedGroup(g.template_name)}
                style={{
                  padding: '4px 12px',
                  fontSize: '11px',
                  cursor: 'pointer',
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

        <div style={{ flex: 1, overflowY: 'auto', padding: '8px' }}>
          {tree && <EditTreeView node={tree} level={0} selectedFile={selectedFile} onFileClick={handleFileClick} />}
        </div>
      </div>

      {/* Right Panel — DXF Editor */}
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
        {selectedFile ? (
          <>
            <div style={{ padding: '12px 16px', borderBottom: '1px solid var(--border)', backgroundColor: 'var(--bg-secondary)', display: 'flex', alignItems: 'center', gap: '8px' }}>
              <FileText size={16} color="var(--accent)" />
              <span style={{ fontSize: '13px', fontWeight: 600, fontFamily: 'var(--font-mono)' }}>
                {selectedFile.split('\\').pop()}
              </span>
              <button className="btn btn-primary" style={{ marginLeft: 'auto', padding: '4px 12px', fontSize: 11 }} onClick={handleSave} disabled={saving}>
                {saving ? <Loader2 size={12} className="spin" /> : <Save size={12} />}
                Save (creates .bak backup)
              </button>
            </div>

            {saveMsg && (
              <div style={{ padding: '8px 16px', background: 'rgba(0,138,0,0.08)', borderBottom: '1px solid var(--border)', fontSize: '12px', color: 'var(--accent)' }}>
                {saveMsg}
              </div>
            )}

            {/* Edit Script Section */}
            <div style={{ padding: '12px 16px', borderBottom: '1px solid var(--border)', backgroundColor: 'var(--bg-elevated)' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '8px' }}>
                <Code size={14} color="var(--accent)" />
                <span style={{ fontSize: '12px', fontWeight: 600 }}>Edit Script (find/replace operations as JSON)</span>
                {selectedGroup && (
                  <span style={{ fontSize: '11px', color: 'var(--accent)', marginLeft: '8px' }}>
                    Target group: <strong>{selectedGroup}</strong>
                  </span>
                )}
                <button className="btn btn-primary" style={{ marginLeft: 'auto', padding: '4px 12px', fontSize: 11 }} onClick={handleApplyEditScript} disabled={applying || !selectedGroup}>
                  {applying ? <Loader2 size={12} className="spin" /> : <Play size={12} />}
                  Apply Script to Template + Group
                </button>
              </div>
              <textarea
                value={editOps}
                onChange={(e) => setEditOps(e.target.value)}
                style={{
                  width: '100%',
                  height: '80px',
                  background: 'var(--bg-primary)',
                  border: '1px solid var(--border-light)',
                  borderRadius: 6,
                  padding: '8px',
                  color: 'var(--text-primary)',
                  fontFamily: 'var(--font-mono)',
                  fontSize: 12,
                  resize: 'vertical',
                }}
                placeholder='[{"find": "old_text", "replace": "new_text"}]'
              />
              {applyMsg && (
                <div style={{ marginTop: '8px', padding: '6px 12px', background: 'rgba(0,138,0,0.08)', borderRadius: 4, fontSize: '12px', color: 'var(--accent)' }}>
                  {applyMsg}
                </div>
              )}
            </div>

            {/* DXF Content Editor */}
            <div style={{ flex: 1, overflow: 'auto', padding: '0' }}>
              {contentLoading ? (
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%' }}>
                  <Loader2 size={24} className="spin" color="var(--accent)" />
                </div>
              ) : (
                <textarea
                  value={dxfContent}
                  onChange={(e) => setDxfContent(e.target.value)}
                  style={{
                    width: '100%',
                    height: '100%',
                    background: 'var(--bg-primary)',
                    border: 'none',
                    padding: '12px 16px',
                    color: 'var(--text-primary)',
                    fontFamily: 'var(--font-mono)',
                    fontSize: 11,
                    resize: 'none',
                    outline: 'none',
                    lineHeight: 1.5,
                  }}
                  spellCheck={false}
                />
              )}
            </div>
          </>
        ) : (
          <div style={{ flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', padding: '24px' }}>
            <Wrench size={32} color="var(--text-muted)" style={{ marginBottom: '8px' }} />
            <p style={{ color: 'var(--text-muted)', fontSize: '14px' }}>Select a DXF file to edit</p>
            <p style={{ color: 'var(--text-muted)', fontSize: '12px', marginTop: '4px' }}>
              View raw DXF content, apply find/replace edit scripts, and apply fixed templates to groups.
            </p>
            <button className="btn btn-secondary" style={{ marginTop: '16px' }} onClick={loadGroups}>
              <Layers size={14} />
              View Template Groups
            </button>
          </div>
        )}
      </div>
    </div>
  );
}

// EditTreeView — simplified tree that shows all files
function EditTreeView({ node, level, selectedFile, onFileClick }: {
  node: TreeNode;
  level: number;
  selectedFile: string | null;
  onFileClick: (filePath: string) => void;
}) {
  const [expanded, setExpanded] = useState(level < 2);

  if (!node.is_dir) {
    const isDXF = !node.is_dir && node.name.toLowerCase().endsWith('.dxf');
    if (!isDXF) return null; // Only show DXF files in edit mode

    return (
      <div
        onClick={() => onFileClick(node.path)}
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: '6px',
          padding: '3px 8px',
          marginLeft: level * 16,
          cursor: 'pointer',
          fontSize: 12,
          fontFamily: 'var(--font-mono)',
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

  // Count DXF children
  const hasDXF = node.children?.some(c => c.is_dir || c.name.toLowerCase().endsWith('.dxf'));
  if (!hasDXF && level > 0) return null;

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