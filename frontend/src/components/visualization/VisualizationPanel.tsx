import BinaryTreeVisualizer from './BinaryTreeVisualizer'
import { useMemo } from 'react'

import {
  Workflow,
  LoaderCircle,
} from 'lucide-react'

import { useExecutionStore } from '../../store/executionStore'

import ArrayVisualizer from './ArrayVisualizer'
import MatrixVisualizer from './MatrixVisualizer'
import ObjectVisualizer from './ObjectVisualizer'
import LinkedListVisualizer from './LinkedListVisualizer'
import RecursionTree from './RecursionTree'
import CallStackVisualizer from './CallStackVisualizer'
import AdaptiveLayout from './AdaptiveLayout'

import type {
  VisualizationItem,
} from './AdaptiveLayout'

import { seekFromVisualization } from '../../hooks/usePlayback'

import PlaybackControls from '../playback/PlaybackControls'

import {
  detectStructures,
} from '../../utils/structureDetector'

import {
  buildCallTree,
} from '../../utils/traceTree'

import {
  extractRecursiveTree,
} from '../../utils/recursiveCalls'
import {
  buildVisualizationTimeline,
  getTimelineFrame,
} from '../../utils/visualizationTimeline'
export default function VisualizationPanel() {
  const trace = useExecutionStore(s => s.trace)
  const jobId = useExecutionStore(s => s.jobId)
  const status = useExecutionStore(s => s.status)
  const error = useExecutionStore(s => s.error)

  const currentStep = useExecutionStore(
    s => s.currentStep,
  )

  const isExecuting = useExecutionStore(
    s => s.isExecuting,
  )


  const event = isExecuting
    ? undefined
    : trace?.events[currentStep]

  const previousEvent =
    trace?.events[currentStep - 1]

  const structures = useMemo(
    () => event
      ? detectStructures(event, previousEvent)
      : [],
    [event, previousEvent],
  )

  const callTree = useMemo(
    () => buildCallTree(
      trace?.events ?? [],
      currentStep,
    ),
    [trace, currentStep],
  )

  const fullRecursiveTree = useMemo(() => extractRecursiveTree(
    buildCallTree(trace?.events ?? [], (trace?.events.length ?? 0) - 1),
  ), [trace])

  const recursiveTree = useMemo(
    () => extractRecursiveTree(callTree),
    [callTree],
  )

  // Calculate stable visualization order and
  // the last meaningful activity at this step.
  // This also works when seeking backwards.

  const timeline = useMemo(
  () => buildVisualizationTimeline(
    trace?.events ?? [],
  ),
  [trace],
)

const timelineFrame = getTimelineFrame(
  timeline,
  currentStep,
)

  const retained = useMemo(() => {
    const result = new Map<string, { structure: ReturnType<typeof detectStructures>[number]; step: number }>()
    const seen = new Set(structures.map(item => item.id))
    for (const step of timelineFrame?.retainedSteps ?? []) {
      const snapshot = trace?.events[step]
      if (!snapshot) continue
      for (const structure of detectStructures(snapshot, trace?.events[step - 1])) {
        if (structure.type === 'object' || structure.type === 'recursion' || seen.has(structure.id)) continue
        // Do not duplicate overlapping linked-list components.
        if (structure.type === 'linked-list' && [...structures, ...Array.from(result.values(), item => item.structure)].some(item => item.type === 'linked-list' && item.nodes.some(node => structure.nodes.some(other => other.id === node.id)))) continue
        if (structure.type === 'binary-tree' && [...structures, ...Array.from(result.values(), item => item.structure)].some(item => item.type === 'binary-tree' && item.nodes.some(node => structure.nodes.some(other => other.id === node.id)))) continue
        seen.add(structure.id)
        result.set(structure.id, { structure, step })
      }
    }
    return result
  }, [structures, timelineFrame, trace])

  if (!event) {
    return (
      <div className="visualization-panel">
        <div className="visualization-empty">
          {isExecuting ? (
            <>
              <LoaderCircle
                size={42}
                className="loading-spinner"
              />

              <h3>Executing your code</h3>

              <p>Generating execution trace...</p>
            </>
          ) : (
            <>
              <Workflow size={48} />

              <h3>Ready to visualize</h3>

              <p>
                Write Python code and click Visualize.
              </p>

              {error && <p>{error}</p>}

              {trace?.error && (
                <p>
                  {trace.error.type}:
                  {' '}
                  {trace.error.message}
                </p>
              )}
            </>
          )}
        </div>
      </div>
    )
  }

  const variables = {
    ...(event.arguments ?? {}),
    ...(event.variables ?? {}),
  }

  const changes = event.changes ?? []

  const items: VisualizationItem[] = []

  for (const structure of [...structures, ...Array.from(retained.values(), item => ({ ...item.structure, activity: 'unchanged' as const }))]) {
    const retainedStep = retained.get(structure.id)?.step
    const label = retainedStep === undefined ? '' : ` (last observed at step ${retainedStep + 1})`
    const structureChanges = retainedStep === undefined ? changes : []
    switch (structure.type) {
      case 'array':
        items.push({
          id: structure.id,
          title: `Array — ${structure.name}${label}`,
          activity: structure.activity,
          content: (
            <ArrayVisualizer
              name={structure.name}
              values={structure.values}
              changes={structureChanges}
            />
          ),
        })
        break

      case 'matrix':
        items.push({
          id: structure.id,
          title: `Matrix — ${structure.name}${label}`,
          activity: structure.activity,
          content: (
            <MatrixVisualizer
              name={structure.name}
              values={structure.values}
              changes={structureChanges}
            />
          ),
        })
        break

      case 'linked-list':
        items.push({
          id: structure.id,
          title: `Linked List — ${structure.name}${label}`,
          activity: structure.activity,
          content: (
            <LinkedListVisualizer
              data={retainedStep === undefined ? structure : { ...structure, nodes: structure.nodes.map(node => ({ ...node, changed: false })) }}
            />
          ),
        })
        break

      case 'binary-tree':
        items.push({
          id: structure.id,
          title: `Binary Tree — ${structure.name}${label}`,
          activity: structure.activity,
          content: <BinaryTreeVisualizer data={retainedStep === undefined ? structure : { ...structure, nodes: structure.nodes.map(node => ({ ...node, changed: false })) }} />,
        })
        break

      case 'object':
        items.push({
          id: structure.id,
          title: 'Object References',
          activity: structure.activity,
          content: (
            <ObjectVisualizer
              objects={structure.objects}
              references={structure.references}
              previousObjects={
                previousEvent?.objects ?? {}
              }
              previousReferences={
                previousEvent?.references ?? {}
              }
              truncated={structure.truncated}
            />
          ),
        })
        break
    }
  }

  const recursiveActivity =
    recursiveTree.nodes.some(
      node =>
        node.callStep === currentStep ||
        node.returnStep === currentStep,
    )

  if (recursiveTree.nodes.length > 0) {
    items.push({
      id: 'recursion',
      title: 'Recursion Tree',
      activity: recursiveActivity
        ? 'active'
        : 'unchanged',

      content: (
        <RecursionTree
          tree={recursiveTree}
          onSelectStep={seekFromVisualization}
          topology={fullRecursiveTree}
        />
      ),
    })
  }

  items.sort(
    (a, b) =>
      (timeline.order.get(a.id) ??
        Number.MAX_SAFE_INTEGER) -
      (timeline.order.get(b.id) ??
        Number.MAX_SAFE_INTEGER),
  )

  const activeId =
      recursiveActivity
        ? 'recursion'
        : items.find(
            item =>
              item.id === timelineFrame?.activeId,
          )?.id ??
          items[0]?.id ??
          null

  return (
    <div className="visualization-layout adaptive-root">
      {/* Compact fixed execution information */}

      <div className="adaptive-execution-header">
        <div>
          <span>Step</span>
          <strong>
            {currentStep + 1}/{trace?.events.length}
          </strong>
        </div>

        <div>
          <span>Line</span>
          <strong>{event.line}</strong>
        </div>

        <div>
          <span>Depth</span>
          <strong>{event.depth}</strong>
        </div>

        <div>
          <span>Event</span>
          <strong>
            {event.event.replaceAll('_', ' ')}
          </strong>
        </div>

        <div>
          <span>Status</span>
          <strong>{status}</strong>
        </div>
      </div>

      {(error || trace?.error || event.exception) && (
        <div className="adaptive-error">
          {event.exception
            ? `${event.exception.type}: ${event.exception.message}`
            : trace?.error
              ? `${trace.error.type}: ${trace.error.message}`
              : error}
        </div>
      )}

      {trace?.limit && (
        <div className="adaptive-error">
          Trace limit: {trace.limit}
        </div>
      )}

      {/* Main multi-structure canvas */}      
      <AdaptiveLayout
        key={`layout-${jobId}`}
        items={items}
        activeId={activeId}
      />

      {/* Collapsible execution inspector */}

      <details className="adaptive-inspector">
        <summary>
          Variables &amp; Call Stack
          <span>
            {Object.keys(variables).length} variables
          </span>
        </summary>

        <div className="adaptive-inspector-content">
          <div className="variable-grid">
            {Object.entries(variables).map(
              ([name, value]) => (
                <div
                  key={name}
                  className={
                    changes.some(
                      change => change.name === name,
                    )
                      ? 'variable-tile variable-changed'
                      : 'variable-tile'
                  }
                >
                  <strong>{name}</strong>

                  <code>
                    {JSON.stringify(value)}
                  </code>
                </div>
              ),
            )}
          </div>

          {callTree.nodes.length > 0 && (
            <CallStackVisualizer tree={callTree} />
          )}

          {event.return_value !== undefined &&
            event.function !== '<module>' && (
              <div className="adaptive-return">
                Return value:
                {' '}
                <code>
                  {JSON.stringify(event.return_value)}
                </code>
              </div>
            )}

          {changes.length > 0 && (
            <div className="adaptive-changes">
              <strong>Variable changes</strong>

              {changes.map((change, index) => (
                <div key={`${change.name}-${index}`}>
                  <code>{change.name}</code>
                  {' — '}
                  {change.type.replaceAll('_', ' ')}
                </div>
              ))}
            </div>
          )}
        </div>
      </details>

      {/* Always accessible playback controls */}

      <PlaybackControls key={`playback-${jobId}`} />
    </div>
  )
}
