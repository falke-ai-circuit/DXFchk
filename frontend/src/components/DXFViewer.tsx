import { useRef, useState, useEffect, useCallback } from 'react';
import { ZoomIn, ZoomOut, Maximize, Grid } from 'lucide-react';
import type { DiffEntity, DiffAttrib } from '../api';

// AutoCAD ACI Color Index (0-255)
// 0=ByBlock, 256=ByLayer (handled separately), 7=white/black depending on bg
const ACI_COLORS: Record<number, string> = {
  0: '#000000', 1: '#ff0000', 2: '#ffff00', 3: '#00ff00', 4: '#00ffff',
  5: '#0000ff', 6: '#ff00ff', 7: '#ffffff', 8: '#808080', 9: '#c0c0c0',
  10: '#ff0000', 11: '#ff4040', 12: '#ff8080', 13: '#ffc0c0', 14: '#e0e0e0',
  15: '#b0b0b0',
  // Common colors
  30: '#ff7f00', 40: '#ffbf00', 50: '#ffff00', 60: '#bfff00', 70: '#80ff00',
  80: '#40ff00', 90: '#00ff00', 100: '#00ff40', 110: '#00ff80', 120: '#00ffbf',
  130: '#00ffff', 140: '#00bfff', 150: '#0080ff', 160: '#0040ff', 170: '#0000ff',
  180: '#4000ff', 190: '#8000ff', 200: '#bf00ff', 210: '#ff00ff', 220: '#ff00bf',
  230: '#ff0080', 240: '#ff0040', 250: '#808080', 251: '#909090', 252: '#a0a0a0',
  253: '#b0b0b0', 254: '#c0c0c0', 255: '#e0e0e0',
};

function aciToColor(aci: number): string {
  if (ACI_COLORS[aci]) return ACI_COLORS[aci];
  if (aci >= 10 && aci <= 249) {
    const hue = ((aci - 10) / 240) * 360;
    return `hsl(${hue}, 100%, 50%)`;
  }
  return '#00ff41';
}

function resolveEntityColor(entity: DiffEntity, layerColors: Record<string, number>): string {
  if (entity.color > 0 && entity.color < 256) {
    return aciToColor(entity.color);
  }
  if (entity.color === 256 || entity.color === 0) {
    const layerAci = layerColors[entity.layer?.toUpperCase()] ?? layerColors[entity.layer];
    if (layerAci && layerAci > 0 && layerAci < 256) {
      return aciToColor(layerAci);
    }
  }
  return '#00ff41';
}

// Convert bulge to SVG arc parameters
function bulgeToArc(x1: number, y1: number, x2: number, y2: number, bulge: number) {
  if (bulge === 0) return null;
  const dx = x2 - x1;
  const dy = y2 - y1;
  const chord = Math.sqrt(dx * dx + dy * dy);
  if (chord === 0) return null;
  const theta = 4 * Math.atan(Math.abs(bulge));
  const radius = chord / (2 * Math.sin(theta / 2));
  const largeArc = Math.abs(bulge) > 1 ? 1 : 0;
  const sweep = bulge > 0 ? 0 : 1;
  return { rx: radius, ry: radius, largeArc, sweep, x: x2, y: y2 };
}

// Build SVG path for LWPOLYLINE/POLYLINE with bulge support
function polylineToPath(entity: DiffEntity): string {
  if (!entity.coords_2d || entity.coords_2d.length < 2) return '';
  const pts = entity.coords_2d;
  const bulges = entity.bulges || [];
  let path = `M ${pts[0][0]} ${pts[0][1]}`;

  for (let i = 0; i < pts.length - 1; i++) {
    const bulge = bulges[i] || 0;
    if (bulge === 0) {
      path += ` L ${pts[i + 1][0]} ${pts[i + 1][1]}`;
    } else {
      const arc = bulgeToArc(pts[i][0], pts[i][1], pts[i + 1][0], pts[i + 1][1], bulge);
      if (arc) {
        path += ` A ${arc.rx} ${arc.ry} 0 ${arc.largeArc} ${arc.sweep} ${arc.x} ${arc.y}`;
      } else {
        path += ` L ${pts[i + 1][0]} ${pts[i + 1][1]}`;
      }
    }
  }

  if (entity.closed && pts.length > 2) {
    const lastIdx = pts.length - 1;
    const lastBulge = bulges[lastIdx] || 0;
    if (lastBulge === 0) {
      path += ` L ${pts[0][0]} ${pts[0][1]}`;
    } else {
      const arc = bulgeToArc(pts[lastIdx][0], pts[lastIdx][1], pts[0][0], pts[0][1], lastBulge);
      if (arc) {
        path += ` A ${arc.rx} ${arc.ry} 0 ${arc.largeArc} ${arc.sweep} ${arc.x} ${arc.y}`;
      } else {
        path += ` L ${pts[0][0]} ${pts[0][1]}`;
      }
    }
    path += ' Z';
  }

  return path;
}

// Text alignment maps
const HALIGN_MAP = ['start', 'middle', 'end', 'start', 'middle', 'start'];
const VALIGN_MAP = ['alphabetic', 'text-after-edge', 'middle', 'text-before-edge'];

interface DXFViewerProps {
  entities: DiffEntity[];
  boundingBox: [number, number, number, number];
  layers?: string[];
  layerColors?: Record<string, number>;
  highlightAdded?: boolean;
  addedSet?: Set<string>;
  removedSet?: Set<string>;
  showInfoPanel?: boolean;
}

export default function DXFViewer({
  entities = [],
  boundingBox,
  layers = [],
  layerColors = {},
  highlightAdded = false,
  addedSet,
  removedSet,
  showInfoPanel = true,
}: DXFViewerProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [size, setSize] = useState({ w: 800, h: 600 });
  const [pan, setPan] = useState({ x: 0, y: 0 });
  const [zoom, setZoom] = useState(1);
  const [isDragging, setIsDragging] = useState(false);
  const [dragStart, setDragStart] = useState({ x: 0, y: 0 });
  const [showGrid, setShowGrid] = useState(true);
  const [visibleLayers, setVisibleLayers] = useState<Set<string>>(new Set());
  const [hoveredEntity, setHoveredEntity] = useState<number | null>(null);

  useEffect(() => {
    setVisibleLayers(new Set(layers));
  }, [layers.join(',')]);

  useEffect(() => {
    if (!containerRef.current) return;
    const observer = new ResizeObserver(entries => {
      for (const entry of entries) {
        const { width, height } = entry.contentRect;
        setSize({ w: width, h: height });
      }
    });
    observer.observe(containerRef.current);
    return () => observer.disconnect();
  }, []);

  // Compute initial zoom-to-fit
  // With per-entity Y negation (no group Y-flip), screen = world * zoom + pan
  const fitToView = useCallback(() => {
    const [minX, minY, maxX, maxY] = boundingBox;
    const bw = maxX - minX || 100;
    const bh = maxY - minY || 100;
    const padding = 60;
    const scaleX = (size.w - padding * 2) / bw;
    const scaleY = (size.h - padding * 2) / bh;
    const fitZoom = Math.min(scaleX, scaleY);
    setZoom(fitZoom);
    const cx = (minX + maxX) / 2;
    const cy = (minY + maxY) / 2;
    // screen = world * zoom + pan → pan = screenCenter - worldCenter * zoom
    setPan({
      x: size.w / 2 - cx * fitZoom,
      y: size.h / 2 - (-cy) * fitZoom, // entities negate Y, so world center is -cy in screen
    });
  }, [boundingBox, size]);

  useEffect(() => {
    if (entities.length > 0 && size.w > 0) {
      fitToView();
    }
  }, [entities.length, size.w, size.h]);

  // Mouse handlers
  const handleMouseDown = (e: React.MouseEvent) => {
    if (e.button === 0 || e.button === 1) {
      setIsDragging(true);
      setDragStart({ x: e.clientX - pan.x, y: e.clientY - pan.y });
    }
  };
  const handleMouseMove = (e: React.MouseEvent) => {
    if (isDragging) {
      setPan({ x: e.clientX - dragStart.x, y: e.clientY - dragStart.y });
    }
  };
  const handleMouseUp = () => setIsDragging(false);

  const handleWheel = (e: React.WheelEvent) => {
    e.preventDefault();
    const factor = e.deltaY > 0 ? 0.9 : 1.1;
    const newZoom = Math.max(0.01, Math.min(100, zoom * factor));
    const rect = containerRef.current?.getBoundingClientRect();
    if (rect) {
      const mx = e.clientX - rect.left;
      const my = e.clientY - rect.top;
      const wx = (mx - pan.x) / zoom;
      const wy = (my - pan.y) / zoom;
      setPan({ x: mx - wx * newZoom, y: my - wy * newZoom });
    }
    setZoom(newZoom);
  };

  const toggleLayer = (layer: string) => {
    setVisibleLayers(prev => {
      const next = new Set(prev);
      if (next.has(layer)) next.delete(layer);
      else next.add(layer);
      return next;
    });
  };

  const isEntityVisible = (e: DiffEntity) => {
    if (!e.layer) return true;
    if (visibleLayers.size === 0) return true; // empty set = all visible
    return visibleLayers.has(e.layer);
  };

  const entityColor = (e: DiffEntity) => {
    if (highlightAdded && addedSet && addedSet.has(entityKey(e))) return '#ff0000';
    if (highlightAdded && removedSet && removedSet.has(entityKey(e))) return '#ffaa00';
    return resolveEntityColor(e, layerColors);
  };

  // Grid lines
  const gridSize = 20 * zoom;
  const gridLines: React.ReactNode[] = [];
  if (showGrid && gridSize > 4) {
    const startX = ((pan.x % gridSize) + gridSize) % gridSize - gridSize;
    const startY = ((pan.y % gridSize) + gridSize) % gridSize - gridSize;
    for (let x = startX; x < size.w; x += gridSize) {
      gridLines.push(<line key={`gx${x}`} x1={x} y1={0} x2={x} y2={size.h} stroke="var(--border)" strokeWidth="0.5" opacity="0.2" />);
    }
    for (let y = startY; y < size.h; y += gridSize) {
      gridLines.push(<line key={`gy${y}`} x1={0} y1={y} x2={size.w} y2={y} stroke="var(--border)" strokeWidth="0.5" opacity="0.2" />);
    }
  }

  // Render an ATTRIB text label — at WORLD coordinates (NOT inside the INSERT transform)
  const renderAttrib = (att: DiffAttrib, idx: number): React.ReactNode => {
    const anchor = HALIGN_MAP[att.h_align] || 'start';
    const baseline = VALIGN_MAP[att.v_align] || 'alphabetic';
    const rot = att.rotation || 0;
    // Minimum font size in screen pixels — text must be readable
    const screenFontSize = att.height * zoom;
    const minScreenSize = 7; // 7px minimum readable
    const actualFontSize = Math.max(att.height, minScreenSize / zoom);
    return (
      <text key={`att${idx}`} x={att.x} y={-att.y} fill="#ffffff" fontSize={actualFontSize}
        fontFamily="sans-serif" textAnchor={anchor as any}
        dominantBaseline={baseline as any}
        transform={rot ? `rotate(${-rot} ${att.x} ${-att.y})` : undefined}
        opacity={0.9}
      >
        {att.text || att.tag}
      </text>
    );
  };

  // Render a single entity (used both for top-level and block entities)
  const renderEntity = (e: DiffEntity, i: number): React.ReactNode => {
    if (!e || !e.coords) return null;
    const color = entityColor(e);
    const isHovered = hoveredEntity === i;
    const strokeW = isHovered ? 2 : Math.max(0.5, 1.5 / zoom);
    const opacity = isHovered ? 1 : 0.85;

    switch (e.type) {
      case 'line':
        if (e.coords.length >= 4) {
          return (
            <line key={i} x1={e.coords[0]} y1={-e.coords[1]} x2={e.coords[2]} y2={-e.coords[3]}
              stroke={color} strokeWidth={strokeW} opacity={opacity}
              vectorEffect="non-scaling-stroke"
              onMouseEnter={() => setHoveredEntity(i)} onMouseLeave={() => setHoveredEntity(null)}
            />
          );
        }
        return null;

      case 'lwpolyline':
      case 'polyline': {
        if (!e.coords_2d || e.coords_2d.length === 0) return null;
        const pts = e.coords_2d.map(p => [p[0], -p[1]]);
        const path = polylineToPath({ ...e, coords_2d: pts });
        return path ? (
          <path key={i} d={path} fill="none" stroke={color} strokeWidth={strokeW} opacity={opacity}
            vectorEffect="non-scaling-stroke"
            onMouseEnter={() => setHoveredEntity(i)} onMouseLeave={() => setHoveredEntity(null)}
          />
        ) : null;
      }

      case 'arc': {
        if (e.coords.length >= 4) {
          let pathD = `M ${e.coords[0]} ${-e.coords[1]}`;
          for (let j = 2; j < e.coords.length; j += 2) {
            pathD += ` L ${e.coords[j]} ${-e.coords[j + 1]}`;
          }
          return (
            <path key={i} d={pathD} fill="none" stroke={color} strokeWidth={strokeW} opacity={opacity}
              vectorEffect="non-scaling-stroke"
              onMouseEnter={() => setHoveredEntity(i)} onMouseLeave={() => setHoveredEntity(null)}
            />
          );
        }
        return null;
      }

      case 'insert': {
        if (e.coords.length < 2) return null;
        const ix = e.coords[0];
        const iy = -e.coords[1];
        const rot = e.rotation || 0;
        const sx = e.scale_x || 1;
        const sy = e.scale_y || 1;
        const bx = e.block_base_x || 0;
        const by = e.block_base_y || 0;

        const transform = `translate(${ix} ${iy}) rotate(${-rot}) scale(${sx} ${sy}) translate(${-bx} ${by})`;

        return (
          <g key={i}>
            {/* Block geometry inside the INSERT transform */}
            <g transform={transform}
              onMouseEnter={() => setHoveredEntity(i)} onMouseLeave={() => setHoveredEntity(null)}
            >
              {e.block_entities?.map((be, bi) => renderEntity(be, i * 1000 + bi))}
              {(!e.block_entities || e.block_entities.length === 0) && (
                <rect x={-3 / sx} y={-3 / sy} width={6 / sx} height={6 / sy}
                  fill="none" stroke={color} strokeWidth={strokeW} opacity={opacity}
                  vectorEffect="non-scaling-stroke"
                />
              )}
            </g>
            {/* ATTRIB text labels — rendered at WORLD coordinates, OUTSIDE the INSERT transform */}
            {e.attribs?.map((att, ai) => renderAttrib(att, i * 10000 + ai))}
          </g>
        );
      }

      case 'text': {
        if (e.coords.length < 2 || !e.block_name) return null;
        const tx_ = e.coords[0];
        const ty_ = -e.coords[1];
        const rawHeight = e.text_height || 2.5;
        const rot = e.rotation || 0;
        const hAlign = e.h_align || 0;
        const vAlign = e.v_align || 0;
        const anchor = HALIGN_MAP[hAlign] || 'start';
        const baseline = VALIGN_MAP[vAlign] || 'alphabetic';
        // Minimum readable font: 7px on screen
        const actualFontSize = Math.max(rawHeight, 7 / zoom);
        return (
          <text key={i} x={tx_} y={ty_} fill={color} fontSize={actualFontSize}
            fontFamily="sans-serif" textAnchor={anchor as any}
            dominantBaseline={baseline as any}
            transform={rot ? `rotate(${-rot} ${tx_} ${ty_})` : undefined}
            opacity={opacity}
            onMouseEnter={() => setHoveredEntity(i)} onMouseLeave={() => setHoveredEntity(null)}
          >
            {e.block_name}
          </text>
        );
      }

      case 'circle': {
        if (e.coords.length < 3) return null;
        return (
          <circle key={i} cx={e.coords[0]} cy={-e.coords[1]} r={e.coords[2]}
            fill="none" stroke={color} strokeWidth={strokeW} opacity={opacity}
            vectorEffect="non-scaling-stroke"
            onMouseEnter={() => setHoveredEntity(i)} onMouseLeave={() => setHoveredEntity(null)}
          />
        );
      }

      case 'point': {
        if (e.coords.length < 2) return null;
        return (
          <circle key={i} cx={e.coords[0]} cy={-e.coords[1]} r={0.5}
            fill={color} stroke="none" opacity={opacity}
            onMouseEnter={() => setHoveredEntity(i)} onMouseLeave={() => setHoveredEntity(null)}
          />
        );
      }

      default:
        return null;
    }
  };

  // SVG group transform: world → screen (no Y-flip; entities negate Y individually)
  const groupTransform = `translate(${pan.x} ${pan.y}) scale(${zoom})`;

  return (
    <div style={{ flex: 1, display: 'flex', overflow: 'hidden', position: 'relative', backgroundColor: 'var(--bg-primary)' }} ref={containerRef}>
      <svg
        width={size.w}
        height={size.h}
        style={{ display: 'block', cursor: isDragging ? 'grabbing' : 'grab' }}
        onMouseDown={handleMouseDown}
        onMouseMove={handleMouseMove}
        onMouseUp={handleMouseUp}
        onMouseLeave={handleMouseUp}
        onWheel={handleWheel}
      >
        {gridLines}

        <g transform={groupTransform}>
          {/* Origin axes */}
          <line x1={-10000} y1={0} x2={10000} y2={0} stroke="#444" strokeWidth={1 / zoom} opacity={0.15} />
          <line x1={0} y1={-10000} x2={0} y2={10000} stroke="#444" strokeWidth={1 / zoom} opacity={0.15} />

          {/* Entities in world space */}
          {entities.map((e, i) => {
            if (!isEntityVisible(e)) return null;
            return renderEntity(e, i);
          })}
        </g>
      </svg>

      {/* Toolbar */}
      <div style={{ position: 'absolute', top: 8, left: 8, display: 'flex', gap: 4, zIndex: 10 }}>
        <button className="btn btn-ghost" style={{ padding: '4px 8px' }} onClick={() => setZoom(z => Math.max(0.01, z * 0.8))} title="Zoom out">
          <ZoomOut size={14} />
        </button>
        <button className="btn btn-ghost" style={{ padding: '4px 8px' }} onClick={() => setZoom(z => z * 1.25)} title="Zoom in">
          <ZoomIn size={14} />
        </button>
        <button className="btn btn-ghost" style={{ padding: '4px 8px' }} onClick={fitToView} title="Zoom to fit">
          <Maximize size={14} />
        </button>
        <button className="btn btn-ghost" style={{ padding: '4px 8px' }} onClick={() => setShowGrid(g => !g)} title="Toggle grid">
          <Grid size={14} />
        </button>
        <span style={{ fontSize: 10, color: 'var(--text-muted)', padding: '2px 6px', alignSelf: 'center' }}>
          {(zoom * 100).toFixed(0)}%
        </span>
      </div>

      {/* Layer Panel */}
      {showInfoPanel && layers.length > 0 && (
        <div style={{
          position: 'absolute', top: 8, right: 8, maxWidth: 200, maxHeight: '60%',
          overflowY: 'auto', backgroundColor: 'var(--bg-elevated)', border: '1px solid var(--border)',
          borderRadius: 6, padding: '8px', zIndex: 10,
        }}>
          <div style={{ fontSize: 11, fontWeight: 600, marginBottom: 6, color: 'var(--text-secondary)' }}>
            Layers ({layers.length})
          </div>
          {layers.map(layer => {
            const aci = layerColors[layer?.toUpperCase()] ?? layerColors[layer] ?? 7;
            const color = aci > 0 && aci < 256 ? aciToColor(aci) : '#00ff41';
            return (
              <div key={layer} onClick={() => toggleLayer(layer)} style={{
                display: 'flex', alignItems: 'center', gap: 6, padding: '2px 0',
                cursor: 'pointer', opacity: visibleLayers.has(layer) ? 1 : 0.4, fontSize: 11,
              }}>
                <div style={{ width: 14, height: 2, backgroundColor: color, flexShrink: 0 }} />
                <span style={{ fontFamily: 'var(--font-mono)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {layer}
                </span>
              </div>
            );
          })}
        </div>
      )}

      {/* Entity info on hover */}
      {hoveredEntity !== null && entities[hoveredEntity] && (
        <div style={{
          position: 'absolute', bottom: 8, left: 8, zIndex: 10,
          backgroundColor: 'var(--bg-elevated)', border: '1px solid var(--border)',
          borderRadius: 6, padding: '6px 10px', fontSize: 11, fontFamily: 'var(--font-mono)',
        }}>
          <span style={{ color: 'var(--accent)' }}>{entities[hoveredEntity].type}</span>
          {entities[hoveredEntity].layer && <span style={{ color: 'var(--text-muted)' }}> | {entities[hoveredEntity].layer}</span>}
          {entities[hoveredEntity].block_name && entities[hoveredEntity].type === 'text' && (
            <span style={{ color: 'var(--text-primary)' }}> | "{entities[hoveredEntity].block_name}"</span>
          )}
          {entities[hoveredEntity].block_name && entities[hoveredEntity].type === 'insert' && (
            <span style={{ color: 'var(--text-primary)' }}> | [{entities[hoveredEntity].block_name}]</span>
          )}
          {entities[hoveredEntity].attribs && entities[hoveredEntity].attribs.length > 0 && (
            <span style={{ color: 'var(--text-muted)' }}> | {entities[hoveredEntity].attribs.length} attrs</span>
          )}
          {entities[hoveredEntity].coords && entities[hoveredEntity].coords.length >= 2 && (
            <span style={{ color: 'var(--text-muted)' }}>
              {' '}({entities[hoveredEntity].coords[0].toFixed(2)}, {entities[hoveredEntity].coords[1].toFixed(2)})
            </span>
          )}
        </div>
      )}

      {entities.length === 0 && (
        <div style={{
          position: 'absolute', top: '50%', left: '50%', transform: 'translate(-50%, -50%)',
          color: 'var(--text-muted)', fontSize: 13,
        }}>
          No drawable entities found in this file
        </div>
      )}
    </div>
  );
}

function entityKey(e: DiffEntity): string {
  return `${e.type}:${e.block_name}:${(e.coords || []).map(c => c.toFixed(2)).join(',')}`;
}