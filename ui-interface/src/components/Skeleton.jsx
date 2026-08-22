export function SkeletonRows({ rows = 4, cols = 4 }) {
  return (
    <>
      {Array.from({ length: rows }).map((_, r) => (
        <tr key={r}>
          {Array.from({ length: cols }).map((__, c) => (
            <td key={c}>
              <span className="skeleton skeleton-bar" style={{ width: `${55 + ((r + c) % 3) * 15}%` }} />
            </td>
          ))}
        </tr>
      ))}
    </>
  )
}

export function SkeletonCards({ count = 4 }) {
  return (
    <div className="stat-grid">
      {Array.from({ length: count }).map((_, i) => (
        <div className="stat-card" key={i}>
          <span className="skeleton skeleton-bar" style={{ width: '50%', height: 11 }} />
          <span className="skeleton skeleton-bar" style={{ width: '35%', height: 24, marginTop: 10 }} />
        </div>
      ))}
    </div>
  )
}
