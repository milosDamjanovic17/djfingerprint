import { defineStore } from 'pinia'
import { ref } from 'vue'
import { uploadSong, type UploadResponse } from '@/api/songs'

export type UploadStatus = 'idle' | 'uploading' | 'success' | 'error'

export const useUploadStore = defineStore('upload', () => {
  const status = ref<UploadStatus>('idle')
  const result = ref<UploadResponse | null>(null)
  const error = ref<string | null>(null)
  const progress = ref<number>(0)

  async function upload(file: File): Promise<void> {
    status.value = 'uploading'
    result.value = null
    error.value = null
    progress.value = 0

    try {
      result.value = await uploadSong(file)
      status.value = 'success'
      progress.value = 100
    } catch (err: unknown) {
      status.value = 'error'
      error.value = extractMessage(err)
    }
  }

  function reset(): void {
    status.value = 'idle'
    result.value = null
    error.value = null
    progress.value = 0
  }

  return { status, result, error, progress, upload, reset }
})

function extractMessage(err: unknown): string {
  if (
    typeof err === 'object' &&
    err !== null &&
    'response' in err &&
    typeof (err as { response?: { data?: { error?: string } } }).response?.data?.error === 'string'
  ) {
    return (err as { response: { data: { error: string } } }).response.data.error
  }
  if (err instanceof Error) return err.message
  return 'Upload failed. Please try again.'
}