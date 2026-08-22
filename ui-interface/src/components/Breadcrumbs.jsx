import { ChevronRight } from 'lucide-react'
import { Link } from 'react-router-dom'

export default function Breadcrumbs({ items }) {
  return (
    <nav className="breadcrumbs" aria-label="Breadcrumb">
      {items.map((item, i) => {
        const isLast = i === items.length - 1
        return (
          <span className="breadcrumb-item" key={i}>
            {item.to && !isLast ? (
              <Link to={item.to}>{item.label}</Link>
            ) : (
              <span className={isLast ? 'breadcrumb-current' : ''}>{item.label}</span>
            )}
            {!isLast && <ChevronRight size={13} className="breadcrumb-sep" />}
          </span>
        )
      })}
    </nav>
  )
}
