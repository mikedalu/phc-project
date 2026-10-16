import { CloudOff, RefreshCw, TriangleAlert, Check } from 'lucide-react';
import { cn } from '@/lib/cn';
import { useSyncState, type SyncStatus } from '@/sync/status';

/**
 * The one place the user is told whether their work has left this machine.
 *
 * Design rules (docs/03-UI-PATTERNS.md §4):
 *  - `offline` is a NEUTRAL state, never red. Seeing red makes a user stop
 *    working, and stopping work is precisely what this system must prevent.
 *  - `pending` is also not an error. Records are safe on the device.
 *  - The indicator never blocks, dims, or interrupts a form.
 *  - Status is conveyed by icon AND text, never by colour alone.
 */

interface Appearance {
  label: string;
  /** Tailwind classes. Colours come from tokens only. */
  tone: string;
  Icon: typeof Check;
  spin?: boolean;
}

function appearance(status: SyncStatus, pendingCount: number): Appearance {
  switch (status) {
    case 'synced':
      return {
        label: 'All records synced',
        tone: 'bg-success-soft text-success-fg',
        Icon: Check,
      };
    case 'syncing':
      return {
        label: 'Syncing…',
        tone: 'bg-info-soft text-info-fg',
        Icon: RefreshCw,
        spin: true,
      };
    case 'pending':
      return {
        label: `${pendingCount} waiting to sync`,
        tone: 'bg-primary-soft text-primary-fg',
        Icon: RefreshCw,
      };
    case 'offline':
      return {
        // Not "error", not "failed". The work is saved locally and that is fine.
        label:
          pendingCount > 0
            ? `Offline — ${pendingCount} saved on this device`
            : 'Offline — working locally',
        tone: 'bg-warning-soft text-warning-fg',
        Icon: CloudOff,
      };
    case 'blocked':
      return {
        label: 'Reconnect required',
        tone: 'bg-danger-soft text-danger-fg',
        Icon: TriangleAlert,
      };
    case 'error':
      return {
        label: 'Sync problem',
        tone: 'bg-danger-soft text-danger-fg',
        Icon: TriangleAlert,
      };
  }
}

export function SyncStatusIndicator({ className }: { className?: string }) {
  const { status, pendingCount } = useSyncState();
  const { label, tone, Icon, spin } = appearance(status, pendingCount);

  return (
    <div
      // aria-live so a screen reader announces a change without stealing focus.
      // Not `assertive`: sync status must never interrupt data entry.
      aria-live="polite"
      role="status"
      className={cn(
        'inline-flex items-center gap-1.5 rounded-sm px-2 py-1',
        'text-xs font-medium',
        tone,
        className,
      )}
      title={label}
    >
      <Icon
        aria-hidden="true"
        className={cn('size-3.5 shrink-0', spin && 'motion-safe:animate-spin')}
      />
      <span className="hidden sm:inline">{label}</span>
    </div>
  );
}
