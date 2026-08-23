import { TriangleAlert } from 'lucide-react'
import { useEffect, useState } from 'react'
import { subscribeConfirm } from '../lib/confirm'

export default function ConfirmHost() {
  const [request, setRequest] = useState(null)

  useEffect(() => subscribeConfirm(setRequest), [])

  const settle = (result) => {
    setRequest((current) => {
      current?.resolve(result)
      return null
    })
  }

  useEffect(() => {
    if (!request) return
    const onKeyDown = (e) => {
      if (e.key === 'Escape') settle(false)
    }
    document.addEventListener('keydown', onKeyDown)
    return () => document.removeEventListener('keydown', onKeyDown)
  }, [request])

  if (!request) return null

  return (
    <div className="confirm-backdrop" onMouseDown={() => settle(false)}>
      <div className="confirm-card" role="alertdialog" aria-modal="true" onMouseDown={(e) => e.stopPropagation()}>
        <TriangleAlert size={20} className="confirm-icon" />
        <p className="confirm-message">{request.message}</p>
        <div className="confirm-actions">
          <button className="btn-ghost" onClick={() => settle(false)}>
            {request.cancelLabel || 'Cancel'}
          </button>
          <button className="btn-danger-solid" onClick={() => settle(true)} autoFocus>
            {request.confirmLabel || 'Confirm'}
          </button>
        </div>
      </div>
    </div>
  )
}
