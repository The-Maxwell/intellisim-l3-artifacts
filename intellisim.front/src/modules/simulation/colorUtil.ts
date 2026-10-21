// PrimeVue's hex ColorPicker emits the hex WITHOUT the leading '#'; restore it so
// the stored value is a usable CSS hex. Shared by AgentInstanceTable and
// EnvironmentSection (mirrors PropertyEditor.normalizeHex in the model module).
export function normalizeHex(v: unknown): string {
  const s = String(v ?? '')
  return s.startsWith('#') ? s : `#${s}`
}
