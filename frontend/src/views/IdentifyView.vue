<script setup lang="ts">
import { onUnmounted, computed } from 'vue'
import { useIdentifyStore } from '@/stores/identify'
import { useRecorder } from '@/composables/useRecorder'

const RECORD_SECONDS = 10

const store = useIdentifyStore()
const recorder = useRecorder()

// ── Actions ───────────────────────────────────────────────────────────────────

async function startIdentify(): Promise<void> {
  store.reset()
  await recorder.start()

  if (recorder.state.value === 'error') {
    store.setError(recorder.error.value ?? 'Microphone error.')
    return
  }

  store.setRecording()

  store.startCountdown(RECORD_SECONDS, async () => {
    const blob = await recorder.stop()
    await store.submitBlob(blob)
  })
}

function retry(): void {
  recorder.reset()
  store.reset()
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function formatDuration(ms: number | null): string {
  if (ms === null) return '—'
  const total = Math.floor(ms / 1000)
  const m = Math.floor(total / 60)
  const s = total % 60
  return `${m}:${s.toString().padStart(2, '0')}`
}

const confidenceLabel = computed(() => {
  const c = store.match?.confidence ?? 0
  if (c >= 0.8) return { text: 'High', cls: 'bg-emerald-500/20 text-emerald-300 border-emerald-500/30' }
  if (c >= 0.5) return { text: 'Medium', cls: 'bg-yellow-500/20 text-yellow-300 border-yellow-500/30' }
  return { text: 'Low', cls: 'bg-red-500/20 text-red-300 border-red-500/30' }
})

const confidencePct = computed(() =>
  Math.round((store.match?.confidence ?? 0) * 100)
)

onUnmounted(() => {
  store.clearCountdown()
  recorder.reset()
})
</script>

<template>
  <main class="min-h-screen bg-zinc-950 flex flex-col items-center justify-center px-4 py-12">
    <div class="w-full max-w-md flex flex-col items-center gap-8">

      <!-- Title -->
      <div class="text-center">
        <h1 class="text-2xl font-bold text-white tracking-tight">Identify a Song</h1>
        <p class="mt-1 text-sm text-zinc-400">Hold your device near the music and tap the button</p>
      </div>

      <!-- ── IDLE ────────────────────────────────────────────────────────────── -->
      <template v-if="store.phase === 'idle'">
        <button
          class="relative h-36 w-36 rounded-full bg-blue-600 hover:bg-blue-500 active:scale-95
                 transition-all duration-150 shadow-lg shadow-blue-900/40 flex items-center justify-center"
          @click="startIdentify"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-16 w-16 text-white"
            fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.3">
            <path stroke-linecap="round" stroke-linejoin="round"
              d="M12 18.75a6 6 0 0 0 6-6v-1.5m-6 7.5a6 6 0 0 1-6-6v-1.5m6 7.5v3.75m-3.75 0h7.5
                 M12 3a4.125 4.125 0 0 0-4.125 4.125v6.75a4.125 4.125 0 0 0 8.25 0v-6.75
                 A4.125 4.125 0 0 0 12 3Z" />
          </svg>
        </button>
        <p class="text-sm text-zinc-500">Tap to start listening</p>
      </template>

      <!-- ── RECORDING ───────────────────────────────────────────────────────── -->
      <template v-else-if="store.phase === 'recording'">
        <!-- Pulsing ring -->
        <div class="relative flex items-center justify-center h-36 w-36">
          <span class="absolute inline-flex h-full w-full rounded-full bg-blue-500 opacity-20 animate-ping" />
          <div class="relative h-36 w-36 rounded-full bg-zinc-800 border-2 border-blue-500
                      flex flex-col items-center justify-center gap-1">
            <span class="text-4xl font-bold text-white tabular-nums">
              {{ store.secondsLeft }}
            </span>
            <span class="text-xs text-zinc-400 uppercase tracking-widest">sec left</span>
          </div>
        </div>

        <!-- Animated waveform bars -->
        <div class="flex items-end gap-1 h-10">
          <span
            v-for="i in 12" :key="i"
            class="w-1.5 rounded-full bg-blue-400"
            :style="{
              height: `${20 + Math.random() * 80}%`,
              animationDelay: `${i * 80}ms`,
              animation: 'wave 0.8s ease-in-out infinite alternate',
            }"
          />
        </div>

        <p class="text-sm text-zinc-400">Listening… keep the music playing</p>
      </template>

      <!-- ── PROCESSING ──────────────────────────────────────────────────────── -->
      <template v-else-if="store.phase === 'processing'">
        <div class="h-36 w-36 rounded-full bg-zinc-800 border border-zinc-700
                    flex items-center justify-center">
          <svg class="h-12 w-12 animate-spin text-blue-400"
            xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
            <circle class="opacity-25" cx="12" cy="12" r="10"
              stroke="currentColor" stroke-width="4" />
            <path class="opacity-75" fill="currentColor"
              d="M4 12a8 8 0 0 1 8-8V0C5.373 0 0 5.373 0 12h4Z" />
          </svg>
        </div>
        <p class="text-sm text-zinc-400">Analyzing audio fingerprint…</p>
      </template>

      <!-- ── MATCH ───────────────────────────────────────────────────────────── -->
      <template v-else-if="store.phase === 'match' && store.match">
        <div class="w-full rounded-2xl bg-zinc-900 border border-zinc-700 p-6 flex flex-col gap-4">

          <!-- Song info -->
          <div class="flex items-center gap-4">
            <div class="shrink-0 h-14 w-14 rounded-xl bg-zinc-800 flex items-center justify-center text-zinc-500">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-7 w-7"
                fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.3">
                <path stroke-linecap="round" stroke-linejoin="round"
                  d="M9 19V6l12-3v13M9 19a2 2 0 1 1-4 0 2 2 0 0 1 4 0Zm12 0a2 2 0 1 1-4 0 2 2 0 0 1 4 0Z" />
              </svg>
            </div>
            <div class="flex-1 min-w-0">
              <p class="text-base font-semibold text-white truncate">
                {{ store.match.title ?? 'Unknown Title' }}
              </p>
              <p class="text-sm text-zinc-400 truncate">
                {{ store.match.artist ?? 'Unknown Artist' }}
                <span v-if="store.match.album"> · {{ store.match.album }}</span>
              </p>
              <p class="text-xs text-zinc-600 mt-0.5">
                {{ formatDuration(store.match.duration_ms) }}
              </p>
            </div>
          </div>

          <!-- Confidence -->
          <div class="flex flex-col gap-1.5">
            <div class="flex items-center justify-between text-xs">
              <span class="text-zinc-500">Confidence</span>
              <span
                :class="['px-2 py-0.5 rounded-full border text-xs font-medium', confidenceLabel.cls]"
              >
                {{ confidenceLabel.text }} · {{ confidencePct }}%
              </span>
            </div>
            <div class="h-1.5 w-full rounded-full bg-zinc-800 overflow-hidden">
              <div
                class="h-full rounded-full transition-all duration-700"
                :class="confidencePct >= 80 ? 'bg-emerald-400' : confidencePct >= 50 ? 'bg-yellow-400' : 'bg-red-400'"
                :style="{ width: `${confidencePct}%` }"
              />
            </div>
          </div>

          <!-- Spotify link -->
          <a
            v-if="store.match.spotify_id"
            :href="`https://open.spotify.com/track/${store.match.spotify_id}`"
            target="_blank"
            rel="noopener noreferrer"
            class="flex items-center justify-center gap-2 rounded-xl bg-[#1DB954] hover:bg-[#1ed760]
                   text-black text-sm font-semibold py-2.5 transition-colors"
          >
            <svg class="h-4 w-4" viewBox="0 0 24 24" fill="currentColor">
              <path d="M12 0C5.4 0 0 5.4 0 12s5.4 12 12 12 12-5.4 12-12S18.66 0 12 0zm5.521
                       17.34c-.24.359-.66.48-1.021.24-2.82-1.74-6.36-2.101-10.561-1.141-.418.122-.779-.179
                       -.899-.539-.12-.421.18-.78.54-.9 4.56-1.021 8.52-.6 11.64 1.32.42.18.479.659.301
                       1.02zm1.44-3.3c-.301.42-.841.6-1.262.3-3.239-1.98-8.159-2.58-11.939-1.38-.479.12
                       -1.02-.12-1.14-.6-.12-.48.12-1.021.6-1.141C9.6 9.9 15 10.561 18.72 12.84c.361.181
                       .54.78.241 1.2zm.12-3.36C15.24 8.4 8.82 8.16 5.16 9.301c-.6.179-1.2-.181-1.38
                       -.721-.18-.601.18-1.2.72-1.381 4.26-1.26 11.28-1.02 15.721 1.621.539.3.719
                       1.02.419 1.56-.299.421-1.02.599-1.559.3z"/>
            </svg>
            Open in Spotify
          </a>
        </div>

        <button
          class="text-sm text-zinc-500 hover:text-white transition-colors"
          @click="retry"
        >
          ↺ Try again
        </button>
      </template>

      <!-- ── NO MATCH ────────────────────────────────────────────────────────── -->
      <template v-else-if="store.phase === 'no-match'">
        <div class="h-36 w-36 rounded-full bg-zinc-800 border border-zinc-700
                    flex items-center justify-center text-zinc-500">
          <svg xmlns="http://www.w3.org/2000/svg" class="h-14 w-14"
            fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.2">
            <path stroke-linecap="round" stroke-linejoin="round"
              d="M9.879 7.519c1.171-1.025 3.071-1.025 4.242 0 1.172 1.025 1.172 2.687
                 0 3.712-.203.179-.43.326-.67.442-.745.361-1.45.999-1.45 1.827v.75
                 M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 5.25h.008v.008H12v-.008Z" />
          </svg>
        </div>
        <div class="text-center">
          <p class="text-white font-semibold">No match found</p>
          <p class="mt-1 text-sm text-zinc-400">
            Make sure the song is in your library and the audio is clear.
          </p>
        </div>
        <button
          class="px-5 py-2 rounded-xl bg-zinc-800 hover:bg-zinc-700 text-sm text-white transition-colors"
          @click="retry"
        >
          Try again
        </button>
      </template>

      <!-- ── ERROR ───────────────────────────────────────────────────────────── -->
      <template v-else-if="store.phase === 'error'">
        <div class="h-36 w-36 rounded-full bg-zinc-800 border border-red-800
                    flex items-center justify-center text-red-400">
          <svg xmlns="http://www.w3.org/2000/svg" class="h-14 w-14"
            fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.2">
            <path stroke-linecap="round" stroke-linejoin="round"
              d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71
                 c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898
                 0L2.697 16.126ZM12 15.75h.007v.008H12v-.008Z" />
          </svg>
        </div>
        <div class="w-full rounded-xl bg-red-900/20 border border-red-800 px-4 py-3 text-sm text-red-300 text-center">
          {{ store.error }}
        </div>
        <button
          class="px-5 py-2 rounded-xl bg-zinc-800 hover:bg-zinc-700 text-sm text-white transition-colors"
          @click="retry"
        >
          Try again
        </button>
      </template>

    </div>
  </main>
</template>

<style scoped>
@keyframes wave {
  from { transform: scaleY(0.4); }
  to   { transform: scaleY(1); }
}
</style>