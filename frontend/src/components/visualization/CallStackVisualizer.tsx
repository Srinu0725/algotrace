import { Layers } from 'lucide-react'

import type {
  CallTreeState,
} from '../../utils/traceTree'

import {
  formatCall,
  formatValue,
} from '../../utils/traceTree'

interface Props {
  tree: CallTreeState
}

export default function CallStackVisualizer({
  tree,
}: Props) {
  if (tree.nodes.length === 0) {
    return null
  }

  const frames = [...tree.stack].reverse()

  return (
    <section className="trace-card callstack-card">
      <div className="recursion-header">
        <div className="recursion-title">
          <Layers size={18} />

          <h3>Call Stack</h3>
        </div>

        <span className="callstack-depth">
          {frames.length} frames
        </span>
      </div>

      {frames.length === 0 ? (
        <div className="callstack-empty">
          All tracked function calls have exited.
        </div>
      ) : (
        <div className="callstack-container">
          <div className="callstack-direction">
            TOP OF STACK
          </div>

          {frames.map((frame, index) => {
            const values = {
              ...frame.arguments,
              ...frame.variables,
            }

            const entries = Object.entries(values)

            return (
              <div
                key={frame.id}
                className={`callstack-frame ${
                  index === 0 ? 'active' : ''
                }`}
              >
                <div className="callstack-frame-header">
                  <strong>
                    {formatCall(frame)}
                  </strong>

                  <span>
                    {index === 0
                      ? 'Executing'
                      : 'Waiting'}
                  </span>
                </div>

                {entries.length > 0 && (
                  <div className="callstack-variables">
                    {entries.map(([name, value]) => (
                      <div
                        className="callstack-variable"
                        key={name}
                      >
                        <span>{name}</span>

                        <code>
                          {formatValue(value)}
                        </code>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )
          })}

          <div className="callstack-direction">
            BOTTOM OF STACK
          </div>
        </div>
      )}
    </section>
  )
}