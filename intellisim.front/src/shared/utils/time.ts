// Shared time/duration formatting helpers. Timestamps elsewhere are formatted
// ad-hoc with `new Date(iso).toLocaleString(...)`; this module owns the one thing
// that had no helper — human-readable elapsed/run durations.

// formatDuration renders a millisecond duration compactly:
//   0 / negative / non-finite → "0s"
//   < 1 min  → "45s"
//   < 1 hour → "1m 05s"
//   ≥ 1 hour → "1h 02m 03s"
// Seconds/minutes are zero-padded only when they follow a larger unit so single
// values read naturally ("45s", not "45s"; "1m 05s", not "1m 5s").
export function formatDuration(ms: number): string {
  if (!Number.isFinite(ms) || ms <= 0) return '0s'
  const totalSec = Math.floor(ms / 1000)
  const h = Math.floor(totalSec / 3600)
  const m = Math.floor((totalSec % 3600) / 60)
  const s = totalSec % 60
  if (h > 0) {
    return `${h}h ${String(m).padStart(2, '0')}m ${String(s).padStart(2, '0')}s`
  }
  if (m > 0) {
    return `${m}m ${String(s).padStart(2, '0')}s`
  }
  return `${s}s`
}

// formatTimestamp renders an ISO timestamp as a locale date-time (or '—' when
// absent/invalid). Mirrors the inline formatters used across the sim panels.
export function formatTimestamp(iso: string | undefined | null): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return d.toLocaleString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}
