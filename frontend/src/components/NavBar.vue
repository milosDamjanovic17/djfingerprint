<script setup lang="ts">
import { RouterLink, useRoute } from 'vue-router'

const route = useRoute()

const links = [
  { to: '/',        label: 'Upload'  },
  { to: '/identify', label: 'Identify' },
  { to: '/library',  label: 'Library'  },
]
</script>

<template>
  <nav class="w-full bg-zinc-900 border-b border-zinc-800">
    <div class="max-w-4xl mx-auto px-4 h-14 flex items-center justify-between">

      <!-- Logo -->
      <RouterLink to="/" class="flex items-center gap-2 text-white font-bold tracking-tight">
        <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-blue-400"
          fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
          <path stroke-linecap="round" stroke-linejoin="round"
            d="M9 19V6l12-3v13M9 19a2 2 0 1 1-4 0 2 2 0 0 1 4 0Zm12 0a2 2 0 1 1-4 0 2 2 0 0 1 4 0Z" />
        </svg>
        DJ Fingerprint
      </RouterLink>

      <!-- Links -->
      <div class="flex items-center gap-1">
        <RouterLink
          v-for="link in links"
          :key="link.to"
          :to="link.to"
          :class="[
            'px-3 py-1.5 rounded-md text-sm font-medium transition-colors',
            route.path === link.to
              ? 'bg-zinc-700 text-white'
              : 'text-zinc-400 hover:text-white hover:bg-zinc-800',
          ]"
        >
          {{ link.label }}
        </RouterLink>
      </div>

    </div>
  </nav>
</template>