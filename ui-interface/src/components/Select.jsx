import { ChevronDown } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'

// Custom dropdown that replaces the native <select> — the browser's own
// option list can't be themed (it renders as a plain white OS popup even in
// dark mode), so this renders its own menu instead, styled from the same
// CSS custom properties as everything else in the app.
export default function Select({ value, onChange, options, placeholder, className = '', style, title }) {
  const [open, setOpen] = useState(false)
  const rootRef = useRef(null)

  useEffect(() => {
    if (!open) return
    const handlePointerDown = (e) => {
      if (rootRef.current && !rootRef.current.contains(e.target)) setOpen(false)
    }
    const handleKeyDown = (e) => {
      if (e.key === 'Escape') setOpen(false)
    }
    document.addEventListener('mousedown', handlePointerDown)
    document.addEventListener('keydown', handleKeyDown)
    return () => {
      document.removeEventListener('mousedown', handlePointerDown)
      document.removeEventListener('keydown', handleKeyDown)
    }
  }, [open])

  const normalized = options.map((o) => (typeof o === 'object' && o !== null ? o : { value: o, label: o || placeholder || '' }))
  const current = normalized.find((o) => o.value === value)

  return (
    <div className={`select ${className}`} style={style} ref={rootRef}>
      <button
        type="button"
        className="select-trigger"
        title={title}
        onClick={() => setOpen((v) => !v)}
        aria-haspopup="listbox"
        aria-expanded={open}
      >
        <span className="select-value">{current?.label ?? placeholder}</span>
        <ChevronDown size={14} className={`select-chevron${open ? ' select-chevron-open' : ''}`} />
      </button>
      {open && (
        <ul className="select-menu" role="listbox">
          {normalized.map((o) => (
            <li
              key={o.value || 'empty'}
              role="option"
              aria-selected={o.value === value}
              className={`select-option${o.value === value ? ' select-option-active' : ''}`}
              onClick={() => {
                onChange(o.value)
                setOpen(false)
              }}
            >
              {o.label}
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
