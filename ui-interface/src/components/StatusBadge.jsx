const ACTIVE = new Set(['running', 'claimed'])

export default function StatusBadge({ status }) {
  return (
    <span className={`badge badge-${status}`}>
      <span className={`status-dot${ACTIVE.has(status) ? ' pulse' : ''}`} />
      {status}
    </span>
  )
}
