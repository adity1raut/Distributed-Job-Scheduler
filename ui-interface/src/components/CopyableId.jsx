import { Check, Copy } from 'lucide-react'
import { useState } from 'react'
import { Link } from 'react-router-dom'
import { shortId } from '../lib/format'

export default function CopyableId({ id, to }) {
  const [copied, setCopied] = useState(false)

  const handleCopy = async (e) => {
    e.preventDefault()
    e.stopPropagation()
    try {
      await navigator.clipboard.writeText(id)
      setCopied(true)
      setTimeout(() => setCopied(false), 1200)
    } catch {
      // clipboard API unavailable — silently ignore, not worth a toast
    }
  }

  return (
    <span className="copyable-id" title={id}>
      {to ? (
        <Link to={to} className="mono">
          {shortId(id)}
        </Link>
      ) : (
        <span className="mono">{shortId(id)}</span>
      )}
      <button className="copy-btn" onClick={handleCopy} aria-label="Copy full ID" type="button">
        {copied ? <Check size={12} /> : <Copy size={12} />}
      </button>
    </span>
  )
}
