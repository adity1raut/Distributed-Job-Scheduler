import { CheckCircle2, X, XCircle } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { dismissToast, subscribeToasts } from '../lib/toast'

const DURATION_MS = 3500

export default function ToastHost() {
  const [toasts, setToasts] = useState([])

  useEffect(() => subscribeToasts(setToasts), [])

  return (
    <div className="toast-host">
      {toasts.map((t) => (
        <ToastItem key={t.id} toast={t} />
      ))}
    </div>
  )
}

function ToastItem({ toast: item }) {
  const [paused, setPaused] = useState(false)
  const remainingRef = useRef(DURATION_MS)
  const startedAtRef = useRef(0)
  const timerRef = useRef(null)

  useEffect(() => {
    const arm = (ms) => {
      startedAtRef.current = Date.now()
      timerRef.current = setTimeout(() => dismissToast(item.id), ms)
    }
    arm(remainingRef.current)
    return () => clearTimeout(timerRef.current)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const handleMouseEnter = () => {
    clearTimeout(timerRef.current)
    remainingRef.current -= Date.now() - startedAtRef.current
    setPaused(true)
  }

  const handleMouseLeave = () => {
    startedAtRef.current = Date.now()
    timerRef.current = setTimeout(() => dismissToast(item.id), remainingRef.current)
    setPaused(false)
  }

  const Icon = item.type === 'success' ? CheckCircle2 : XCircle

  return (
    <div
      className={`toast toast-${item.type}`}
      role="status"
      onMouseEnter={handleMouseEnter}
      onMouseLeave={handleMouseLeave}
    >
      <Icon size={17} className="toast-icon" />
      <span className="toast-message">{item.message}</span>
      <button className="toast-close" onClick={() => dismissToast(item.id)} aria-label="Dismiss notification">
        <X size={13} />
      </button>
      <span
        className="toast-progress"
        style={{ animationDuration: `${DURATION_MS}ms`, animationPlayState: paused ? 'paused' : 'running' }}
      />
    </div>
  )
}
