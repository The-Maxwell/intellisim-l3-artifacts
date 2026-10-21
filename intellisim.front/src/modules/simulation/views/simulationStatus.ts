import type { SimulationStatus } from '@/shared/types'

// PrimeVue Tag severities accepted by the simulation status tag. Shared by the
// list view and the editor so both colour statuses identically.
type TagSeverity = 'success' | 'info' | 'warn' | 'danger' | 'secondary'

// Map a simulation status to a Tag severity:
//   generating       → info   (work in progress)
//   ready            → success
//   generation_failed / failed → danger
//   running          → info
//   everything else  → secondary
export function statusSeverity(status: SimulationStatus): TagSeverity {
  switch (status) {
    case 'ready':
      return 'success'
    case 'generating':
    case 'running':
      return 'info'
    case 'generation_failed':
    case 'failed':
      return 'danger'
    default:
      return 'secondary'
  }
}
