import { createRouter, createWebHistory } from 'vue-router'
import UploadView from '@/views/UploadView.vue'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      name: 'upload',
      component: UploadView,
    },
    {
      path: '/library',
      name: 'library',
      component: () => import('@/views/LibraryView.vue'),
    },
    {
      path: '/identify',
      name: 'identify',
      component: () => import('@/views/IdentifyView.vue'),
    },
  ],
})

export default router