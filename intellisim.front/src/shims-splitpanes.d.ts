// Minimal type shim for `splitpanes` (the package ships no .d.ts).
// Only the surface we use (Splitpanes/Pane components + the resized event) is typed.
declare module 'splitpanes' {
  import type { DefineComponent } from 'vue'
  export const Splitpanes: DefineComponent<Record<string, unknown>>
  export const Pane: DefineComponent<Record<string, unknown>>
}
