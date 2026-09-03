import { Workflow } from 'lucide-react'

export default function Wordmark() {
  return (
    <span className="brand-mark">
      <span className="brand-icon">
        <Workflow size={16} strokeWidth={2.5} />
      </span>
      <span className="brand-name">
        <span className="accent">job</span>scheduler
      </span>
    </span>
  )
}
