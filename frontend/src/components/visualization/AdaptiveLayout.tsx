import {
  useLayoutEffect,
  useRef,
  useState,
} from 'react'

import type { ReactNode } from 'react'
import './adaptive-phase15.css'

import {
  Pin,
  Maximize2,
  Minimize2,
  ChevronDown,
  ChevronUp,
  LocateFixed,
} from 'lucide-react'

import type {
  VisualizationMode,
  ActivityState,
} from '../../types/visualization'

export interface VisualizationItem {
  id: string
  title: string
  activity: ActivityState
  content: ReactNode
}

interface Props {
  items: VisualizationItem[]
  activeId: string | null
}

export default function AdaptiveLayout({
  items,
  activeId,
}: Props) {
  const [mode, setMode] =
    useState<VisualizationMode>('auto')

  const [autoFollow, setAutoFollow] =
    useState(true)

  const [pinned, setPinned] =
    useState<string[]>([])

  const [collapsed, setCollapsed] =
    useState<string[]>([])

  const [focusedId, setFocusedId] =
    useState<string | null>(null)

  const containerRef =
    useRef<HTMLDivElement>(null)

  const focusedItem =
    items.find(item => item.id === focusedId) ??
    items.find(item => item.id === activeId) ??
    items[0]

  const visibleItems =
    mode === 'focus'
      ? focusedItem
        ? [focusedItem, ...items.filter(item =>
            item.id !== focusedItem.id && pinned.includes(item.id),
          )]
        : []
      : items

  function togglePin(id: string) {
    setPinned(current =>
      current.includes(id)
        ? current.filter(value => value !== id)
        : [...current, id],
    )
  }

  function toggleCollapse(id: string) {
    setCollapsed(current =>
      current.includes(id)
        ? current.filter(value => value !== id)
        : [...current, id],
    )
  }

  function toggleFocus(id: string) {
    if (mode === 'focus' && focusedId === id) {
      setMode('auto')
      setFocusedId(null)
    } else {
      setFocusedId(id)
      setMode('focus')
    }
  }

  // Scroll only when the active visualization changes.
  // Never reposition the viewport on every trace step.

  useLayoutEffect(() => {
    if (
      !autoFollow ||
      mode !== 'auto' ||
      !activeId
    ) {
      return
    }

    const container = containerRef.current

    if (!container) return

    const target = Array.from(
      container.querySelectorAll<HTMLElement>(
        '[data-viz-id]',
      ),
    ).find(
      element =>
        element.dataset.vizId === activeId,
    )

    if (!target) return

    const viewport =
      container.getBoundingClientRect()

    const bounds =
      target.getBoundingClientRect()

    const visibleHeight =
      Math.min(bounds.bottom, viewport.bottom) -
      Math.max(bounds.top, viewport.top)

    const requiredHeight = Math.min(
      bounds.height,
      viewport.height * 0.6,
    )

    if (visibleHeight >= requiredHeight) {
      return
    }

    container.scrollTo({
      top: Math.max(
        0,
        container.scrollTop +
          bounds.top -
          viewport.top -
          8,
      ),
      behavior: 'auto',
    })
  }, [activeId, autoFollow, mode])

  return (
    <div className="adaptive-layout">
      <div className="adaptive-toolbar">
        <div className="adaptive-modes">
          {(['auto', 'all', 'focus'] as const)
            .map(value => (
              <button
                key={value}
                className={
                  mode === value
                    ? 'adaptive-mode selected'
                    : 'adaptive-mode'
                }
                onClick={() => setMode(value)}
              >
                {value[0].toUpperCase() +
                  value.slice(1)}
              </button>
            ))}
        </div>

        <button
          className={
            autoFollow
              ? 'auto-follow-button active'
              : 'auto-follow-button'
          }
          aria-pressed={autoFollow}
          onClick={() =>
            setAutoFollow(value => !value)
          }
        >
          <LocateFixed size={14} />
          Follow {autoFollow ? 'ON' : 'OFF'}
        </button>
      </div>

      {mode === 'focus' && (
        <div className="adaptive-selector">
          <select
            value={focusedItem?.id ?? ''}
            onChange={event =>
              setFocusedId(event.target.value)
            }
          >
            {items.map(item => (
              <option
                key={item.id}
                value={item.id}
              >
                {item.title}
              </option>
            ))}
          </select>
        </div>
      )}

      <div
        ref={containerRef}
        className={`adaptive-scroll mode-${mode}`}
      >
        <div className="adaptive-grid">
          {visibleItems.map(item => {
            const isActive =
              item.id === activeId

            const isPinned =
              pinned.includes(item.id)

            const isCollapsed =
              mode !== 'focus' &&
              !isPinned &&
              collapsed.includes(item.id)

            return (
              <article
                key={item.id}
                data-viz-id={item.id}
                className={[
                  'adaptive-card',
                  isActive ? 'is-active' : '',
                  isPinned ? 'is-pinned' : '',
                  mode === 'focus' && item.id === focusedItem?.id ? 'is-focused' : '',
                ].join(' ')}
              >
                <header className="adaptive-card-header">
                  <div className="adaptive-card-title">
                    <strong>{item.title}</strong>

                    {item.activity !== 'unchanged' && (
                      <span className="adaptive-activity">
                        {item.activity}
                      </span>
                    )}
                  </div>

                  <div className="adaptive-card-actions">
                    <button
                      className={
                        isPinned
                          ? 'icon-button pinned-button'
                          : 'icon-button'
                      }
                      title={
                        isPinned ? 'Unpin' : 'Pin'
                      }
                      aria-pressed={isPinned}
                      onClick={() =>
                        togglePin(item.id)
                      }
                    >
                      <Pin size={14} />
                    </button>

                    <button
                      className="icon-button"
                      title="Toggle focus"
                      onClick={() =>
                        toggleFocus(item.id)
                      }
                    >
                      {mode === 'focus' && item.id === focusedItem?.id
                        ? <Minimize2 size={14} />
                        : <Maximize2 size={14} />}
                    </button>

                    {mode !== 'focus' && (
                      <button
                        className="icon-button"
                        title={
                          isCollapsed
                            ? 'Expand'
                            : 'Collapse'
                        }
                        onClick={() =>
                          toggleCollapse(item.id)
                        }
                      >
                        {isCollapsed
                          ? <ChevronDown size={15} />
                          : <ChevronUp size={15} />}
                      </button>
                    )}
                  </div>
                </header>

                {!isCollapsed && (
                  <div className="adaptive-card-body">
                    {item.content}
                  </div>
                )}
              </article>
            )
          })}
        </div>

        {visibleItems.length === 0 && (
          <div className="adaptive-empty">
            No data structures available
            at this execution step.
          </div>
        )}
      </div>
    </div>
  )
}
