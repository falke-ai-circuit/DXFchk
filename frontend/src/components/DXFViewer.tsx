import { useRef, useState, useEffect, useCallback } from 'react';
import { ZoomIn, ZoomOut, Maximize, Grid } from 'lucide-react';
import type { DiffEntity } from '../api';

// CAD-like color palette for layers (AutoCAD ACI colors)
const CAD_COLORS = [
  '#ff0000', '#ffff00', '#00ff00', '#00ffff', '#0000ff', '#ff00ff',
  '#ffffff', '#808080', '#ff8c00', '#8b4513', '#90ee90', '#add8e6',
  '#ffa0a0', '#a0ffa0', '#a0a0ff', '#ffffa0', '#ffa0ff', '#a0ffff',
  '#e0e0e0', '#c0c0c0', '#80ff80', '#80c0ff', '#ff80c0', '#c0ff80',
];

function getLayerColor(layer: string, layerColorMap: Record<string, string>): string {
  if (layerColorMap[layer]) return layerColorMap[layer];
  return '#00ff41'; // default matrix green
}

interface DXFViewerProps {
  entities: DiffEntity[];
  boundingBox: [number, number, number, number];
  layers?: string[];
  highlightAdded?: boolean;
  addedSet?: Set<string>;
  showInfoPanel?: boolean;
}

export default function DXFViewer({
  entities,
  boundingBox,
  layers = [],
  highlightAdded = false,
  addedSet,
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
  const [layerColorMap, setLayerColorMap] = useState<Record<string, string>>({});

  // Initialize visible layers and color map
  useEffect(() => {
    const map: Record<string, string> = {};
    layers.forEach((layer, i) => {
      map[layer] = CAD_COLORS[i % CAD_COLORS.length];
    });
    setLayerColorMap(map);
    setVisibleLayers(new Set(layers));
  }, [layers.join(',')]);

  // Resize observer
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
  const fitToView = useCallback(() => {
    const [minX, minY, maxX, maxY] = boundingBox;
    const bw = maxX - minX || 100;
    const bh = maxY - minY || 100;
    const padding = 60;
    const scaleX = (size.w - padding * 2) / bw;
    const scaleY = (size.h - padding * 2) / bh;
    const fitZoom = Math.min(scaleX, scaleY);
    setZoom(fitZoom);
    // Center the drawing
    const cx = (minX + maxX) / 2;
    const cy = (minY + maxY) / 2;
    setPan({
      x: size.w / 2 - cx * fitZoom,
      y: size.h / 2 + cy * fitZoom,
    });
  }, [boundingBox, size]);

  // Fit on mount and when entities change
  useEffect(() => {
    if (entities.length > 0 && size.w > 0) {
      fitToView();
    }
  }, [entities.length, size.w, size.h]);

  // Coordinate transform
  const tx = useCallback((x: number) => x * zoom + pan.x, [zoom, pan]);
  const ty = useCallback((y: number) => -y * zoom + pan.y, [zoom, pan]);

  // Mouse handlers
  const handleMouseDown = (e: React.MouseEvent) => {
    if (e.button === 0 || e.button === 1) {
      setIsDragging(true);
      setDragStart({ x: e.clientX - pan.x, y: e.clientY - pan.y });
    }
  };

  const handleMouseMove = (e: React.MouseEvent) => {
    if (isDragging) {
      setPan({
        x: e.clientX - dragStart.x,
        y: e.clientY - dragStart.y,
      });
    }
  };

  const handleMouseUp = () => setIsDragging(false);

  const handleWheel = (e: React.WheelEvent) => {
    e.preventDefault();
    const factor = e.deltaY > 0 ? 0.9 : 1.1;
    const newZoom = Math.max(0.01, Math.min(100, zoom * factor));
    // Zoom toward mouse position
    const rect = containerRef.current?.getBoundingClientRect();
    if (rect) {
      const mx = e.clientX - rect.left;
      const my = e.clientY - rect.top;
      // World coordinates under mouse before zoom
      const wx = (mx - pan.x) / zoom;
      const wy = -(my - pan.y) / zoom;
      setPan({
        x: mx - wx * newZoom,
        y: my + wy * newZoom,
      });
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
    return visibleLayers.has(e.layer);
  };

  const entityColor = (e: DiffEntity) => {
    if (highlightAdded && addedSet && addedSet.has(entityKey(e))) return '#ff0000';
    return getLayerColor(e.layer, layerColorMap);
  };

  // Grid lines
  const gridSize = 20 * zoom;
  const gridLines: React.ReactNode[] = [];
  if (showGrid && gridSize > 4) {
    const startX = (pan.x % gridSize) - gridSize;
    const startY = (pan.y % gridSize) - gridSize;
    for (let x = startX; x < size.w; x += gridSize) {
      gridLines.push(<line key={`gx${x}`} x1={x} y1={0} x2={x} y2={size.h} stroke="var(--border)" strokeWidth="0.5" opacity="0.3" />);
    }
    for (let y = startY; y < size.h; y += gridSize) {
      gridLines.push(<line key={`gy${y}`} x1={0} y1={y} x2={size.w} y2={y} stroke="var(--border)" strokeWidth="0.5" opacity="0.3" />);
    }
  }

  return (
    <div style={{ flex: 1, display: 'flex', overflow: 'hidden', position: 'relative', backgroundColor: 'var(--bg-primary)' }} ref={containerRef}>
      {/* SVG Canvas */}
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
        {/* Grid */}
        {gridLines}

        {/* Origin axes */}
        <line x1={tx(-10000)} y1={ty(0)} x2={tx(10000)} y2={ty(0)} stroke="#444" strokeWidth="0.5" opacity="0.2" />
        <line x1={tx(0)} y1={ty(-10000)} x2={tx(0)} y2={ty(10000)} stroke="#444" strokeWidth="0.5" opacity="0.2" />

        {/* Entities */}
        {entities.map((e, i) => {
          if (!isEntityVisible(e)) return null;
          const color = entityColor(e);
          const isHovered = hoveredEntity === i;
          const strokeWidth = isHovered ? 2 : Math.max(0.5, 1 / zoom * 2);
          const opacity = isHovered ? 1 : 0.8;
          const fontWeight = isHovered ? 'bold' : 'normal';

          switch (e.type) {
            case 'line':
              if (e.coords.length >= 4) {
                return (
                  <line key={i}
                    x1={tx(e.coords[0])} y1={ty(e.coords[1])}
                    x2={tx(e.coords[2])} y2={ty(e.coords[3])}
                    stroke={color} strokeWidth={strokeWidth} opacity={opacity}
                    onMouseEnter={() => setHoveredEntity(i)}
                    onMouseLeave={() => setHoveredEntity(null)}
                  />
                );
              }
              return null;

            case 'lwpolyline':
            case 'polyline':
            case 'arc':
              if (e.coords_2d.length >= 2) {
                const points = e.coords_2d.map(p => `${tx(p[0])},${ty(p[1])}`).join(' ');
                return (
                  <polyline key={i} points={points} fill="none"
                    stroke={color} strokeWidth={strokeWidth} opacity={opacity}
                    onMouseEnter={() => setHoveredEntity(i)}
                    onMouseLeave={() => setHoveredEntity(null)}
                  />
                );
              }
              return null;

            case 'insert':
              if (e.coords.length >= 2) {
                const s = Math.max(4, 8 / zoom * 2);
                return (
                  <g key={i}
                    onMouseEnter={() => setHoveredEntity(i)}
                    onMouseLeave={() => setHoveredEntity(null)}
                  >
                    <rect x={tx(e.coords[0]) - s/2} y={ty(e.coords[1]) - s/2}
                      width={s} height={s} fill="none" stroke={color} strokeWidth={strokeWidth} opacity={opacity} />
                    {/* Block name label at high zoom */}
                    {zoom > 5 && e.block_name && (
                      <text x={tx(e.coords[0]) + s} y={ty(e.coords[1]) + 2}
                        fill={color} fontSize={Math.max(8, 10 / zoom * 2)} fontFamily="monospace" opacity={opacity * 0.8}>
                        {e.block_name}
                      </text>
                    )}
                  </g>
                );
              }
              return null;

            case 'text':
              if (e.coords.length >= 2 && e.block_name) {
                const height = e.coords_2d[1]?.[0] || 2.5;
                const rot = e.coords_2d[2]?.[0] || 0;
                const fontSize = Math.max(6, height * zoom * 0.8);
                return (
                  <text key={i}
                    x={tx(e.coords[0])} y={ty(e.coords[1])}
                    fill={color} fontSize={fontSize} fontFamily="sans-serif"
                    opacity={opacity} fontWeight={fontWeight as any}
                    transform={rot ? `rotate(${-rot} ${tx(e.coords[0])} ${ty(e.coords[1])})` : undefined}
                    onMouseEnter={() => setHoveredEntity(i)}
                    onMouseLeave={() => setHoveredEntity(null)}
                  >
                    {e.block_name}
                  </text>
                );
              }
              return null;

            case 'circle':
              if (e.coords.length >= 3) {
                return (
                  <circle key={i}
                    cx={tx(e.coords[0])} cy={ty(e.coords[1])} r={e.coords[2] * zoom}
                    fill="none" stroke={color} strokeWidth={strokeWidth} opacity={opacity}
                    onMouseEnter={() => setHoveredEntity(i)}
                    onMouseLeave={() => setHoveredEntity(null)}
                  />
                );
              }
              return null;

            case 'point':
              if (e.coords.length >= 2) {
                const s = Math.max(2, 3 / zoom * 2);
                return (
                  <circle key={i} cx={tx(e.coords[0])} cy={ty(e.coords[1])} r={s}
                    fill={color} stroke="none" opacity={opacity}
                    onMouseEnter={() => setHoveredEntity(i)}
                    onMouseLeave={() => setHoveredEntity(null)}
                  />
                );
              }
              return null;

            default:
              return null;
          }
        })}
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
          {layers.map(layer => (
            <div key={layer} onClick={() => toggleLayer(layer)} style={{
              display: 'flex', alignItems: 'center', gap: 6, padding: '2px 0',
              cursor: 'pointer', opacity: visibleLayers.has(layer) ? 1 : 0.4, fontSize: 11,
            }}>
              <div style={{ width: 12, height: 2, backgroundColor: getLayerColor(layer, layerColorMap), flexShrink: 0 }} />
              <span style={{ fontFamily: 'var(--font-mono)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                {layer}
              </span>
            </div>
          ))}
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
          {entities[hoveredEntity].coords.length >= 2 && (
            <span style={{ color: 'var(--text-muted)' }}>
              {' '}({entities[hoveredEntity].coords[0].toFixed(2)}, {entities[hoveredEntity].coords[1].toFixed(2)})
            </span>
          )}
        </div>
      )}

      {/* Empty state */}
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
  return `${e.type}:${e.block_name}:${e.coords.map(c => c.toFixed(2)).join(',')}`;
}