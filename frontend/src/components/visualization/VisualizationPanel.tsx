
import {
  Workflow,
  ChevronLeft,
  ChevronRight,
  LoaderCircle,
} from 'lucide-react'

import { useExecutionStore } from '../../store/executionStore'

export default function VisualizationPanel() {
  const {
    trace,
    status,
    error,
    currentStep,
    setStep,
    nextStep,
    previousStep,
    isExecuting,
  } = useExecutionStore()

  const events = trace?.events ?? []
  const event = events[currentStep]

  // Empty, loading or error state
  if (!event) {
    return (
      <div className="visualization-panel">
        <div className="visualization-empty">
          <div className="visualization-icon">
            {isExecuting ? (
              <LoaderCircle
                size={36}
                strokeWidth={1.5}
                className="loading-spinner"
              />
            ) : (
              <Workflow
                size={36}
                strokeWidth={1.5}
              />
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
                ? 'Waiting for execution results...'
                : status === 'idle'
                  ? 'Click Visualize to execute Python code.'
                  : `Execution status: ${status.replaceAll('_', ' ')}`)}
          </p>
        </div>
      </div>
    )
  }

  // Function arguments and local variables
  const variables = {
    ...(event.arguments ?? {}),
    ...(event.variables ?? {}),
  }

  const changedVariables = new Set(
    (event.changes ?? []).map((change) => change.name),
  )

  return (
    <div className="trace-workspace">
      {/* Execution summary */}

      <div className="trace-summary">
        <span className={`trace-status status-${status}`}>
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
          <strong>{trace.error.type}</strong>

          <p>{trace.error.message}</p>

          {trace.error.line !== null && (
            <p>Source line: {trace.error.line}</p>
          )}
        </div>
      )}

      {/* Trace limit information */}

      {trace?.limit && (
        <div className="trace-card">
          <h3>Trace Limit Reached</h3>

          <p className="muted">
            {trace.limit}
          </p>

          <p className="muted">
            Available execution steps can still be explored.
          </p>
        </div>
      )}

      {/* Execution state */}

      <div className="trace-card">
        <h3>Execution State</h3>

        <div className="trace-details">
          <span>Step</span>
          <strong>{event.step}</strong>

          <span>Event</span>
          <strong>
            {event.event.replaceAll('_', ' ')}
          </strong>

          <span>Source line</span>
          <strong>{event.line}</strong>

          <span>Function</span>
          <strong>{event.function}</strong>

          <span>Call depth</span>
          <strong>{event.depth}</strong>
        </div>

        {event.event === 'line' && (
          <p className="muted">
            State before executing this line.
          </p>
        )}
      </div>

      {/* Variables */}

      <div className="trace-card">
        <h3>
          Variables ({Object.keys(variables).length})
        </h3>

        {Object.keys(variables).length === 0 && (
          <p className="muted">
            No variables in this snapshot.
          </p>
        )}

        {Object.entries(variables).map(([name, value]) => (
          <div
            className={`variable-row ${
              changedVariables.has(name)
                ? 'variable-changed'
                : ''
            }`}
            key={name}
          >
            <span>{name}</span>

            <code>
              {JSON.stringify(value)}
            </code>
          </div>
        ))}
      </div>

      {/* Call stack */}

      <div className="trace-card">
        <h3>Call Stack</h3>

        {!event.call_stack?.length && (
          <p className="muted">
            No active stack frames in this snapshot.
          </p>
        )}

        {event.call_stack?.map((frame, index) => (
          <div
            className="stack-frame"
            key={`${frame}-${index}`}
          >
            {index + 1}. {frame}
          </div>
        ))}
      </div>

      {/* Function return value */}

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

      {/* Exception information */}

      {event.exception && (
        <div className="trace-card console-stderr">
          <h3>Exception</h3>

          <strong>{event.exception.type}</strong>

          <p>{event.exception.message}</p>
        </div>
      )}

      {/* Variable changes */}

      <div className="trace-card">
        <h3>Variable Changes</h3>

        {!event.changes?.length ? (
          <p className="muted">
            No variable changes in this step.
          </p>
        ) : (
          <pre className="trace-json">
            {JSON.stringify(
              event.changes,
              null,
              2,
            )}
          </pre>
        )}
      </div>

      {/* Step navigation */}

      <div className="trace-controls">
        <button
          className="icon-button"
          title="Previous step"
          aria-label="Previous step"
          onClick={previousStep}
          disabled={currentStep === 0}
        >
          <ChevronLeft size={20} />
        </button>

        <input
          aria-label="Execution step"
          type="range"
          min={0}
          max={events.length - 1}
          value={currentStep}
          onChange={(event) =>
            setStep(Number(event.target.value))
          }
        />

        <button
          className="icon-button"
          title="Next step"
          aria-label="Next step"
          onClick={nextStep}
          disabled={
            currentStep >= events.length - 1
          }
        >
          <ChevronRight size={20} />
        </button>

        <span>
          {currentStep + 1} / {events.length}
        </span>
      </div>
    </div>
  )
}
