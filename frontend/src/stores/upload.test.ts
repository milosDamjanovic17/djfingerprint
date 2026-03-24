import { setActivePinia, createPinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useUploadStore } from '@/stores/upload'
import * as songsApi from '@/api/songs'

describe('useUploadStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('initial state is idle', () => {
    const store = useUploadStore()
    expect(store.status).toBe('idle')
    expect(store.result).toBeNull()
    expect(store.error).toBeNull()
    expect(store.progress).toBe(0)
  })

  it('sets status to uploading while request is in flight', async () => {
    const store = useUploadStore()

    vi.spyOn(songsApi, 'uploadSong').mockImplementation(
      () => new Promise(() => {})
    )

    store.upload(new File([''], 'test.mp3', { type: 'audio/mpeg' }))
    expect(store.status).toBe('uploading')
  })

  it('sets status to success and stores result on resolution', async () => {
    const store = useUploadStore()

    const mockResult: songsApi.UploadResponse = {
      id: 1,
      title: 'Test Song',
      artist: 'Test Artist',
      album: null,
      duration_ms: 180000,
      source: 'manual',
      file_hash: 'abc123',
      created_at: '2024-01-01T00:00:00Z',
      updated_at: '2024-01-01T00:00:00Z',
    }

    vi.spyOn(songsApi, 'uploadSong').mockResolvedValue(mockResult)

    await store.upload(new File([''], 'test.mp3', { type: 'audio/mpeg' }))

    expect(store.status).toBe('success')
    expect(store.result).toEqual(mockResult)
    expect(store.error).toBeNull()
    expect(store.progress).toBe(100)
  })

  it('sets status to error and stores message on rejection', async () => {
    const store = useUploadStore()

    vi.spyOn(songsApi, 'uploadSong').mockRejectedValue(new Error('Network error'))

    await store.upload(new File([''], 'test.mp3', { type: 'audio/mpeg' }))

    expect(store.status).toBe('error')
    expect(store.error).toBe('Network error')
    expect(store.result).toBeNull()
  })

  it('extracts error message from axios response body', async () => {
    const store = useUploadStore()

    const axiosError = {
      response: { data: { error: 'file too large' } },
    }

    vi.spyOn(songsApi, 'uploadSong').mockRejectedValue(axiosError)

    await store.upload(new File([''], 'test.mp3', { type: 'audio/mpeg' }))

    expect(store.error).toBe('file too large')
  })

  it('reset returns store to idle state', async () => {
    const store = useUploadStore()

    vi.spyOn(songsApi, 'uploadSong').mockResolvedValue({
      id: 1, title: null, artist: null, album: null,
      duration_ms: null, source: 'manual', file_hash: 'x',
      created_at: '', updated_at: '',
    })

    await store.upload(new File([''], 'test.mp3', { type: 'audio/mpeg' }))
    expect(store.status).toBe('success')

    store.reset()

    expect(store.status).toBe('idle')
    expect(store.result).toBeNull()
    expect(store.error).toBeNull()
    expect(store.progress).toBe(0)
  })
})