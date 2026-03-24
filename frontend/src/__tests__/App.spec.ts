import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import { createPinia } from 'pinia'
import App from '../App.vue'
import UploadView from '../views/UploadView.vue'
import LibraryView from '../views/LibraryView.vue'
import IdentifyView from '../views/IdentifyView.vue'

const router = createRouter({
  history: createMemoryHistory(),
  routes: [
    { path: '/', component: UploadView },
    { path: '/library', component: LibraryView },
    { path: '/identify', component: IdentifyView },
  ],
})

describe('App', () => {
  it('mounts without errors', async () => {
    const wrapper = mount(App, {
      global: { plugins: [router, createPinia()] },
    })
    await router.isReady()
    expect(wrapper.exists()).toBe(true)
  })

  it('renders the navbar', async () => {
    const wrapper = mount(App, {
      global: { plugins: [router, createPinia()] },
    })
    await router.isReady()
    expect(wrapper.text()).toContain('DJ Fingerprint')
  })
})