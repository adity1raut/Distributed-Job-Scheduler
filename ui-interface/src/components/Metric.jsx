export default function Metric({ label, value, tone }) {
  return (
    <div className={tone ? `metric-${tone}` : undefined}>
      <div className="metric-value">{value ?? '—'}</div>
      <div className="metric-label">{label}</div>
    </div>
  )
}
