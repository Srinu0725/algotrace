import { useMemo, useRef, useState } from 'react'
import type { BinaryTreeVisualization } from '../../types/visualization'

export default function BinaryTreeVisualizer({ data }: { data: BinaryTreeVisualization }) {
  const [zoom, setZoom] = useState(1)
  const viewport = useRef<HTMLDivElement>(null)
  const layout = useMemo(() => {
    const nodes = new Map(data.nodes.map(n => [n.id, n]))
    const positions = new Map<string, { x: number; y: number }>()
    const seen = new Set<string>()
    let column = 0
    let maxDepth = 0
    function visit(id: string | null, depth: number) {
      if (!id || seen.has(id) || !nodes.has(id)) return
      seen.add(id)
      const n = nodes.get(id)!
      visit(n.left, depth + 1)
      positions.set(id, { x: 65 + column++ * 100, y: 65 + depth * 100 })
      maxDepth = Math.max(maxDepth, depth)
      visit(n.right, depth + 1)
    }
    visit(data.root, 0)
    for (const id of nodes.keys()) visit(id, 0)
    return { positions, width: Math.max(300, column * 100 + 30), height: maxDepth * 100 + 130 }
  }, [data.nodes, data.root])
  const fit = () => setZoom(Math.max(0.1, Math.min(1, (viewport.current?.clientWidth ?? layout.width) / layout.width)))
  return <section className="trace-card">
    <div className="recursion-header">
      <strong>{data.nodes.length} nodes</strong>
      <div style={{ display: 'flex', gap: 8 }}>
        <button onClick={() => setZoom(z => Math.max(0.1, z - 0.15))}>−</button>
        <button onClick={fit}>Fit</button>
        <button onClick={() => setZoom(z => Math.min(2, z + 0.15))}>+</button>
        <button onClick={() => setZoom(1)}>Reset</button>
      </div>
    </div>
    {data.irregular && <p>Non-tree references detected (cycle or shared child).</p>}
    {data.incomplete && <p>Snapshot incomplete; some child references may be unavailable.</p>}
    <div ref={viewport} style={{ overflow: 'auto', maxHeight: 420 }}>
      <svg width={layout.width * zoom} height={layout.height * zoom} viewBox={`0 0 ${layout.width} ${layout.height}`} role="img" aria-label="Binary tree">
        {data.nodes.flatMap(n => (['left', 'right'] as const).map(side => {
          const a = layout.positions.get(n.id)!
          const target = n[side]
          if (!target) return null
          const b = layout.positions.get(target)
          return <g key={`${n.id}-${side}`}>
            {b ? (target === n.id ? <path d={`M${a.x+23} ${a.y} c55 -65 55 65 0 0`} fill="none" stroke="#94a3b8" strokeWidth="2"/> : <line x1={a.x} y1={a.y} x2={b.x} y2={b.y} stroke="#94a3b8" strokeWidth="2"/> ) : <text x={a.x + (side === 'left' ? -35 : 35)} y={a.y+40} fill="#fbbf24" fontSize="12">?</text>}
            {b && <text x={(a.x+b.x)/2+6} y={(a.y+b.y)/2-5} fill="#94a3b8" fontSize="11">{side === 'left' ? 'L' : 'R'}</text>}
          </g>
        }))}
        {data.nodes.map(n => {
          const p = layout.positions.get(n.id)!
          const value = JSON.stringify(n.value)
          const aliases = Object.keys(data.pointers).filter(k => data.pointers[k] === n.id).join(', ')
          return <g key={n.id}>
            <title>{`${n.id}: ${value}; left: ${n.left ?? 'null'}; right: ${n.right ?? 'null'}; ${aliases}`}</title>
            <circle cx={p.x} cy={p.y} r="24" fill={n.changed ? '#854d0e' : '#17263d'} stroke={n.changed ? '#fbbf24' : '#60a5fa'} strokeWidth="2"/>
            <text x={p.x} y={p.y+4} textAnchor="middle" fill="#f1f5f9" fontSize="12">{value.length > 8 ? value.slice(0,7)+'…' : value}</text>
            <text x={p.x} y={p.y+40} textAnchor="middle" fill="#94a3b8" fontSize="10">{n.id}</text>
            <text x={p.x} y={p.y-32} textAnchor="middle" fill="#94a3b8" fontSize="10">{aliases.slice(0,16)}</text>
          </g>
        })}
      </svg>
    </div>
    <small>Amber: new or modified node. L/R: child references. Hover for details.</small>
  </section>
}
