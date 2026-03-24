import { describe, expect, it, vi, beforeEach } from 'vitest'
import * as songsApi from '@/api/songs'
import client from '@/api/client'

describe('songs API', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('uploadSong posts to /api/v1/upload with FormData', async () => {
    const mockData = {
      id: 1, title: 'Song', artist: 'Artist', album: null,
      duration_ms: null, source: 'manual', file_hash: 'hash',
      created_at: '', updated_at: '',
    }

    vi.spyOn(client, 'post').mockResolvedValue({ data: mockData })

    const file = new File(['audio'], 'track.mp3', { type: 'audio/mpeg' })
    const result = await songsApi.uploadSong(file)

    expect(client.post).toHaveBeenCalledWith(
      '/api/v1/upload',
      expect.any(FormData),
      expect.objectContaining({ headers: { 'Content-Type': 'multipart/form-data' } }),
    )
    expect(result).toEqual(mockData)
  })

  it('listSongs calls GET /api/v1/songs with page params', async () => {
    const mockData = { songs: [], total: 0, page: 1, page_size: 20 }

    vi.spyOn(client, 'get').mockResolvedValue({ data: mockData })

    const result = await songsApi.listSongs(1, 20)

    expect(client.get).toHaveBeenCalledWith('/api/v1/songs', {
      params: { page: 1, page_size: 20 },
    })
    expect(result).toEqual(mockData)
  })

  it('getSong calls GET /api/v1/songs/:id', async () => {
    const mockSong = {
      id: 5, title: 'T', artist: 'A', album: null,
      duration_ms: null, source: 'manual', spotify_id: null,
      file_hash: 'h', created_at: '', updated_at: '',
    }

    vi.spyOn(client, 'get').mockResolvedValue({ data: mockSong })

    const result = await songsApi.getSong(5)

    expect(client.get).toHaveBeenCalledWith('/api/v1/songs/5')
    expect(result).toEqual(mockSong)
  })

  it('deleteSong calls DELETE /api/v1/songs/:id', async () => {
    vi.spyOn(client, 'delete').mockResolvedValue({ data: null })

    await songsApi.deleteSong(3)

    expect(client.delete).toHaveBeenCalledWith('/api/v1/songs/3')
  })
})