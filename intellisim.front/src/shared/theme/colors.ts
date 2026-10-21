export const colors = {
  primaryDark: '#18181B',
  surface: '#27272A',
  accent: '#8B5CF6',
  accentLight: '#A78BFA',
  accentDark: '#7C3AED',
  background: '#FAFAFA',
  border: '#E4E4E7',
  mutedText: '#71717A',
  text: '#18181B',
  white: '#FFFFFF',
  error: '#EF4444',
  errorLight: '#FEE2E2',
  success: '#22C55E',
  successLight: '#DCFCE7',
  warning: '#F59E0B',
  warningLight: '#FEF3C7',
  info: '#3B82F6',
  infoLight: '#DBEAFE',
} as const

export type ColorToken = keyof typeof colors
