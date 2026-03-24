import client from '@/api/client'

// ── Backend response types ────────────────────────────────────────────────────

export interface SongItem {
  id: number
  title: string | null
  artist: string | null
  album: string | null
  duration_ms: number | null
  source: string
  spotify_id: string | null
  created_at: string
}

export interface MatchItem {
  rank: number
  song: SongItem
  score: number
  offset_secs: number
}

export interface IdentifyApiResponse {
  found: boolean
  confidence: string // "high" | "medium" | "low" | ""
  matches: MatchItem[]
  processed_in: string
}

// ── Flattened match used by the store and view ────────────────────────────────

export interface IdentifyMatch {
  id: number
  title: string | null
  artist: string | null
  album: string | null
  duration_ms: number | null
  spotify_id: string | null
  confidence: number  // 0.0 – 1.0, derived from backend confidence string
  score: number
  offset_secs: number
}

export async function identifySong(blob: Blob): Promise<IdentifyApiResponse> {
  const form = new FormData()
  form.append('file', blob, 'recording.webm')

  const { data } = await client.post<IdentifyApiResponse>('/api/v1/identify', form, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })

  return data
}
