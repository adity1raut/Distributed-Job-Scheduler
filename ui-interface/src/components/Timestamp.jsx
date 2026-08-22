import { absoluteTime, relativeTime } from '../lib/format'

export default function Timestamp({ value }) {
  if (!value) return <span className="muted">—</span>
  return (
    <span className="timestamp" title={absoluteTime(value)}>
      {relativeTime(value)}
    </span>
  )
}
