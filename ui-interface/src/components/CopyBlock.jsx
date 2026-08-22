import { Check, Copy } from 'lucide-react'
import { useState } from 'react'

export default function CopyBlock({ text, tone }) {
  const [copied, setCopied] = useState(false)

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(text)
      setCopied(true)
      setTimeout(() => setCopied(false), 1200)
    } catch {
      // clipboard API unavailable — silently ignore, not worth a toast
    }
  }

  return (
    <div className={`code-block${tone ? ` code-block-${tone}` : ''}`}>
      <button className="code-copy-btn" onClick={handleCopy} type="button" aria-label="Copy to clipboard">
        {copied ? <Check size={12} /> : <Copy size={12} />}
      </button>
      <pre>{text}</pre>
    </div>
  )
}
