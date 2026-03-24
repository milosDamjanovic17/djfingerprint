import { defineStore } from 'pinia'
import { ref } from 'vue'
import { identifySong, type IdentifyMatch } from '@/api/identify'

export type IdentifyPhase =
  | 'idle'
  | 'recording'
  | 'processing'
  | 'match'
  | 'no-match'
  | 'error'

export const useIdentifyStore = defineStore('identify', () => {
  const phase = ref<IdentifyPhase>('idle')
  const match = ref<IdentifyMatch | null>(null)
  const error = ref<string | null>(null)
  const secondsLeft = ref(0)

  let countdownTimer: ReturnType<typeof setInterval> | null = null

  function startCountdown(seconds: number, onDone: () => void): void {
    secondsLeft.value = seconds
    countdownTimer = setInterval(() => {
      secondsLeft.value -= 1
      if (secondsLeft.value <= 0) {
        clearCountdown()
        onDone()
      }
    }, 1000)
  }

  function clearCountdown(): void {
    if (countdownTimer !== null) {
      clearInterval(countdownTimer)
      countdownTimer = null
    }
  }

  async function submitBlob(blob: Blob): Promise<void> {
    phase.value = 'processing'
    error.value = null
    match.value = null

    try {
      const res = await identifySong(blob)
      const top = res.matches[0]
      if (res.found && top !== undefined) {
        match.value = {
          id: top.song.id,
          title: top.song.title,
          artist: top.song.artist,
          album: top.song.album,
          duration_ms: top.song.duration_ms,
          spotify_id: top.song.spotify_id,
          confidence: confidenceToNumber(res.confidence),
          score: top.score,
          offset_secs: top.offset_secs,
        }
        phase.value = 'match'
      } else {
        phase.value = 'no-match'
      }
    } catch (err: unknown) {
      error.value = err instanceof Error ? err.message : 'Identification failed.'
      phase.value = 'error'
    }
  }

  function reset(): void {
    clearCountdown()
    phase.value = 'idle'
    match.value = null
    error.value = null
    secondsLeft.value = 0
  }

  function setError(message: string): void {
    error.value = message
    phase.value = 'error'
  }

  function setRecording(): void {
    phase.value = 'recording'
  }

  return {
    phase,
    match,
    error,
    secondsLeft,
    startCountdown,
    clearCountdown,
    submitBlob,
    reset,
    setError,
    setRecording,
  }
})

// Maps the backend's string confidence label to a 0–1 float for the progress bar
function confidenceToNumber(confidence: string): number {
  switch (confidence) {
    case 'high':   return 0.9
    case 'medium': return 0.65
    case 'low':    return 0.3
    default:       return 0
  }
}
