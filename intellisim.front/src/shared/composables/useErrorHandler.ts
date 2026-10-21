import { useToast } from 'primevue/usetoast'
import { ApiError } from '@/shared/api/client'

export function useErrorHandler() {
  const toast = useToast()

  function handleError(error: unknown, context: string): void {
    let detail = error instanceof Error ? error.message : 'An unexpected error occurred'
    if (error instanceof ApiError && error.fields) {
      detail =
        error.message +
        '\n' +
        Object.entries(error.fields)
          .map(([f, r]) => `${f}: ${r}`)
          .join('\n')
    }
    toast.add({
      severity: 'error',
      summary: context,
      detail,
      life: 5000,
    })
    console.error(`[${context}]`, error)
  }

  function handleSuccess(message: string): void {
    toast.add({
      severity: 'success',
      summary: 'Success',
      detail: message,
      life: 3000,
    })
  }

  return { handleError, handleSuccess }
}
