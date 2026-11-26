import { useMemo } from 'react'

import {
  Workflow,
  LoaderCircle,
} from 'lucide-react'

import { useExecutionStore } from '../../store/executionStore'

import ArrayVisualizer from './ArrayVisualizer'
import RecursionTree from './RecursionTree'
import CallStackVisualizer from './CallStackVisualizer'

import PlaybackControls from '../playback/PlaybackControls'

import { buildCallTree } from '../../utils/traceTree'
import MatrixVisualizer from './MatrixVisualizer'

import { isMatrix } from '../../utils/dataStructure'

export default function VisualizationPanel() {
  const {
    trace,
    status,
    error,
    currentStep,
    isExecuting,
    jobId,
    setStep,
  } = useExecutionStore()

  const events = trace?.events ?? []

  const event = events[currentStep]

  // Reconstruct function calls up to the current step.
  const callTree = useMemo(
    () =>
      buildCallTree(
        trace?.events ?? [],
        currentStep,
      ),
    [trace, currentStep],
  )

  // Empty or loading state.
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

              <p>
                Generating execution trace...
              </p>
            </>
          ) : (
            <>
              <Workflow size={48} />

              <h3>
                {trace
                  ? 'No execution steps available'
                  : 'Ready to visualize'}
              </h3>

              <p>
                Write your Python code and click
                Visualize to explore its execution.
              </p>

              {error && (
                <p className="console-stderr">
                  {error}
                </p>
              )}
            </>
          )}
        </div>
      </div>
    )
  }

  // Merge arguments and local variables.
  const variables = {
    ...event.arguments,
    ...event.variables,
  }

  const changes = event.changes ?? []

  const changedVariables = new Set(
    changes.map(change => change.name),
  )

  const arrayVariables = Object.entries(
    variables,
  ).filter(([, value]) => Array.isArray(value))

  const scalarVariables = Object.entries(
    variables,
  ).filter(([, value]) => !Array.isArray(value))

  return (
    <div className="visualization-layout">
      <div className="trace-workspace">

        {/* Execution summary */}

        <div className="trace-summary">
          <div>
            <h3>Execution Trace</h3>

            <p>
              {events.length} execution steps
            </p>
          </div>

          <span className="trace-status">
            {status.replaceAll('_', ' ')}
          </span>
        </div>

        {/* Execution errors */}

        {error && (
          <section className="trace-card">
            <h3>Execution Error</h3>

            <pre className="console-stderr">
              {error}
            </pre>
          </section>
        )}

        {trace?.error && (
          <section className="trace-card">
            <h3>{trace.error.type}</h3>

            <pre className="console-stderr">
              {trace.error.message}
            </pre>

            {trace.error.line != null && (
              <p>
                Source line: {trace.error.line}
              </p>
            )}
          </section>
        )}

        {trace?.limit && (
          <section className="trace-card">
            <h3>Execution Limit Reached</h3>

            <p>{trace.limit}</p>
          </section>
        )}

        {/* Current execution state */}

        <section className="trace-card">
          <h3>Execution State</h3>

          <div className="execution-state-grid">
            <div>
              <span>Step</span>

              <strong>
                {currentStep + 1}/{events.length}
              </strong>
            </div>

            <div>
              <span>Source Line</span>

              <strong>{event.line}</strong>
            </div>

            <div>
              <span>Event</span>

              <strong>
                {event.event.replaceAll('_', ' ')}
              </strong>
            </div>

            <div>
              <span>Depth</span>

              <strong>{event.depth}</strong>
            </div>
          </div>

          {event.event === 'line' && (
            <p className="trace-event-note">
              Next line to execute: {event.line}.
              Showing state before execution.
            </p>
          )}
        </section>

        {/* NEW: Function call tree */}

        {callTree.nodes.length > 0 && (
          <RecursionTree
            key={jobId}
            tree={callTree}
            onSelectStep={setStep}
          />
        )}

        {/* NEW: Dynamic call stack */}

        {callTree.nodes.length > 0 && (
          <CallStackVisualizer
            tree={callTree}
          />
        )}

        {/* Automatic array and matrix visualization */}

{arrayVariables.map(([name, value]) => {
  if (!Array.isArray(value)) {
    return null
  }

  return (
    <section
      className="trace-card"
      key={name}
    >
      {isMatrix(value) ? (
        <MatrixVisualizer
          name={name}
          values={value}
          changes={changes}
        />
      ) : (
        <ArrayVisualizer
          name={name}
          values={value}
          changes={changes}
        />
      )}
    </section>
  )
})}

        {arrayVariables.map(([name, value]) => {
          if (!Array.isArray(value)) {
            return null
          }

          return (
            <section
              className="trace-card"
              key={name}
            >
              <ArrayVisualizer
                name={name}
                values={value}
                changes={changes}
              />
            </section>
          )
        })}

        {/* Scalar variables */}

        {scalarVariables.length > 0 && (
          <section className="trace-card">
            <h3>Variables</h3>

            <div className="variable-grid">
              {scalarVariables.map(
                ([name, value]) => (
                  <div
                    key={name}
                    className={`variable-tile ${
                      changedVariables.has(name)
                        ? 'variable-changed'
                        : ''
                    }`}
                  >
                    <span>{name}</span>

                    <code>
                      {JSON.stringify(value)}
                    </code>
                  </div>
                ),
              )}
            </div>
          </section>
        )}

        {/* Basic stack fallback */}

        {callTree.nodes.length === 0 && (
          <section className="trace-card">
            <h3>Call Stack</h3>

            <div className="trace-details">
              <span>Current Function</span>

              <strong>{event.function}</strong>
            </div>

            {(event.call_stack ?? []).map(
              (frame, index) => (
                <div
                  className="stack-frame"
                  key={`${frame}-${index}`}
                >
                  {frame}
                </div>
              ),
            )}
          </section>
        )}

        {/* Variable changes */}

        {changes.length > 0 && (
          <section className="trace-card">
            <h3>Variable Changes</h3>

            {changes.map((change, index) => (
              <div
                className="variable-change-row"
                key={`${change.name}-${index}`}
              >
                <div>
                  <strong>{change.name}</strong>

                  <span>
                    {change.type.replaceAll(
                      '_',
                      ' ',
                    )}
                  </span>
                </div>

                {change.old_value !== undefined && (
                  <div>
                    <span>Previous</span>

                    <code>
                      {JSON.stringify(
                        change.old_value,
                      )}
                    </code>
                  </div>
                )}

                {change.new_value !== undefined && (
                  <div>
                    <span>Current</span>

                    <code>
                      {JSON.stringify(
                        change.new_value,
                      )}
                    </code>
                  </div>
                )}

                {change.value !== undefined && (
                  <div>
                    <span>Value</span>

                    <code>
                      {JSON.stringify(
                        change.value,
                      )}
                    </code>
                  </div>
                )}
              </div>
            ))}
          </section>
        )}

        {/* Function return value */}

        {event.return_value !== undefined && (
          <section className="trace-card">
            <h3>Return Value</h3>

            <pre className="trace-json">
              {JSON.stringify(
                event.return_value,
                null,
                2,
              )}
            </pre>
          </section>
        )}

        {/* Exception information */}

        {event.exception && (
          <section className="trace-card">
            <h3>Exception</h3>

            <div className="trace-details">
              <span>Type</span>

              <strong>
                {event.exception.type}
              </strong>

              <span>Message</span>

              <strong className="console-stderr">
                {event.exception.message}
              </strong>
            </div>
          </section>
        )}
      </div>

      {/* Fixed playback controls */}

      <PlaybackControls key={jobId} />
    </div>
  )
}