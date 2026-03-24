<script setup lang="ts">
import UploadZone from '../components/UploadZone.vue'
</script>

<template>
  <main class="min-h-screen bg-zinc-950 flex flex-col items-center justify-center px-4">
    <div class="mb-10 text-center">
      <h1 class="text-3xl font-bold text-white tracking-tight">DJ Fingerprint</h1>
      <p class="mt-2 text-zinc-400 text-sm">Register a song by uploading its audio file</p>
    </div>
    <UploadZone />
  </main>
</template>