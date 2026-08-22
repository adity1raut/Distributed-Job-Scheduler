export default function StatCard({ label, value, tone, icon: Icon }) {
  return (
    <div className={`stat-card${tone ? ` stat-${tone}` : ''}`}>
      <div className="stat-card-top">
        <div className="stat-label">{label}</div>
        {Icon && <Icon size={15} strokeWidth={2} className="stat-icon" />}
      </div>
      <div className="stat-value">{value ?? '—'}</div>
    </div>
  )
}
