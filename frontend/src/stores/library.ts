import { defineStore } from 'pinia'
import { ref } from 'vue'
import { listSongs, deleteSong, type Song } from '@/api/songs'

export const useLibraryStore = defineStore('library', () => {
  const songs = ref<Song[]>([])
  const total = ref(0)
  const page = ref(1)
  const pageSize = ref(20)
  const loading = ref(false)
  const error = ref<string | null>(null)
  const deletingId = ref<number | null>(null)

  async function fetchSongs(p = 1): Promise<void> {
    loading.value = true
    error.value = null
    page.value = p

    try {
      const res = await listSongs(p, pageSize.value)
      songs.value = res.songs
      total.value = res.total
    } catch (err: unknown) {
      error.value = err instanceof Error ? err.message : 'Failed to load songs.'
    } finally {
      loading.value = false
    }
  }

  async function removeSong(id: number): Promise<void> {
    deletingId.value = id

    try {
      await deleteSong(id)
      songs.value = songs.value.filter(s => s.id !== id)
      total.value = Math.max(0, total.value - 1)
    } catch (err: unknown) {
      error.value = err instanceof Error ? err.message : 'Failed to delete song.'
    } finally {
      deletingId.value = null
    }
  }

  const totalPages = () => Math.ceil(total.value / pageSize.value)

  return {
    songs,
    total,
    page,
    pageSize,
    loading,
    error,
    deletingId,
    fetchSongs,
    removeSong,
    totalPages,
  }
})