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
    <div className="metric-grid">
      {Array.from({ length: count }).map((_, i) => (
        <div key={i}>
          <span className="skeleton skeleton-bar" style={{ width: 44, height: 27 }} />
          <span className="skeleton skeleton-bar" style={{ width: 60, height: 10, marginTop: 6 }} />
        </div>
      ))}
    </div>
  )
}
