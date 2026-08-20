<script setup>
import { useRoute } from 'vue-router'
import AppNav from './components/AppNav.vue'
import Toast from './components/Toast.vue'

const route = useRoute()
</script>

<template>
  <div class="app-shell">
    <AppNav v-if="route.meta.auth" />
    <main class="app-main" :class="{ 'with-nav': route.meta.auth }">
      <router-view v-slot="{ Component }">
        <Transition name="page" mode="out-in">
          <component :is="Component" />
        </Transition>
      </router-view>
    </main>
    <Toast />
  </div>
</template>
