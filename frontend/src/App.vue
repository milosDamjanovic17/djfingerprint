<script setup lang="ts">
import { RouterView } from 'vue-router'
import NavBar from '@/components/NavBar.vue'
</script>

<template>
  <div class="min-h-screen bg-zinc-950 flex flex-col">
    <NavBar />
    <RouterView />
  </div>
</template>