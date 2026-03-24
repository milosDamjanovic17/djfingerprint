<script setup lang="ts">
import { ref, computed } from 'vue'
import { useUploadStore } from '../stores/upload'

const store = useUploadStore()

const isDragging = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)

const acceptedTypes = ['audio/mpeg', 'audio/wav', 'audio/flac', 'audio/mp4', 'audio/ogg']
const acceptedExtensions = '.mp3,.wav,.flac,.m4a,.ogg'

const statusLabel = computed(() => {
  switch (store.status) {
    case 'uploading': return 'Uploading and fingerprinting…'
    case 'success':   return 'Song registered successfully'
    case 'error':     return store.error ?? 'Upload failed'
    default:          return 'Drop an audio file here or click to browse'
  }
})

const borderColor = computed(() => {
  switch (store.status) {
    case 'uploading': return 'border-blue-400'
    case 'success':   return 'border-green-400'
    case 'error':     return 'border-red-400'
    default:          return isDragging.value ? 'border-blue-400' : 'border-zinc-600'
  }
})

function onDragOver(e: DragEvent) {
  e.preventDefault()
  isDragging.value = true
}

function onDragLeave() {
  isDragging.value = false
}

function onDrop(e: DragEvent) {
  e.preventDefault()
  isDragging.value = false
  const file = e.dataTransfer?.files[0]
  if (file) handleFile(file)
}

function onFileChange(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (file) handleFile(file)
}

function handleFile(file: File) {
  if (!acceptedTypes.includes(file.type)) {
    alert(`Unsupported file type: ${file.type}. Please upload an MP3, WAV, FLAC, M4A, or OGG file.`)
    return
  }
  store.upload(file)
}

function openFilePicker() {
  if (store.status === 'uploading') return
  fileInput.value?.click()
}

function reset() {
  store.reset()
  if (fileInput.value) fileInput.value.value = ''
}
</script>

<template>
  <div class="w-full max-w-xl mx-auto">

    <!-- Drop zone -->
    <div
      :class="[
        'relative flex flex-col items-center justify-center',
        'rounded-2xl border-2 border-dashed transition-colors duration-200',
        'bg-zinc-900 cursor-pointer select-none p-12',
        borderColor,
        store.status === 'uploading' ? 'pointer-events-none opacity-70' : '',
      ]"
      @dragover="onDragOver"
      @dragleave="onDragLeave"
      @drop="onDrop"
      @click="openFilePicker"
    >
      <!-- Icon -->
      <div class="mb-4 text-zinc-400">
        <svg
          v-if="store.status === 'idle' || store.status === 'uploading'"
          xmlns="http://www.w3.org/2000/svg"
          class="h-14 w-14"
          fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.2"
        >
          <path stroke-linecap="round" stroke-linejoin="round"
            d="M9 19V6l12-3v13M9 19a2 2 0 1 1-4 0 2 2 0 0 1 4 0Zm12 0a2 2 0 1 1-4 0 2 2 0 0 1 4 0Z" />
        </svg>

        <svg
          v-else-if="store.status === 'success'"
          xmlns="http://www.w3.org/2000/svg"
          class="h-14 w-14 text-green-400"
          fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"
        >
          <path stroke-linecap="round" stroke-linejoin="round" d="m4.5 12.75 6 6 9-13.5" />
        </svg>

        <svg
          v-else-if="store.status === 'error'"
          xmlns="http://www.w3.org/2000/svg"
          class="h-14 w-14 text-red-400"
          fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"
        >
          <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m9-.75a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 3.75h.008v.008H12v-.008Z" />
        </svg>
      </div>

      <!-- Spinner -->
      <div v-if="store.status === 'uploading'" class="mb-4">
        <svg class="animate-spin h-6 w-6 text-blue-400" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
          <path class="opacity-75" fill="currentColor"
            d="M4 12a8 8 0 0 1 8-8V0C5.373 0 0 5.373 0 12h4Z" />
        </svg>
      </div>

      <!-- Status text -->
      <p
        :class="[
          'text-sm font-medium text-center',
          store.status === 'success' ? 'text-green-400' :
          store.status === 'error'   ? 'text-red-400'   :
                                       'text-zinc-300'
        ]"
      >
        {{ statusLabel }}
      </p>

      <!-- Supported formats hint -->
      <p v-if="store.status === 'idle'" class="mt-2 text-xs text-zinc-500">
        MP3 · WAV · FLAC · M4A · OGG
      </p>

      <!-- Hidden file input -->
      <input
        ref="fileInput"
        type="file"
        :accept="acceptedExtensions"
        class="hidden"
        @change="onFileChange"
      />
    </div>

    <!-- Success result card -->
    <div
      v-if="store.status === 'success' && store.result"
      class="mt-6 rounded-xl bg-zinc-800 border border-zinc-700 p-5 space-y-2"
    >
      <h3 class="text-sm font-semibold text-zinc-200 uppercase tracking-wide">Registered</h3>
      <p class="text-lg font-bold text-white">
        {{ store.result.title ?? 'Unknown Title' }}
      </p>
      <p class="text-sm text-zinc-400">
        {{ store.result.artist ?? 'Unknown Artist' }}
        <span v-if="store.result.album"> · {{ store.result.album }}</span>
      </p>
      <p class="text-xs text-zinc-500 font-mono">ID: {{ store.result.id }}</p>

      <button
        class="mt-3 w-full rounded-lg bg-zinc-700 hover:bg-zinc-600 text-zinc-200 text-sm py-2 transition-colors"
        @click.stop="reset"
      >
        Upload another
      </button>
    </div>

    <!-- Error retry -->
    <div v-if="store.status === 'error'" class="mt-4 text-center">
      <button
        class="text-sm text-zinc-400 hover:text-white underline transition-colors"
        @click.stop="reset"
      >
        Try again
      </button>
    </div>

  </div>
</template>