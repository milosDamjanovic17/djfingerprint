<script setup lang="ts">
import { onMounted, computed } from 'vue'
import { useLibraryStore } from '@/stores/library'

const store = useLibraryStore()

onMounted(() => store.fetchSongs(1))

const totalPages = computed(() => store.totalPages())

function formatDuration(ms: number | null): string {
  if (ms === null) return '—'
  const total = Math.floor(ms / 1000)
  const m = Math.floor(total / 60)
  const s = total % 60
  return `${m}:${s.toString().padStart(2, '0')}`
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString(undefined, {
    year: 'numeric', month: 'short', day: 'numeric',
  })
}
</script>

<template>
  <main class="min-h-screen bg-zinc-950 px-4 py-10">
    <div class="max-w-4xl mx-auto">

      <!-- Header -->
      <div class="mb-8 flex items-center justify-between">
        <div>
          <h1 class="text-2xl font-bold text-white tracking-tight">Song Library</h1>
          <p class="mt-1 text-sm text-zinc-400">
            {{ store.total }} song{{ store.total !== 1 ? 's' : '' }} registered
          </p>
        </div>

        <button
          class="text-sm text-zinc-400 hover:text-white transition-colors"
          :disabled="store.loading"
          @click="store.fetchSongs(store.page)"
        >
          ↺ Refresh
        </button>
      </div>

      <!-- Error -->
      <div v-if="store.error" class="mb-6 rounded-lg bg-red-900/30 border border-red-700 px-4 py-3 text-sm text-red-300">
        {{ store.error }}
      </div>

      <!-- Loading skeleton -->
      <div v-if="store.loading" class="space-y-3">
        <div
          v-for="n in 5" :key="n"
          class="h-16 rounded-xl bg-zinc-800 animate-pulse"
        />
      </div>

      <!-- Empty state -->
      <div
        v-else-if="!store.loading && store.songs.length === 0"
        class="flex flex-col items-center justify-center py-24 text-zinc-500"
      >
        <svg xmlns="http://www.w3.org/2000/svg" class="h-12 w-12 mb-4 opacity-40"
          fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.2">
          <path stroke-linecap="round" stroke-linejoin="round"
            d="M9 19V6l12-3v13M9 19a2 2 0 1 1-4 0 2 2 0 0 1 4 0Zm12 0a2 2 0 1 1-4 0 2 2 0 0 1 4 0Z" />
        </svg>
        <p class="text-sm">No songs registered yet.</p>
        <RouterLink to="/" class="mt-3 text-sm text-blue-400 hover:underline">
          Upload your first song →
        </RouterLink>
      </div>

      <!-- Song list -->
      <ul v-else class="space-y-3">
        <li
          v-for="song in store.songs"
          :key="song.id"
          class="flex items-center gap-4 rounded-xl bg-zinc-900 border border-zinc-800 px-5 py-4 transition-colors hover:border-zinc-700"
        >
          <!-- Icon -->
          <div class="shrink-0 text-zinc-600">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-8 w-8"
              fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.2">
              <path stroke-linecap="round" stroke-linejoin="round"
                d="M9 19V6l12-3v13M9 19a2 2 0 1 1-4 0 2 2 0 0 1 4 0Zm12 0a2 2 0 1 1-4 0 2 2 0 0 1 4 0Z" />
            </svg>
          </div>

          <!-- Info -->
          <div class="flex-1 min-w-0">
            <p class="text-sm font-semibold text-white truncate">
              {{ song.title ?? 'Unknown Title' }}
            </p>
            <p class="text-xs text-zinc-400 truncate mt-0.5">
              {{ song.artist ?? 'Unknown Artist' }}
              <span v-if="song.album"> · {{ song.album }}</span>
            </p>
          </div>

          <!-- Meta -->
          <div class="hidden sm:flex flex-col items-end gap-0.5 shrink-0">
            <span class="text-xs text-zinc-400">{{ formatDuration(song.duration_ms) }}</span>
            <span class="text-xs text-zinc-600">{{ formatDate(song.created_at) }}</span>
          </div>

          <!-- Delete -->
          <button
            class="shrink-0 ml-2 text-zinc-600 hover:text-red-400 transition-colors disabled:opacity-30"
            :disabled="store.deletingId === song.id"
            :title="`Delete ${song.title ?? 'this song'}`"
            @click="store.removeSong(song.id)"
          >
            <svg v-if="store.deletingId === song.id"
              class="h-5 w-5 animate-spin" xmlns="http://www.w3.org/2000/svg"
              fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10"
                stroke="currentColor" stroke-width="4" />
              <path class="opacity-75" fill="currentColor"
                d="M4 12a8 8 0 0 1 8-8V0C5.373 0 0 5.373 0 12h4Z" />
            </svg>
            <svg v-else xmlns="http://www.w3.org/2000/svg" class="h-5 w-5"
              fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
              <path stroke-linecap="round" stroke-linejoin="round"
                d="m14.74 9-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0" />
            </svg>
          </button>
        </li>
      </ul>

      <!-- Pagination -->
      <div
        v-if="totalPages > 1"
        class="mt-8 flex items-center justify-center gap-2"
      >
        <button
          class="px-3 py-1.5 rounded-md text-sm text-zinc-400 hover:text-white hover:bg-zinc-800 disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
          :disabled="store.page <= 1 || store.loading"
          @click="store.fetchSongs(store.page - 1)"
        >
          ← Prev
        </button>

        <span class="text-sm text-zinc-500">
          {{ store.page }} / {{ totalPages }}
        </span>

        <button
          class="px-3 py-1.5 rounded-md text-sm text-zinc-400 hover:text-white hover:bg-zinc-800 disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
          :disabled="store.page >= totalPages || store.loading"
          @click="store.fetchSongs(store.page + 1)"
        >
          Next →
        </button>
      </div>

    </div>
  </main>
</template>