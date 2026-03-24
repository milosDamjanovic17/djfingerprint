import { setActivePinia, createPinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useLibraryStore } from '@/stores/library'
import * as songsApi from '@/api/songs'

const mockSong: songsApi.Song = {
  id: 1,
  title: 'Test Song',
  artist: 'Test Artist',
  album: null,
  duration_ms: 180000,
  source: 'manual',
  spotify_id: null,
  file_hash: 'abc123',
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
}

describe('useLibraryStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.restoreAllMocks()
  })

  it('initial state is empty', () => {
    const store = useLibraryStore()
    expect(store.songs).toEqual([])
    expect(store.total).toBe(0)
    expect(store.loading).toBe(false)
    expect(store.error).toBeNull()
  })

  it('fetchSongs populates songs and total', async () => {
    const store = useLibraryStore()

    vi.spyOn(songsApi, 'listSongs').mockResolvedValue({
      songs: [mockSong],
      total: 1,
      page: 1,
      page_size: 20,
    })

    await store.fetchSongs(1)

    expect(store.songs).toHaveLength(1)
    expect(store.songs.at(0)?.title).toBe('Test Song')
    expect(store.total).toBe(1)
    expect(store.loading).toBe(false)
    expect(store.error).toBeNull()
  })

  it('fetchSongs sets error on failure', async () => {
    const store = useLibraryStore()

    vi.spyOn(songsApi, 'listSongs').mockRejectedValue(new Error('Network error'))

    await store.fetchSongs(1)

    expect(store.songs).toHaveLength(0)
    expect(store.error).toBe('Network error')
    expect(store.loading).toBe(false)
  })

  it('removeSong removes song from list and decrements total', async () => {
    const store = useLibraryStore()

    vi.spyOn(songsApi, 'listSongs').mockResolvedValue({
      songs: [mockSong],
      total: 1,
      page: 1,
      page_size: 20,
    })

    await store.fetchSongs(1)

    vi.spyOn(songsApi, 'deleteSong').mockResolvedValue()

    await store.removeSong(1)

    expect(store.songs).toHaveLength(0)
    expect(store.total).toBe(0)
    expect(store.deletingId).toBeNull()
  })

  it('removeSong sets error on failure', async () => {
    const store = useLibraryStore()
    store.songs.push(mockSong)
    store.total = 1

    vi.spyOn(songsApi, 'deleteSong').mockRejectedValue(new Error('Delete failed'))

    await store.removeSong(1)

    expect(store.error).toBe('Delete failed')
    expect(store.songs).toHaveLength(1)
  })

  it('totalPages computes correctly', async () => {
    const store = useLibraryStore()

    vi.spyOn(songsApi, 'listSongs').mockResolvedValue({
      songs: Array(20).fill(mockSong),
      total: 45,
      page: 1,
      page_size: 20,
    })

    await store.fetchSongs(1)

    expect(store.totalPages()).toBe(3)
  })
})