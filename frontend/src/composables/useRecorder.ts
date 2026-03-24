import { ref } from 'vue'

export type RecorderState = 'idle' | 'recording' | 'stopped' | 'error'

export function useRecorder() {
  const state = ref<RecorderState>('idle')
  const error = ref<string | null>(null)

  let mediaRecorder: MediaRecorder | null = null
  let stream: MediaStream | null = null
  let chunks: BlobEvent['data'][] = []

  function getSupportedMimeType(): string {
    const candidates = [
      'audio/webm;codecs=opus',
      'audio/webm',
      'audio/ogg;codecs=opus',
      'audio/ogg',
    ]
    return candidates.find(t => MediaRecorder.isTypeSupported(t)) ?? ''
  }

  async function start(): Promise<void> {
    error.value = null
    chunks = []

    try {
      stream = await navigator.mediaDevices.getUserMedia({ audio: true, video: false })
    } catch {
      error.value = 'Microphone access denied. Please allow microphone access and try again.'
      state.value = 'error'
      return
    }

    const mimeType = getSupportedMimeType()

    try {
      mediaRecorder = new MediaRecorder(stream, mimeType ? { mimeType } : undefined)
    } catch {
      error.value = 'Your browser does not support audio recording.'
      state.value = 'error'
      stopStream()
      return
    }

    mediaRecorder.ondataavailable = (e: BlobEvent) => {
      if (e.data.size > 0) chunks.push(e.data)
    }

    mediaRecorder.start(100) // collect chunks every 100 ms
    state.value = 'recording'
  }

  function stop(): Promise<Blob> {
    return new Promise((resolve, reject) => {
      if (!mediaRecorder || state.value !== 'recording') {
        reject(new Error('Recorder is not running.'))
        return
      }

      mediaRecorder.onstop = () => {
        const mimeType = mediaRecorder?.mimeType ?? 'audio/webm'
        const blob = new Blob(chunks, { type: mimeType })
        stopStream()
        state.value = 'stopped'
        resolve(blob)
      }

      mediaRecorder.stop()
    })
  }

  function reset(): void {
    stopStream()
    chunks = []
    mediaRecorder = null
    state.value = 'idle'
    error.value = null
  }

  function stopStream(): void {
    stream?.getTracks().forEach(t => t.stop())
    stream = null
  }

  return { state, error, start, stop, reset }
}