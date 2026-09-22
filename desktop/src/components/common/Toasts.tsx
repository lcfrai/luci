// The optimistic-mutation Undo window. Contract §6.7. Plain text only.
import { useEffect } from "react";
import type { JSX } from "react";
import { useShallow } from "zustand/react/shallow";
import { useCockpit } from "../../store/store";
import type { Toast } from "../../store/types";
import "./common.css";

function ToastRow(props: { toast: Toast; onDismiss: (id: string) => void }): JSX.Element {
  const { toast, onDismiss } = props;

  useEffect(() => {
    const remaining = Math.max(0, toast.createdAt + toast.ttl - Date.now());
    const timer = setTimeout(() => onDismiss(toast.id), remaining);
    return () => clearTimeout(timer);
  }, [toast.id, toast.createdAt, toast.ttl, onDismiss]);

  return (
    <div className="toast" role="status">
      <span className="toast-text">{toast.message}</span>
      {toast.undo === undefined ? null : (
        <button
          type="button"
          className="toast-undo"
          onClick={() => {
            toast.undo?.();
            onDismiss(toast.id);
          }}
        >
          Undo
        </button>
      )}
    </div>
  );
}

export function Toasts(): JSX.Element {
  const toasts = useCockpit(useShallow((s) => s.toasts));
  const dismissToast = useCockpit((s) => s.dismissToast);

  return (
    <div className="toast-stack" aria-live="polite">
      {toasts.map((toast) => (
        <ToastRow key={toast.id} toast={toast} onDismiss={dismissToast} />
      ))}
    </div>
  );
}
