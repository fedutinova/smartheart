import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';

export interface Toast {
  id: number;
  type: 'success' | 'error';
  message: string;
  link?: string;
  linkText?: string;
}

function ToastItem({ toast, onDismiss }: { toast: Toast; onDismiss: (id: number) => void }) {
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    requestAnimationFrame(() => setVisible(true));
    const timer = setTimeout(() => {
      setVisible(false);
      setTimeout(() => onDismiss(toast.id), 300);
    }, 5000);
    return () => clearTimeout(timer);
  }, [toast.id, onDismiss]);

  const bg = toast.type === 'success' ? 'bg-green-50 border-green-200' : 'bg-red-50 border-red-200';
  const text = toast.type === 'success' ? 'text-green-800' : 'text-red-800';
  const icon = toast.type === 'success' ? '✓' : '✕';
  const iconBg = toast.type === 'success' ? 'bg-green-500' : 'bg-red-500';

  return (
    <div
      className={`border rounded-lg shadow-lg p-4 flex items-start gap-3 max-w-sm transition-all duration-300 ${bg} ${
        visible ? 'opacity-100 translate-x-0' : 'opacity-0 translate-x-4'
      }`}
    >
      <span className={`flex-shrink-0 w-5 h-5 rounded-full ${iconBg} text-white flex items-center justify-center text-xs font-bold`}>
        {icon}
      </span>
      <div className="flex-1 min-w-0">
        <p className={`text-sm font-medium ${text}`}>{toast.message}</p>
        {toast.link && (
          <Link to={toast.link} className="text-xs text-rose-600 hover:text-rose-800 underline mt-1 block">
            {toast.linkText || 'Открыть'}
          </Link>
        )}
      </div>
      <button onClick={() => onDismiss(toast.id)} className="text-gray-400 hover:text-gray-600 text-sm flex-shrink-0">
        ✕
      </button>
    </div>
  );
}

export function ToastContainer({ toasts, onDismiss }: { toasts: Toast[]; onDismiss: (id: number) => void }) {
  if (toasts.length === 0) return null;

  return (
    <div className="fixed top-4 right-4 z-50 flex flex-col gap-2">
      {toasts.map((t) => (
        <ToastItem key={t.id} toast={t} onDismiss={onDismiss} />
      ))}
    </div>
  );
}
