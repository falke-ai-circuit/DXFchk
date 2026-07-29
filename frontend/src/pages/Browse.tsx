import { useState, useEffect, useRef, useCallback } from 'react';
import {
  FolderTree, Folder, FileText, ChevronRight, ChevronDown,
  RefreshCw, Loader2, GitCompare, Plus, AlertCircle, Wrench, Layers, FileText as FileLog,
  ExternalLink,
} from 'lucide-react';
import { useStore } from '../store';
import { api, type TreeNode, type DiffResponse, type DiffEntity, type TemplateGroup, type DXFRenderResponse } from '../api';
import DXFViewer from '../components/DXFViewer';

export default function Browse() {
  const { activeProject } = useStore();
  const [tree, setTree] = useState<TreeNode | null>(null);
  const [loading, setLoading] = useState(false);
  const [selectedFile, setSelectedFile] = useState<string | null>(null);
  const [diff, setDiff] = useState<DiffResponse | null>(null);
  const [diffLoading, setDiffLoading] = useState(false);
  const [templatePath, setTemplatePath] = useState<string | null>(null);
  const [createTemplateMsg, setCreateTemplateMsg] = useState<string | null>(null);
  const [showGroups, setShowGroups] = useState(false);
  const [groups, setGroups] = useState<TemplateGroup[]>([]);
  const [groupsLoading, setGroupsLoading] = useState(false);
  const [applyMsg, setApplyMsg] = useState<string | null>(null);
  const [logContent, setLogContent] = useState<string | null>(null);
  const [logLoading, setLogLoading] = useState(false);
  const [compareMode, setCompareMode] = useState<'template' | 'mod'>('template');
  const [renderData, setRenderData] = useState<DXFRenderResponse | null>(null);
  const [renderLoading, setRenderLoading] = useState(false);
  const [contextMenu, setContextMenu] = useState<{ x: number; y: number; filePath: string } | null>(null);
  const [externalMsg, setExternalMsg] = useState<string | null>(null);

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

  // Auto-select file from URL query param: /browse?file=PATH
  // handleFileClick is defined below, so we use a ref to call it
  const autoFileRef = useRef<string | null>(null);
  const pendingFileRef = useRef<string | null>(null);

  // Read the file param once on mount
  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const fileParam = params.get('file');
    if (fileParam) {
      pendingFileRef.current = fileParam;
    }
  }, []);
  
  const loadGroups = async () => {
    if (!outputFolder) return;
    setGroupsLoading(true);
    try {
      const resp = await api.getTemplateGroups(outputFolder);
      setGroups(resp.groups);
      setShowGroups(true);
    } catch { /* ignore */ }
    setGroupsLoading(false);
  };

  const findTemplateForFile = async (filePath: string) => {
    if (!activeProject?.template_folder) return null;
    // Derive template name from the folder structure:
    // - If file is in Output/TEMPLATE_NAME/TEMPLATE_NAME_mod1/ → template = TEMPLATE_NAME
    // - If file is in Output/TEMPLATE_NAME/ → template = TEMPLATE_NAME
    // - If file is in Output/notemplate/ → no template
    const parts = filePath.split('\\');
    // Find the output folder in the path to get the template folder name
    const outputBase = activeProject.output_folder?.split('\\').pop() || '';
    const outputIdx = parts.findIndex(p => p === outputBase);
    let templateName = '';
    if (outputIdx >= 0 && outputIdx + 1 < parts.length) {
      templateName = parts[outputIdx + 1];
    } else {
      // Fallback: use the immediate parent folder if it's not a _mod folder
      const parentFolder = parts[parts.length - 2] || '';
      if (parentFolder.includes('_mod')) {
        templateName = parentFolder.split('_mod')[0];
      } else {
        templateName = parentFolder;
      }
    }
    if (!templateName || templateName === 'notemplate') return null;

    // Find template file in template folder — match by filename prefix
    try {
      const resp = await api.browseFolder(activeProject.template_folder);
      // Try exact match first (TEMPLATE_NAME.dxf)
      const exactMatch = resp.children?.find(c =>
        !c.is_dir && c.name.toLowerCase().replace('.dxf', '') === templateName.toLowerCase()
      );
      if (exactMatch) return exactMatch.path;
      // Try prefix match (TEMPLATE_NAME*.dxf)
      const prefixMatch = resp.children?.find(c =>
        !c.is_dir && c.name.toLowerCase().startsWith(templateName.toLowerCase()) && c.name.toLowerCase().endsWith('.dxf')
      );
      if (prefixMatch) return prefixMatch.path;
    } catch { /* ignore */ }
    return null;
  };

  const findModTemplate = async (filePath: string) => {
    // Look for {template}_fixed.dxf in the same folder
    const dir = filePath.substring(0, filePath.lastIndexOf('\\'));
    // Extract template name from folder (e.g., "BI001" from "BI001_mod1")
    const folderName = dir.split('\\').pop() || '';
    if (folderName.includes('_mod')) {
      const baseName = folderName.split('_mod')[0];
      const fixedPath = dir + '\\' + baseName + '_fixed.dxf';
      try {
        const resp = await api.getDXFContent(fixedPath);
        if (resp.content) return fixedPath;
      } catch { /* not found */ }
    }
    return null;
  };

  const handleFileClick = async (filePath: string, _folderPath: string) => {
    setSelectedFile(filePath);
    setDiff(null);
    setCreateTemplateMsg(null);
    setApplyMsg(null);
    setLogContent(null);

    const isLog = filePath.toLowerCase().endsWith('.log');
    const isDXF = filePath.toLowerCase().endsWith('.dxf');

    if (isLog) {
      // Load log content
      setLogLoading(true);
      try {
        const resp = await api.getLogContent(filePath);
        setLogContent(resp.content);
      } catch (err) {
        setLogContent('Error loading log: ' + (err instanceof Error ? err.message : 'unknown'));
      }
      setLogLoading(false);
      return;
    }

    if (!isDXF) return;

    // Always load render data so we can see the DXF content visually
    setRenderData(null);
    setRenderLoading(true);
    try {
      const resp = await api.getDXFRender(filePath);
      setRenderData(resp);
    } catch (err) {
      console.error('Render failed:', err);
    }
    setRenderLoading(false);

    // Also find template and show diff
    const tmpl = await findTemplateForFile(filePath);
    setTemplatePath(tmpl);

    if (tmpl) {
      setDiffLoading(true);
      try {
        const d = await api.diff(tmpl, filePath);
        setDiff(d);
      } catch (err) {
        console.error('Diff failed:', err);
      }
      setDiffLoading(false);
    }
  };

  // Auto-select file from URL query param after projects are loaded
  useEffect(() => {
    if (autoFileRef.current) return; // only once
    if (!activeProject || !outputFolder) return; // wait for project
    if (pendingFileRef.current) {
      autoFileRef.current = pendingFileRef.current;
      handleFileClick(pendingFileRef.current, '');
    }
  }, [activeProject, outputFolder, loadTree]);

  const handleSwitchCompareMode = async (mode: 'template' | 'mod') => {
    if (!selectedFile) return;
    setCompareMode(mode);
    setDiff(null);

    if (mode === 'mod') {
      // Compare against mod template (fixed template in same folder)
      const modTemplate = await findModTemplate(selectedFile);
      if (modTemplate) {
        setDiffLoading(true);
        try {
          const d = await api.diff(modTemplate, selectedFile);
          setDiff(d);
        } catch { /* ignore */ }
        setDiffLoading(false);
      }
    } else {
      // Compare against original template
      const tmpl = await findTemplateForFile(selectedFile);
      setTemplatePath(tmpl);
      if (tmpl) {
        setDiffLoading(true);
        try {
          const d = await api.diff(tmpl, selectedFile);
          setDiff(d);
        } catch { /* ignore */ }
        setDiffLoading(false);
      }
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

  const handleApplyTemplate = async (groupName: string, fixedFilePath: string) => {
    setApplyMsg(`Applying fixed template to group ${groupName}...`);
    try {
      const resp = await api.applyTemplate({
        template_path: fixedFilePath,
        group_name: groupName,
        output_folder: outputFolder,
      });
      setApplyMsg(`✓ ${resp.message} (${resp.total_files} files affected)`);
      loadTree();
    } catch (err) {
      setApplyMsg(`✗ Failed: ${err instanceof Error ? err.message : 'unknown error'}`);
    }
  };

  const handleOpenExternal = async (filePath: string) => {
    setExternalMsg('Opening in external editor...');
    setContextMenu(null);
    try {
      const resp = await api.openExternal(filePath);
      setExternalMsg(`✓ Opened in ${resp.editor || 'default editor'}`);
    } catch (err) {
      setExternalMsg(`✗ Failed: ${err instanceof Error ? err.message : 'unknown'}`);
    }
  };

  const handleBrowseContextMenu = (e: React.MouseEvent, filePath: string) => {
    e.preventDefault();
    e.stopPropagation();
    setContextMenu({ x: e.clientX, y: e.clientY, filePath });
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

  if (!tree) {
    return (
      <div style={{ padding: '24px', textAlign: 'center' }}>
        <p style={{ color: 'var(--text-muted)' }}>Output folder is empty. Run a comparison first.</p>
        <button className="btn btn-primary" style={{ marginTop: '12px' }} onClick={loadTree}>
          <RefreshCw size={14} /> Refresh
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
          <button className="btn btn-ghost" style={{ marginLeft: 'auto', padding: '4px 8px' }} onClick={loadTree} title="Refresh">
            <RefreshCw size={14} />
          </button>
          <button className="btn btn-ghost" style={{ padding: '4px 8px' }} onClick={loadGroups} title="Template Groups">
            {groupsLoading ? <Loader2 size={14} className="spin" /> : <Layers size={14} />}
          </button>
        </div>

        {/* Template Groups Panel */}
        {showGroups && groups.length > 0 && (
          <div style={{ maxHeight: '300px', overflowY: 'auto', borderBottom: '2px solid var(--border)', backgroundColor: 'var(--bg-elevated)' }}>
            <div style={{ padding: '8px 12px', fontSize: '11px', fontWeight: 600, color: 'var(--text-secondary)', display: 'flex', alignItems: 'center', gap: '6px' }}>
              <Wrench size={12} color="var(--warning)" />
              Template Groups — Fix & Apply
              <span style={{ marginLeft: 'auto', color: 'var(--text-muted)' }}>{groups.length} groups</span>
            </div>
            {groups.map(g => (
              <div key={g.template_name} style={{ padding: '6px 12px', borderBottom: '1px solid var(--border)', fontSize: '11px' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                  <span style={{ fontWeight: 600, color: 'var(--text-primary)' }}>{g.template_name}</span>
                  <span style={{ color: 'var(--text-muted)' }}>{g.total_files} files</span>
                  {g.matched_count > 0 && <span style={{ color: 'var(--success)' }}>✓{g.matched_count}</span>}
                  {g.mod_folders.length > 0 && <span style={{ color: 'var(--warning)' }}>mod:{g.mod_folders.length}</span>}
                </div>
                {g.mod_folders.map(mod => (
                  <div key={mod.folder_name} style={{ marginLeft: '12px', marginTop: '4px', display: 'flex', alignItems: 'center', gap: '4px' }}>
                    <span style={{ color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>{mod.folder_name}</span>
                    <span style={{ color: 'var(--text-muted)' }}>({mod.file_count})</span>
                  </div>
                ))}
              </div>
            ))}
          </div>
        )}

        <div style={{ flex: 1, overflowY: 'auto', padding: '8px' }} onClick={() => setContextMenu(null)}>
          <TreeView node={tree} level={0} selectedFile={selectedFile} onFileClick={handleFileClick} onContextMenu={handleBrowseContextMenu} />
        </div>
      </div>

      {/* Right Panel — Content Viewer */}
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
        {selectedFile ? (
          <>
            <div style={{ padding: '12px 16px', borderBottom: '1px solid var(--border)', backgroundColor: 'var(--bg-secondary)', display: 'flex', alignItems: 'center', gap: '8px' }}>
              <FileText size={16} color="var(--accent)" />
              <span style={{ fontSize: '13px', fontWeight: 600, fontFamily: 'var(--font-mono)' }}>
                {selectedFile.split('\\').pop()}
              </span>
              {templatePath && selectedFile.toLowerCase().endsWith('.dxf') && (
                <span style={{ fontSize: '11px', color: 'var(--text-muted)', marginLeft: '8px' }}>
                  vs {templatePath.split('\\').pop()}
                </span>
              )}

              {/* Compare mode switcher for DXF files in _mod folders */}
              {selectedFile.includes('_mod') && selectedFile.toLowerCase().endsWith('.dxf') && (
                <div style={{ marginLeft: 'auto', display: 'flex', gap: '4px' }}>
                  <button
                    className={`btn ${compareMode === 'template' ? 'btn-primary' : 'btn-ghost'}`}
                    style={{ padding: '4px 10px', fontSize: 10 }}
                    onClick={() => handleSwitchCompareMode('template')}
                  >
                    vs Template
                  </button>
                  <button
                    className={`btn ${compareMode === 'mod' ? 'btn-primary' : 'btn-ghost'}`}
                    style={{ padding: '4px 10px', fontSize: 10 }}
                    onClick={() => handleSwitchCompareMode('mod')}
                  >
                    vs Fixed Template
                  </button>
                </div>
              )}

              {diff && diff.summary.added_count > 0 && selectedFile.toLowerCase().endsWith('.dxf') && (
                <button className="btn btn-primary" style={{ marginLeft: 'auto', padding: '4px 12px', fontSize: 11 }} onClick={handleCreateTemplate}>
                  <Plus size={12} />
                  Create Template from this file
                </button>
              )}
              {selectedFile && selectedFile.includes('_mod') && selectedFile.toLowerCase().endsWith('.dxf') && (
                <button className="btn btn-primary" style={{ marginLeft: 'auto', padding: '4px 12px', fontSize: 11 }} onClick={() => {
                  const parts = selectedFile.split('\\');
                  const modFolder = parts.find(p => p.includes('_mod'));
                  if (modFolder) {
                    const groupName = modFolder.split('_mod')[0];
                    handleApplyTemplate(groupName, selectedFile);
                  }
                }}>
                  <Wrench size={12} />
                  Apply as Fixed Template to Group
                </button>
              )}
            </div>

            {/* Log Content View */}
            {logContent !== null && (
              <>
                <div style={{ padding: '8px 16px', borderBottom: '1px solid var(--border)', backgroundColor: 'var(--bg-elevated)', fontSize: '11px', color: 'var(--text-muted)' }}>
                  <FileLog size={12} style={{ display: 'inline', marginRight: '6px' }} />
                  Log file — {logContent.split('\n').length} lines
                </div>
                <div style={{ flex: 1, overflow: 'auto', padding: '12px 16px', fontFamily: 'var(--font-mono)', fontSize: '11px', color: 'var(--text-secondary)', lineHeight: 1.6, whiteSpace: 'pre-wrap' }}>
                  {logLoading ? (
                    <div style={{ textAlign: 'center', padding: '24px' }}>
                      <Loader2 size={24} className="spin" color="var(--accent)" />
                    </div>
                  ) : (
                    logContent.split('\n').map((line, i) => (
                      <div key={i} style={{ marginBottom: '1px' }}>
                        <span style={{ color: 'var(--text-muted)' }}>{String(i + 1).padStart(4, ' ')}</span>{' '}
                        {line}
                      </div>
                    ))
                  )}
                </div>
              </>
            )}

            {/* Diff Summary Bar */}
            {diff && !logContent && (
              <div style={{ padding: '8px 16px', borderBottom: '1px solid var(--border)', display: 'flex', gap: '16px', fontSize: '12px' }}>
                <span style={{ color: 'var(--text-muted)' }}>Template: <strong style={{ color: 'var(--text-primary)' }}>{diff.summary.template_count}</strong> entities</span>
                <span style={{ color: 'var(--text-muted)' }}>Module: <strong style={{ color: 'var(--text-primary)' }}>{diff.summary.module_count}</strong> entities</span>
                <span style={{ color: 'var(--error)' }}>Added: <strong>{diff.summary.added_count}</strong></span>
                <span style={{ color: 'var(--warning)' }}>Removed: <strong>{diff.summary.removed_count}</strong></span>
              </div>
            )}

            {createTemplateMsg && !logContent && (
              <div style={{ padding: '8px 16px', background: 'rgba(0,138,0,0.08)', borderBottom: '1px solid var(--border)', fontSize: '12px', color: 'var(--accent)' }}>
                {createTemplateMsg}
              </div>
            )}

            {applyMsg && !logContent && (
              <div style={{ padding: '8px 16px', background: 'rgba(0,138,0,0.08)', borderBottom: '1px solid var(--border)', fontSize: '12px', color: 'var(--accent)' }}>
                {applyMsg}
              </div>
            )}

            {/* DXF Viewer with diff highlighting */}
            {!logContent && (
              <div style={{ flex: 1, overflow: 'auto', display: 'flex', flexDirection: 'column', backgroundColor: 'var(--bg-primary)' }}>
                {/* Loading indicator */}
                {(diffLoading || renderLoading) && !diff && !renderData ? (
                  <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                    <Loader2 size={24} className="spin" color="var(--accent)" />
                  </div>
                ) : diff ? (
                  /* Single view — module DXF with differences highlighted */
                  <div style={{ flex: 1, display: 'flex', flexDirection: 'column' }}>
                    <div style={{ padding: '8px 12px', borderBottom: '1px solid var(--border)', display: 'flex', alignItems: 'center', gap: '12px', fontSize: '12px', backgroundColor: 'var(--bg-secondary)' }}>
                      <span style={{ fontWeight: 600, color: 'var(--text-secondary)' }}>
                        {selectedFile?.split('\\').pop()}
                      </span>
                      <span style={{ color: 'var(--text-muted)' }}>vs {templatePath?.split('\\').pop()}</span>
                      {diff.summary.added_count > 0 && <span style={{ color: 'var(--error)' }}>+{diff.summary.added_count} added</span>}
                      {diff.summary.removed_count > 0 && <span style={{ color: 'var(--warning)' }}>-{diff.summary.removed_count} removed</span>}
                    </div>
                    <DXFViewer
                      entities={diff.module_entities || []}
                      boundingBox={diff.bounding_box}
                      highlightAdded={true}
                      addedSet={new Set((diff.added || []).map(e => entityKey(e)))}
                      removedSet={new Set((diff.removed || []).map(e => entityKey(e)))}
                      showInfoPanel={true}
                    />
                  </div>
                ) : renderData ? (
                  /* Single DXF render (no template found, or just viewing) */
                  <div style={{ flex: 1, display: 'flex', flexDirection: 'column' }}>
                    {templatePath === null && selectedFile?.toLowerCase().endsWith('.dxf') && (
                      <div style={{ padding: '8px 16px', borderBottom: '1px solid var(--border)', display: 'flex', gap: '16px', fontSize: '12px', backgroundColor: 'var(--bg-elevated)' }}>
                        <span style={{ color: 'var(--warning)' }}>
                          <AlertCircle size={12} style={{ display: 'inline', marginRight: '4px' }} />
                          No matching template found
                        </span>
                        <button className="btn btn-primary" style={{ padding: '2px 10px', fontSize: 11 }} onClick={handleCreateTemplate}>
                          <Plus size={12} />
                          Create Template
                        </button>
                        {selectedFile.includes('_mod') && (
                          <button className="btn btn-primary" style={{ padding: '2px 10px', fontSize: 11 }} onClick={() => {
                            const parts = selectedFile.split('\\');
                            const modFolder = parts.find(p => p.includes('_mod'));
                            if (modFolder) {
                              const groupName = modFolder.split('_mod')[0];
                              handleApplyTemplate(groupName, selectedFile);
                            }
                          }}>
                            <Wrench size={12} />
                            Apply as Fixed Template to Group
                          </button>
                        )}
                      </div>
                    )}
                    <DXFViewer
                      entities={renderData.entities || []}
                      boundingBox={renderData.bounding_box}
                      layers={renderData.layers || []}
                      layerColors={renderData.layer_colors || {}}
                      showInfoPanel={true}
                    />
                  </div>
                ) : templatePath === null && selectedFile?.toLowerCase().endsWith('.dxf') ? (
                  <div style={{ flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', padding: '24px' }}>
                    <AlertCircle size={32} color="var(--warning)" style={{ marginBottom: '8px' }} />
                    <p style={{ color: 'var(--text-secondary)', fontSize: '14px', textAlign: 'center', marginBottom: '12px' }}>
                      No matching template found for this file.
                    </p>
                    <p style={{ color: 'var(--text-muted)', fontSize: '12px', marginBottom: '16px' }}>
                      This file is in a _modN folder, meaning it differs from the template.
                      You can create a new template from this file, or apply it as a fixed template to the entire group.
                    </p>
                    <div style={{ display: 'flex', gap: '12px' }}>
                      <button className="btn btn-primary" onClick={handleCreateTemplate}>
                        <Plus size={14} />
                        Create Template from this file
                      </button>
                    </div>
                  </div>
                ) : (
                  <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                    <p style={{ color: 'var(--text-muted)' }}>No diff data available.</p>
                  </div>
                )}
              </div>
            )}
          </>
        ) : (
          <div style={{ flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', padding: '24px' }}>
            <GitCompare size={32} color="var(--text-muted)" style={{ marginBottom: '8px' }} />
            <p style={{ color: 'var(--text-muted)', fontSize: '14px' }}>Select a DXF file to view the visual diff</p>
            <p style={{ color: 'var(--text-muted)', fontSize: '12px', marginTop: '4px' }}>
              Click on a .dxf file for side-by-side template comparison · Click on a .log file to read the detailed analysis
            </p>
            <button className="btn btn-secondary" style={{ marginTop: '16px' }} onClick={loadGroups}>
              <Layers size={14} />
              View Template Groups
            </button>
          </div>
        )}
      </div>

      {/* External editor message */}
      {externalMsg && (
        <div style={{ position: 'fixed', bottom: 60, right: 16, padding: '8px 12px', background: 'var(--bg-elevated)', border: '1px solid var(--accent)', borderRadius: 6, fontSize: 12, color: 'var(--accent)', zIndex: 1000 }}>
          {externalMsg}
        </div>
      )}

      {/* Right-click context menu */}
      {contextMenu && (
        <div
          style={{
            position: 'fixed', top: contextMenu.y, left: contextMenu.x,
            background: 'var(--bg-secondary)', border: '1px solid var(--border)',
            borderRadius: 6, padding: '4px 0', zIndex: 2000,
            boxShadow: '0 4px 12px rgba(0,0,0,0.4)', minWidth: 220,
          }}
          onClick={(e) => e.stopPropagation()}
        >
          <div
            onClick={() => { handleOpenExternal(contextMenu.filePath); }}
            style={{ padding: '6px 12px', cursor: 'pointer', fontSize: 12, display: 'flex', alignItems: 'center', gap: '8px', color: 'var(--text-primary)' }}
            onMouseEnter={(e) => (e.currentTarget.style.background = 'rgba(0,138,0,0.1)')}
            onMouseLeave={(e) => (e.currentTarget.style.background = 'transparent')}
          >
            <ExternalLink size={14} color="var(--accent)" />
            Open in external editor
          </div>
          <div style={{ borderTop: '1px solid var(--border)', margin: '4px 0' }} />
          <div
            onClick={() => { handleFileClick(contextMenu.filePath, ''); setContextMenu(null); }}
            style={{ padding: '6px 12px', cursor: 'pointer', fontSize: 12, display: 'flex', alignItems: 'center', gap: '8px', color: 'var(--text-primary)' }}
            onMouseEnter={(e) => (e.currentTarget.style.background = 'rgba(0,138,0,0.1)')}
            onMouseLeave={(e) => (e.currentTarget.style.background = 'transparent')}
          >
            <GitCompare size={14} color="var(--accent)" />
            View diff with template
          </div>
          {contextMenu.filePath.includes('_mod') && (
            <div
              onClick={() => {
                const parts = contextMenu.filePath.split('\\\\');
                const modFolder = parts.find(p => p.includes('_mod'));
                if (modFolder) {
                  const groupName = modFolder.split('_mod')[0];
                  handleApplyTemplate(groupName, contextMenu.filePath);
                }
                setContextMenu(null);
              }}
              style={{ padding: '6px 12px', cursor: 'pointer', fontSize: 12, display: 'flex', alignItems: 'center', gap: '8px', color: 'var(--text-primary)' }}
              onMouseEnter={(e) => (e.currentTarget.style.background = 'rgba(0,138,0,0.1)')}
              onMouseLeave={(e) => (e.currentTarget.style.background = 'transparent')}
            >
              <Wrench size={14} color="var(--accent)" />
              Apply as fixed template to group
            </div>
          )}
        </div>
      )}
    </div>
  );
}
function TreeView({ node, level, selectedFile, onFileClick, onContextMenu }: {
  node: TreeNode;
  level: number;
  selectedFile: string | null;
  onFileClick: (filePath: string, folderPath: string) => void;
  onContextMenu: (e: React.MouseEvent, filePath: string) => void;
}) {
  // Auto-expand root (level 0) and template folders (level 1) so _modN subfolders are visible
  const [expanded, setExpanded] = useState(level <= 1);

  const isDXF = !node.is_dir && node.name.toLowerCase().endsWith('.dxf');
  const isLog = !node.is_dir && node.name.toLowerCase().endsWith('.log');

  if (!node.is_dir) {
    return (
      <div
        onClick={() => (isDXF || isLog) && onFileClick(node.path, '')}
        onContextMenu={(e) => isDXF && onContextMenu(e, node.path)}
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: '6px',
          padding: '3px 8px',
          marginLeft: level * 16,
          cursor: (isDXF || isLog) ? 'pointer' : 'default',
          fontSize: 12,
          fontFamily: 'var(--font-mono)',
          color: selectedFile === node.path ? 'var(--accent)' : isDXF || isLog ? 'var(--text-primary)' : 'var(--text-muted)',
          background: selectedFile === node.path ? 'rgba(0,138,0,0.08)' : 'transparent',
          borderRadius: 4,
        }}
      >
        {isDXF ? <FileText size={12} color="var(--accent)" /> : isLog ? <FileText size={12} color="var(--info)" /> : <FileText size={12} color="var(--text-muted)" />}
        <span>{node.name}</span>
        {isDXF && <span style={{ fontSize: 10, color: 'var(--text-muted)' }}>📐</span>}
        {isLog && <span style={{ fontSize: 10, color: 'var(--text-muted)' }}>📝</span>}
        <span style={{ fontSize: 10, color: 'var(--text-muted)', marginLeft: 'auto' }}>{formatSize(node.size)}</span>
      </div>
    );
  }

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
            <TreeView key={i} node={child} level={level + 1} selectedFile={selectedFile} onFileClick={onFileClick} onContextMenu={onContextMenu} />
          ))}
        </div>
      )}
    </div>
  );
}

// entityKey generates a unique key for an entity (used for diff set lookup)
function entityKey(e: DiffEntity): string {
  const coords = (e.coords || []).map(c => c.toFixed(2)).join(',');
  return `${e.type}:${e.block_name || ''}:${coords}`;
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}