import { useMemo, useState } from 'react'

const WIDTH = 760
const HEIGHT = 200
const PAD = { top: 12, right: 12, bottom: 24, left: 34 }
const PLOT_W = WIDTH - PAD.left - PAD.right
const PLOT_H = HEIGHT - PAD.top - PAD.bottom
const MAX_BAR_W = 24
const GAP = 2 // surface gap between stacked segments
const TICK_COUNT = 4

function niceMax(value) {
  if (value <= 0) return TICK_COUNT
  const magnitude = Math.pow(10, Math.floor(Math.log10(value)))
  const steps = [1, 2, 2.5, 5, 10]
  for (const step of steps) {
    const candidate = step * magnitude
    // Keeping the max at least TICK_COUNT means every gridline step is >= 1,
    // so rounding two ticks to the same integer (a duplicate React key, and
    // a duplicate-looking label) can't happen for small data ranges.
    if (candidate >= value) return Math.max(candidate, TICK_COUNT)
  }
  return Math.max(10 * magnitude, TICK_COUNT)
}

// A <rect> with rx rounds every corner — this draws a bar with rounded top
// corners only, square at the baseline (or at the stack seam), per the
// "4px rounded data-end, square at the baseline" mark spec.
function roundedTopPath(x, y, w, h, r) {
  const radius = Math.max(0, Math.min(r, w / 2, h))
  if (radius === 0) return `M${x},${y} h${w} v${h} h${-w} Z`
  return `M${x},${y + radius}
    a${radius},${radius} 0 0 1 ${radius},${-radius}
    h${w - 2 * radius}
    a${radius},${radius} 0 0 1 ${radius},${radius}
    v${h - radius}
    h${-w}
    Z`
}

function formatHour(iso) {
  return new Date(iso).toLocaleTimeString([], { hour: 'numeric' })
}

export default function ThroughputChart({ buckets }) {
  const [hovered, setHovered] = useState(null)

  const { bars, yMax, ticks } = useMemo(() => {
    const totals = buckets.map((b) => b.completed + b.failed)
    const max = niceMax(Math.max(...totals, 0))
    const slotW = PLOT_W / buckets.length
    const barW = Math.min(MAX_BAR_W, slotW * 0.6)

    const scaledBars = buckets.map((b, i) => {
      const completedH = max === 0 ? 0 : (b.completed / max) * PLOT_H
      const failedH = max === 0 ? 0 : (b.failed / max) * PLOT_H
      const x = PAD.left + i * slotW + (slotW - barW) / 2
      const baseline = PAD.top + PLOT_H
      const hasFailed = b.failed > 0
      const completedY = baseline - completedH
      const failedY = completedY - (hasFailed ? GAP : 0) - failedH
      return { ...b, x, barW, completedY, completedH, failedY, failedH, hasFailed, baseline }
    })

    const tickValues = Array.from({ length: TICK_COUNT + 1 }, (_, i) => Math.round((max / TICK_COUNT) * i))

    return { bars: scaledBars, yMax: max, ticks: tickValues }
  }, [buckets])

  const total = buckets.reduce((sum, b) => sum + b.completed + b.failed, 0)
  const labelEvery = Math.ceil(buckets.length / 6)

  return (
    <div className="throughput-chart">
      <div className="throughput-legend">
        <span className="throughput-legend-item">
          <span className="throughput-swatch" style={{ background: 'var(--success)' }} />
          Completed
        </span>
        <span className="throughput-legend-item">
          <span className="throughput-swatch" style={{ background: 'var(--danger)' }} />
          Failed
        </span>
      </div>

      {total === 0 ? (
        <p className="muted throughput-empty">No completed or failed jobs in the last 24 hours.</p>
      ) : (
        <div className="throughput-svg-wrap">
          <svg viewBox={`0 0 ${WIDTH} ${HEIGHT}`} role="img" aria-label="Completed and failed jobs per hour, last 24 hours">
            {ticks.map((value, i) => {
              const y = PAD.top + PLOT_H - (yMax === 0 ? 0 : (value / yMax) * PLOT_H)
              return (
                <g key={i}>
                  <line x1={PAD.left} x2={WIDTH - PAD.right} y1={y} y2={y} className="throughput-gridline" />
                  <text x={PAD.left - 8} y={y} className="throughput-tick" textAnchor="end" dominantBaseline="middle">
                    {value}
                  </text>
                </g>
              )
            })}

            {bars.map((bar, i) => (
              <g key={bar.hour}>
                {bar.completedH > 0 && (
                  <path
                    d={
                      bar.hasFailed
                        ? `M${bar.x},${bar.completedY} h${bar.barW} v${bar.completedH} h${-bar.barW} Z`
                        : roundedTopPath(bar.x, bar.completedY, bar.barW, bar.completedH, 4)
                    }
                    fill="var(--success)"
                  />
                )}
                {bar.failedH > 0 && (
                  <path
                    d={roundedTopPath(bar.x, bar.failedY, bar.barW, bar.failedH, 4)}
                    fill="var(--danger)"
                  />
                )}
                {i % labelEvery === 0 && (
                  <text
                    x={bar.x + bar.barW / 2}
                    y={HEIGHT - 6}
                    className="throughput-tick"
                    textAnchor="middle"
                  >
                    {formatHour(bar.hour)}
                  </text>
                )}
                {/* Hit target spans the full plot height, not just the bar, and is bigger than the painted mark. */}
                <rect
                  x={PAD.left + i * (PLOT_W / bars.length)}
                  y={PAD.top}
                  width={PLOT_W / bars.length}
                  height={PLOT_H}
                  fill="transparent"
                  onMouseEnter={() => setHovered(i)}
                  onMouseLeave={() => setHovered(null)}
                  onFocus={() => setHovered(i)}
                  onBlur={() => setHovered(null)}
                  tabIndex={0}
                  role="button"
                  aria-label={`${formatHour(bar.hour)}: ${bar.completed} completed, ${bar.failed} failed`}
                />
                {hovered === i && (
                  <rect
                    x={PAD.left + i * (PLOT_W / bars.length)}
                    y={PAD.top}
                    width={PLOT_W / bars.length}
                    height={PLOT_H}
                    fill="var(--text)"
                    opacity={0.05}
                    pointerEvents="none"
                  />
                )}
              </g>
            ))}
          </svg>

          {hovered !== null && (
            <div
              className="throughput-tooltip"
              style={{
                left: `${((hovered + 0.5) / bars.length) * 100}%`,
                transform: hovered < 2 ? 'translateX(0%)' : hovered > bars.length - 3 ? 'translateX(-100%)' : 'translateX(-50%)',
              }}
            >
              <div className="throughput-tooltip-hour">
                {new Date(bars[hovered].hour).toLocaleString([], {
                  month: 'short',
                  day: 'numeric',
                  hour: 'numeric',
                })}
              </div>
              <div className="throughput-tooltip-row">
                <span className="throughput-swatch" style={{ background: 'var(--success)' }} />
                <strong>{bars[hovered].completed}</strong> completed
              </div>
              <div className="throughput-tooltip-row">
                <span className="throughput-swatch" style={{ background: 'var(--danger)' }} />
                <strong>{bars[hovered].failed}</strong> failed
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
