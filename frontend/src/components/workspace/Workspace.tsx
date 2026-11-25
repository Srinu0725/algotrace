
import { useRef, useState } from 'react'
import type { PointerEvent } from 'react'

import {
  Code2,
  Play,
  Maximize2,
  Minimize2,
  RotateCcw,
  Terminal,
  Workflow,
  GripVertical,
  LoaderCircle,
} from 'lucide-react'

import { useExecutionStore } from '../../store/executionStore'

import CodeEditor from '../editor/CodeEditor'
import VisualizationPanel from '../visualization/VisualizationPanel'

const DEFAULT_CODE = `# Bubble Sort

arr = [5, 3, 8, 1, 2]

n = len(arr)

for i in range(n):
    for j in range(n - i - 1):
        if arr[j] > arr[j + 1]:
            arr[j], arr[j + 1] = arr[j + 1], arr[j]

print(arr)
`

type PanelMode =
  | 'split'
  | 'editor'
  | 'visualization'

export default function Workspace() {
  // Workspace state

  const [code, setCode] = useState(DEFAULT_CODE)

  const [split, setSplit] = useState(50)

  const [panelMode, setPanelMode] =
    useState<PanelMode>('split')

  const [showConsole, setShowConsole] =
    useState(true)

  const workspaceRef =
    useRef<HTMLDivElement>(null)

  // Execution state

  const run = useExecutionStore(
    (state) => state.run,
  )

  const reset = useExecutionStore(
    (state) => state.reset,
  )

  const status = useExecutionStore(
    (state) => state.status,
  )

  const job = useExecutionStore(
    (state) => state.job,
  )

  const trace = useExecutionStore(
    (state) => state.trace,
  )

  const error = useExecutionStore(
    (state) => state.error,
  )

  const isExecuting = useExecutionStore(
    (state) => state.isExecuting,
  )

  // Update editor content

  function updateCode(value: string) {
    if (isExecuting || value === code) {
      return
    }

    reset()
    setCode(value)
  }

  // Execute Python code

  function handleRun() {
    if (isExecuting || !code.trim()) {
      return
    }

    setShowConsole(true)

    // Ensure visualization is visible
    if (panelMode === 'editor') {
      setPanelMode('split')
    }

    void run(code)
  }

  // Reset code and execution state

  function handleReset() {
    if (isExecuting) return

    reset()
    setCode(DEFAULT_CODE)
  }

  // Resize workspace panels

  function resize(
    event: PointerEvent<HTMLDivElement>,
  ) {
    if (
      !event.currentTarget.hasPointerCapture(
        event.pointerId,
      )
    ) {
      return
    }

    const workspace = workspaceRef.current

    if (!workspace) return

    const rect =
      workspace.getBoundingClientRect()

    if (rect.width === 0) return

    const percentage =
      ((event.clientX - rect.left) /
        rect.width) *
      100

    setSplit(
      Math.min(
        75,
        Math.max(25, percentage),
      ),
    )
  }

  // Maximize or restore panels

  function togglePanel(
    panel: 'editor' | 'visualization',
  ) {
    setPanelMode((current) =>
      current === panel ? 'split' : panel,
    )
  }

  return (
    <div className="app-shell">
      {/* Main toolbar */}

      <header className="toolbar">
        <div className="brand">
          <div className="brand-icon">
            <Code2 size={24} />
          </div>

          <span>
            Algo
            <span className="brand-accent">
              Trace
            </span>
          </span>

          <span className="version">
            v0.1
          </span>
        </div>

        <div className="toolbar-actions">
          <span className="language-label">
            <span className="language-dot" />
            Python 3.12
          </span>

          {/* Reset button */}

          <button
            className="icon-button"
            title="Reset code"
            aria-label="Reset code"
            disabled={isExecuting}
            onClick={handleReset}
          >
            <RotateCcw size={17} />
          </button>

          {/* Visualize button */}

          <button
            className="run-button"
            disabled={
              isExecuting || !code.trim()
            }
            onClick={handleRun}
            title="Execute and visualize Python code"
          >
            {isExecuting ? (
              <LoaderCircle
                size={16}
                className="loading-spinner"
              />
            ) : (
              <Play
                size={16}
                fill="currentColor"
              />
            )}

            {isExecuting
              ? 'Executing...'
              : 'Visualize'}
          </button>
        </div>
      </header>

      {/* Main workspace */}

      <main
        ref={workspaceRef}
        className="workspace"
      >
        {/* Left panel: Code editor */}

        {panelMode !== 'visualization' && (
          <section
            className="workspace-panel"
            style={{
              flexBasis:
                panelMode === 'editor'
                  ? '100%'
                  : `${split}%`,
            }}
          >
            {/* Editor header */}

            <div className="panel-header">
              <div className="panel-title">
                <Code2 size={16} />

                <span>Code Editor</span>
              </div>

              <button
                className="icon-button"
                title={
                  panelMode === 'editor'
                    ? 'Restore split workspace'
                    : 'Maximize editor'
                }
                aria-label="Toggle editor maximization"
                onClick={() =>
                  togglePanel('editor')
                }
              >
                {panelMode === 'editor' ? (
                  <Minimize2 size={16} />
                ) : (
                  <Maximize2 size={16} />
                )}
              </button>
            </div>

            {/* Editor tabs */}

            <div className="editor-tabs">
              <div className="editor-tab">
                <span className="python-icon">
                  Py
                </span>

                main.py
              </div>
            </div>

            {/* Monaco Editor */}

            <CodeEditor
              code={code}
              onChange={updateCode}
              readOnly={isExecuting}
            />

            {/* Output console */}

            <div className="console">
              <button
                className="console-header"
                aria-expanded={showConsole}
                onClick={() =>
                  setShowConsole(
                    (current) => !current,
                  )
                }
              >
                <Terminal size={15} />

                Output Console

                <span className="console-toggle">
                  {showConsole ? '−' : '+'}
                </span>
              </button>

              {showConsole && (
                <div
                  className="console-content"
                  aria-live="polite"
                >
                  {/* Execution loading state */}

                  {isExecuting && (
                    <div className="console-loading">
                      <LoaderCircle
                        size={14}
                        className="loading-spinner"
                      />

                      <span className="muted">
                        {status === 'loading_trace'
                          ? 'Loading execution trace...'
                          : `Execution: ${status.replaceAll('_', ' ')}...`}
                      </span>
                    </div>
                  )}

                  {/* Standard output */}

                  {job?.stdout && (
                    <pre className="console-stdout">
                      {job.stdout}
                    </pre>
                  )}

                  {/* Standard error */}

                  {job?.stderr && (
                    <pre className="console-stderr">
                      {job.stderr}
                    </pre>
                  )}

                  {/* API or execution errors */}

                  {error && (
                    <pre className="console-stderr">
                      {error}
                    </pre>
                  )}

                  {/* Structured Python error */}

                  {trace?.error && (
                    <pre className="console-stderr">
                      {trace.error.type}:{' '}
                      {trace.error.message}
                    </pre>
                  )}

                  {/* Output truncation warnings */}

                  {job?.stdout_truncated && (
                    <div className="muted">
                      Standard output was truncated.
                    </div>
                  )}

                  {job?.stderr_truncated && (
                    <div className="muted">
                      Standard error was truncated.
                    </div>
                  )}

                  {/* Execution metadata */}

                  {job && !isExecuting && (
                    <div className="console-meta">
                      Exit code: {job.exit_code}

                      {' · '}

                      Duration: {job.duration_ms} ms

                      {' · '}

                      Steps: {trace?.events.length ?? 0}
                    </div>
                  )}

                  {/* Successful execution without output */}

                  {job &&
                    !isExecuting &&
                    job.status === 'completed' &&
                    !job.stdout &&
                    !job.stderr && (
                      <span className="muted">
                        Execution completed without output.
                      </span>
                    )}

                  {/* Initial empty state */}

                  {!job &&
                    !error &&
                    !isExecuting && (
                      <span className="muted">
                        Execution output will appear here.
                      </span>
                    )}
                </div>
              )}
            </div>
          </section>
        )}

        {/* Draggable divider */}

        {panelMode === 'split' && (
          <div
            className="resize-handle"
            role="separator"
            aria-label="Resize workspace panels"
            aria-orientation="vertical"
            aria-valuemin={25}
            aria-valuemax={75}
            aria-valuenow={Math.round(split)}
            tabIndex={0}
            onKeyDown={(event) => {
              if (event.key === 'ArrowLeft') {
                event.preventDefault()

                setSplit((value) =>
                  Math.max(25, value - 5),
                )
              }

              if (event.key === 'ArrowRight') {
                event.preventDefault()

                setSplit((value) =>
                  Math.min(75, value + 5),
                )
              }
            }}
            onPointerDown={(event) => {
              event.preventDefault()

              event.currentTarget.setPointerCapture(
                event.pointerId,
              )
            }}
            onPointerMove={resize}
            onPointerUp={(event) => {
              if (
                event.currentTarget.hasPointerCapture(
                  event.pointerId,
                )
              ) {
                event.currentTarget.releasePointerCapture(
                  event.pointerId,
                )
              }
            }}
            onPointerCancel={(event) => {
              if (
                event.currentTarget.hasPointerCapture(
                  event.pointerId,
                )
              ) {
                event.currentTarget.releasePointerCapture(
                  event.pointerId,
                )
              }
            }}
          >
            <GripVertical size={14} />
          </div>
        )}

        {/* Right panel: Visualization */}

        {panelMode !== 'editor' && (
          <section className="workspace-panel visualization-section">
            <div className="panel-header">
              <div className="panel-title">
                <Workflow size={16} />

                <span>Visualization</span>
              </div>

              <button
                className="icon-button"
                title={
                  panelMode === 'visualization'
                    ? 'Restore split workspace'
                    : 'Maximize visualization'
                }
                aria-label="Toggle visualization maximization"
                onClick={() =>
                  togglePanel('visualization')
                }
              >
                {panelMode === 'visualization' ? (
                  <Minimize2 size={16} />
                ) : (
                  <Maximize2 size={16} />
                )}
              </button>
            </div>

            <VisualizationPanel />
          </section>
        )}
      </main>

      {/* Bottom status bar */}

      <footer className="status-bar">
        <div
          className="status-item"
          role="status"
        >
          <span
            className={`status-dot status-dot-${status}`}
          />

          <span>
            {status === 'idle'
              ? 'Workspace ready'
              : status.replaceAll('_', ' ')}
          </span>

          {job && !isExecuting && (
            <span>
              {job.duration_ms} ms
            </span>
          )}
        </div>

        <div className="status-right">
          <span>Python</span>

          <span>UTF-8</span>

          <span>Spaces: 4</span>

          <span>
            {code.split('\n').length} lines
          </span>
        </div>
      </footer>
    </div>
  )
}
