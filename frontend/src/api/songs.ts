import client from '@/api/client'

export interface Song {
  id: number
  title: string | null
  artist: string | null
  album: string | null
  duration_ms: number | null
  source: string
  spotify_id: string | null
  created_at: string
}

export interface UploadResponse {
  id: number
  title: string | null
  artist: string | null
  album: string | null
  duration_ms: number | null
  source: string
  spotify_id: string | null
  created_at: string
}

export interface ListSongsResponse {
  songs: Song[]
  total: number
  page: number
  page_size: number
}

export async function uploadSong(file: File): Promise<UploadResponse> {
  const form = new FormData()
  form.append('file', file)

  const { data } = await client.post<UploadResponse>('/api/v1/upload', form, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })

  return data
}

export async function listSongs(page = 1, pageSize = 20): Promise<ListSongsResponse> {
  const { data } = await client.get<ListSongsResponse>('/api/v1/songs', {
    params: { page, page_size: pageSize },
  })
  return data
}

export async function getSong(id: number): Promise<Song> {
  const { data } = await client.get<Song>(`/api/v1/songs/${id}`)
  return data
}

export async function deleteSong(id: number): Promise<void> {
  await client.delete(`/api/v1/songs/${id}`)
}