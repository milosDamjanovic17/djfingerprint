import { ref } from 'vue'

export type RecorderState = 'idle' | 'recording' | 'stopped' | 'error'
export type AudioSource = 'microphone' | 'browser'

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

  async function start(source: AudioSource = 'microphone'): Promise<void> {
    error.value = null
    chunks = []

    if (source === 'browser') {
      stream = await startBrowserCapture()
    } else {
      stream = await startMicCapture()
    }

    if (!stream) return // error already set

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

  async function startMicCapture(): Promise<MediaStream | null> {
    try {
      return await navigator.mediaDevices.getUserMedia({ audio: true, video: false })
    } catch {
      error.value = 'Microphone access denied. Please allow microphone access and try again.'
      state.value = 'error'
      return null
    }
  }

  async function startBrowserCapture(): Promise<MediaStream | null> {
    // getDisplayMedia lets the user pick a tab / window / screen.
    // Audio capture is supported in Chrome and Edge; Firefox and Safari have limited support.
    if (!navigator.mediaDevices?.getDisplayMedia) {
      error.value = 'Browser audio capture is not supported in this browser. Try Chrome or Edge.'
      state.value = 'error'
      return null
    }

    let displayStream: MediaStream
    try {
      // Ask for both video and audio — some browsers require video:true to
      // allow audio capture at all. We discard the video tracks immediately after.
      displayStream = await navigator.mediaDevices.getDisplayMedia({
        video: true,
        audio: true,
      })
    } catch {
      error.value = 'Screen/audio share cancelled or denied.'
      state.value = 'error'
      return null
    }

    // Stop video tracks right away — we only need audio.
    displayStream.getVideoTracks().forEach(t => t.stop())

    const audioTracks = displayStream.getAudioTracks()
    if (audioTracks.length === 0) {
      error.value =
        'No audio captured. Make sure to check "Share audio" (or "Share tab audio") in the browser dialog.'
      state.value = 'error'
      displayStream.getTracks().forEach(t => t.stop())
      return null
    }

    // Build an audio-only stream from the captured tracks.
    return new MediaStream(audioTracks)
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
