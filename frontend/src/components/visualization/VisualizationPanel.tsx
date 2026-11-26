
import {
  Workflow,
  LoaderCircle,
} from 'lucide-react'

import {
  useExecutionStore,
} from '../../store/executionStore'

import ArrayVisualizer from './ArrayVisualizer'

import PlaybackControls from '../playback/PlaybackControls'

export default function VisualizationPanel() {
  const {
    trace,
    status,
    error,
    currentStep,
    isExecuting,
    jobId,
  } = useExecutionStore()

  const events = trace?.events ?? []

  const event = events[currentStep]

  if (!event) {
    return (
      <div className="visualization-panel">
        <div className="visualization-empty">
          <div className="visualization-icon">
            {isExecuting ? (
              <LoaderCircle
                size={36}
                className="loading-spinner"
              />
            ) : (
              <Workflow size={36} />
            )}
          </div>

          <h2>
            {isExecuting
              ? 'Executing your code'
              : status === 'idle'
                ? 'See your code come to life'
                : 'No execution steps available'}
          </h2>

          <p>
            {error ??
              trace?.error?.message ??
              (isExecuting
                ? 'Generating execution trace...'
                : status === 'idle'
                  ? 'Click Visualize to begin.'
                  : `Execution status: ${status}`)}
          </p>
        </div>
      </div>
    )
  }

  const variables = {
    ...(event.arguments ?? {}),
    ...(event.variables ?? {}),
  }

  const changes = event.changes ?? []

  const changedVariables = new Set(
    changes.map((change) => change.name),
  )

  const arrays = Object.entries(
    variables,
  ).filter(
    ([, value]) => Array.isArray(value),
  )

  const scalarVariables = Object.entries(
    variables,
  ).filter(
    ([, value]) => !Array.isArray(value),
  )

  return (
    <div className="visualization-layout">
      {/* Scrollable visualization area */}

      <div className="trace-workspace">
        <div className="trace-summary">
          <span
            className={`trace-status status-${status}`}
          >
            {status.replaceAll('_', ' ')}
          </span>

          <span>
            {events.length} execution steps
          </span>
        </div>

        {/* Execution errors */}

        {error && (
          <div className="trace-card console-stderr">
            {error}
          </div>
        )}

        {trace?.error && (
          <div className="trace-card console-stderr">
            <strong>
              {trace.error.type}
            </strong>

            <p>
              {trace.error.message}
            </p>
          </div>
        )}

        {trace?.limit && (
          <div className="trace-card">
            <h3>Trace limit reached</h3>

            <p className="muted">
              {trace.limit}
            </p>
          </div>
        )}

        {/* Compact execution state */}

        <div className="execution-state-compact">
          <div>
            <span>Step</span>
            <strong>{event.step}</strong>
          </div>

          <div>
            <span>Source line</span>
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
          <div className="execution-line-info">
            Next line to execute: {event.line}.
            Showing state before execution.
          </div>
        )}

        {/* Graphical arrays */}

        {arrays.map(([name, value]) => {
          if (!Array.isArray(value)) {
            return null
          }

          return (
            <div
              className="trace-card"
              key={name}
            >
              <ArrayVisualizer
                name={name}
                values={value}
                changes={changes}
              />
            </div>
          )
        })}

        {/* Other variables */}

        <div className="trace-card">
          <h3>Variables</h3>

          {scalarVariables.length === 0 ? (
            <p className="muted">
              No other variables in this snapshot.
            </p>
          ) : (
            <div className="variable-grid">
              {scalarVariables.map(
                ([name, value]) => (
                  <div
                    key={name}
                    className={`variable-tile ${
                      changedVariables.has(name)
                        ? 'variable-tile-changed'
                        : ''
                    }`}
                  >
                    <span>{name}</span>

                    <strong>
                      {JSON.stringify(value)}
                    </strong>
                  </div>
                ),
              )}
            </div>
          )}
        </div>

        {/* Call stack */}

        <div className="trace-card">
          <h3>Call Stack</h3>

          <p className="muted">
            Current function: {event.function}
          </p>

          {event.call_stack?.map(
            (frame, index) => (
              <div
                className="stack-frame"
                key={`${frame}-${index}`}
              >
                {index + 1}. {frame}
              </div>
            ),
          )}
        </div>

        {/* Variable changes */}

        <div className="trace-card">
          <h3>Variable Changes</h3>

          {changes.length === 0 ? (
            <p className="muted">
              No variable changes in this step.
            </p>
          ) : (
            changes.map((change, index) => (
              <div
                className="change-row"
                key={`${change.name}-${index}`}
              >
                <div className="change-header">
                  <strong>
                    {change.name}
                  </strong>

                  <span>
                    {change.type.replaceAll(
                      '_',
                      ' ',
                    )}
                  </span>
                </div>

                {change.old_value !== undefined && (
                  <div className="change-old">
                    {JSON.stringify(
                      change.old_value,
                    )}
                  </div>
                )}

                {change.new_value !== undefined && (
                  <div className="change-new">
                    {JSON.stringify(
                      change.new_value,
                    )}
                  </div>
                )}

                {change.value !== undefined && (
                  <div className="change-new">
                    {JSON.stringify(
                      change.value,
                    )}
                  </div>
                )}
              </div>
            ))
          )}
        </div>

        {/* Return value */}

        {event.return_value !== undefined && (
          <div className="trace-card">
            <h3>Return Value</h3>

            <pre className="trace-json">
              {JSON.stringify(
                event.return_value,
                null,
                2,
              )}
            </pre>
          </div>
        )}

        {/* Exception */}

        {event.exception && (
          <div className="trace-card console-stderr">
            <h3>
              {event.exception.type}
            </h3>

            <p>
              {event.exception.message}
            </p>
          </div>
        )}
      </div>

      {/* Fixed playback controls */}

      <PlaybackControls key={jobId} />
    </div>
  )
}
