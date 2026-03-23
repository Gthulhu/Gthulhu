import React, { useEffect, useRef } from 'react';
import { X } from 'lucide-react';

export default function SlidePanel({ open, onClose, title, children, width = 500 }) {
  const panelRef = useRef(null);

  useEffect(() => {
    if (!open) return;
    const handleEsc = (e) => {
      if (e.key === 'Escape') onClose();
    };
    document.addEventListener('keydown', handleEsc);
    return () => document.removeEventListener('keydown', handleEsc);
  }, [open, onClose]);

  return (
    <div className={`slide-panel-overlay${open ? ' open' : ''}`} onClick={onClose}>
      <div
        ref={panelRef}
        className="slide-panel"
        style={{ width: `${width}px` }}
        onClick={(e) => e.stopPropagation()}
      >
        <div className="slide-panel-header">
          <h3 className="slide-panel-title">{title}</h3>
          <button className="slide-panel-close" onClick={onClose}>
            <X size={18} />
          </button>
        </div>
        <div className="slide-panel-body">{children}</div>
      </div>
    </div>
  );
}
