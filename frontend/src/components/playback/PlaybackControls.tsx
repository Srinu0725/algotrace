
import {
  Play,
  Pause,
  SkipBack,
  SkipForward,
  ChevronLeft,
  ChevronRight,
} from 'lucide-react'

import {
  usePlayback,
} from '../../hooks/usePlayback'

export default function PlaybackControls() {
  const {
    isPlaying,
    speed,
    setSpeed,

    currentStep,
    totalSteps,

    togglePlayback,
    seek,
    restart,
    next,
    previous,
    finish,
  } = usePlayback()

  const hasSteps = totalSteps > 0

  const isFirst =
    currentStep === 0

  const isLast =
    currentStep >= totalSteps - 1

  return (
    <div className="playback-panel">
      <div className="playback-progress">
        <span>
          Step {hasSteps ? currentStep + 1 : 0}
          {' of '}
          {totalSteps}
        </span>

        <span>
          {hasSteps
            ? Math.round(
                ((currentStep + 1) /
                  totalSteps) *
                  100,
              )
            : 0}
          %
        </span>
      </div>

      <input
        className="playback-slider"
        type="range"
        aria-label="Execution progress"
        min={0}
        max={Math.max(0, totalSteps - 1)}
        value={currentStep}
        disabled={!hasSteps}
        onChange={(event) =>
          seek(Number(event.target.value))
        }
      />

      <div className="playback-actions">
        <div className="playback-buttons">
          <button
            className="icon-button"
            title="Restart"
            aria-label="Restart"
            disabled={!hasSteps}
            onClick={restart}
          >
            <SkipBack size={17} />
          </button>

          <button
            className="icon-button"
            title="Previous step"
            aria-label="Previous step"
            disabled={!hasSteps || isFirst}
            onClick={previous}
          >
            <ChevronLeft size={22} />
          </button>

          <button
            className="playback-play"
            title={isPlaying ? 'Pause' : 'Play'}
            aria-label={
              isPlaying ? 'Pause' : 'Play'
            }
            disabled={totalSteps < 2}
            onClick={togglePlayback}
          >
            {isPlaying ? (
              <Pause
                size={20}
                fill="currentColor"
              />
            ) : (
              <Play
                size={20}
                fill="currentColor"
              />
            )}
          </button>

          <button
            className="icon-button"
            title="Next step"
            aria-label="Next step"
            disabled={!hasSteps || isLast}
            onClick={next}
          >
            <ChevronRight size={22} />
          </button>

          <button
            className="icon-button"
            title="Last step"
            aria-label="Last step"
            disabled={!hasSteps || isLast}
            onClick={finish}
          >
            <SkipForward size={17} />
          </button>
        </div>

        <select
          className="playback-speed"
          aria-label="Playback speed"
          value={speed}
          onChange={(event) =>
            setSpeed(Number(event.target.value))
          }
        >
          <option value={0.25}>0.25x</option>
          <option value={0.5}>0.5x</option>
          <option value={1}>1x</option>
          <option value={1.5}>1.5x</option>
          <option value={2}>2x</option>
          <option value={4}>4x</option>
        </select>
      </div>
    </div>
  )
}
