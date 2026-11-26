
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

  const setStep = useExecutionStore(
    s => s.setStep,
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

  const recursiveTree = useMemo(
    () => extractRecursiveTree(callTree),
    [callTree],
  )

  // Calculate stable visualization order and
  // the last meaningful activity at this step.
  // This also works when seeking backwards.

  const activityHistory = useMemo(() => {
    const order = new Map<string, number>()

    let activeId: string | null = null

    if (!trace) {
      return { order, activeId }
    }

    for (
      let index = 0;
      index <= currentStep;
      index++
    ) {
      const current = trace.events[index]

      if (!current) continue

      const detected = detectStructures(
        current,
        trace.events[index - 1],
      )

      for (const structure of detected) {
        if (!order.has(structure.id)) {
          order.set(structure.id, order.size)
        }
      }

      const changed = detected.filter(
        structure =>
          structure.activity !== 'unchanged',
      )

      const specialized = changed.filter(
        structure => structure.type !== 'object',
      )

      const candidates =
        specialized.length > 0
          ? specialized
          : changed

      if (candidates.length > 0) {
        activeId =
          candidates.find(
            item => item.id === activeId,
          )?.id ??
          candidates[0].id
      }
    }

    return { order, activeId }
  }, [trace, currentStep])

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

  for (const structure of structures) {
    switch (structure.type) {
      case 'array':
        items.push({
          id: structure.id,
          title: `Array — ${structure.name}`,
          activity: structure.activity,
          content: (
            <ArrayVisualizer
              name={structure.name}
              values={structure.values}
              changes={changes}
            />
          ),
        })
        break

      case 'matrix':
        items.push({
          id: structure.id,
          title: `Matrix — ${structure.name}`,
          activity: structure.activity,
          content: (
            <MatrixVisualizer
              name={structure.name}
              values={structure.values}
              changes={changes}
            />
          ),
        })
        break

      case 'linked-list':
        items.push({
          id: structure.id,
          title: `Linked List — ${structure.name}`,
          activity: structure.activity,
          content: (
            <LinkedListVisualizer
              data={structure}
            />
          ),
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
          onSelectStep={setStep}
        />
      ),
    })
  }

  items.sort(
    (a, b) =>
      (activityHistory.order.get(a.id) ??
        Number.MAX_SAFE_INTEGER) -
      (activityHistory.order.get(b.id) ??
        Number.MAX_SAFE_INTEGER),
  )

  const activeId =
    recursiveActivity
      ? 'recursion'
      : items.find(
          item =>
            item.id === activityHistory.activeId,
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
