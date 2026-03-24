import { setActivePinia, createPinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'  // ← add afterEach
import { useIdentifyStore } from '@/stores/identify'
import * as identifyApi from '@/api/identify'

const mockMatch: identifyApi.IdentifyMatch = {
  id: 1,
  title: 'Test Song',
  artist: 'Test Artist',
  album: 'Test Album',
  duration_ms: 210000,
  spotify_id: 'abc123',
  confidence: 0.92,
}

describe('useIdentifyStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.restoreAllMocks()
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('initial state is idle', () => {
    const store = useIdentifyStore()
    expect(store.phase).toBe('idle')
    expect(store.match).toBeNull()
    expect(store.error).toBeNull()
    expect(store.secondsLeft).toBe(0)
  })

  it('reset returns to idle', () => {
    const store = useIdentifyStore()
    store.setError('something went wrong')
    store.reset()
    expect(store.phase).toBe('idle')
    expect(store.error).toBeNull()
    expect(store.match).toBeNull()
  })

  it('setError sets error phase and message', () => {
    const store = useIdentifyStore()
    store.setError('mic denied')
    expect(store.phase).toBe('error')
    expect(store.error).toBe('mic denied')
  })

  it('setRecording sets recording phase', () => {
    const store = useIdentifyStore()
    store.setRecording()
    expect(store.phase).toBe('recording')
  })

  it('startCountdown counts down and calls onDone', () => {
    const store = useIdentifyStore()
    const onDone = vi.fn()

    store.startCountdown(3, onDone)
    expect(store.secondsLeft).toBe(3)

    vi.advanceTimersByTime(1000)
    expect(store.secondsLeft).toBe(2)

    vi.advanceTimersByTime(1000)
    expect(store.secondsLeft).toBe(1)

    vi.advanceTimersByTime(1000)
    expect(store.secondsLeft).toBe(0)
    expect(onDone).toHaveBeenCalledOnce()
  })

  it('submitBlob sets match phase on successful response', async () => {
    const store = useIdentifyStore()

    vi.spyOn(identifyApi, 'identifySong').mockResolvedValue({ match: mockMatch })

    await store.submitBlob(new Blob(['audio'], { type: 'audio/webm' }))

    expect(store.phase).toBe('match')
    expect(store.match).toEqual(mockMatch)
    expect(store.error).toBeNull()
  })

  it('submitBlob sets no-match phase when match is null', async () => {
    const store = useIdentifyStore()

    vi.spyOn(identifyApi, 'identifySong').mockResolvedValue({ match: null })

    await store.submitBlob(new Blob(['audio'], { type: 'audio/webm' }))

    expect(store.phase).toBe('no-match')
    expect(store.match).toBeNull()
  })

  it('submitBlob sets error phase on API failure', async () => {
    const store = useIdentifyStore()

    vi.spyOn(identifyApi, 'identifySong').mockRejectedValue(new Error('Server error'))

    await store.submitBlob(new Blob(['audio'], { type: 'audio/webm' }))

    expect(store.phase).toBe('error')
    expect(store.error).toBe('Server error')
    expect(store.match).toBeNull()
  })

  it('clearCountdown stops the timer', () => {
    const store = useIdentifyStore()
    const onDone = vi.fn()

    store.startCountdown(5, onDone)
    store.clearCountdown()

    vi.advanceTimersByTime(10_000)
    expect(onDone).not.toHaveBeenCalled()
  })
})