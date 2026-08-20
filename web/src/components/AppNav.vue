<script setup>
import { useRoute } from 'vue-router'
const route = useRoute()
</script>

<template>
  <nav class="tabbar">
    <router-link to="/add" class="tabbar-item" :class="{ active: route.path === '/add' }">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
        <path d="M12 5v14M5 12h14" />
      </svg>
      <span>录入</span>
    </router-link>

    <router-link to="/draw" class="tabbar-item" :class="{ active: route.path === '/draw' }">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
        <rect x="3.5" y="5" width="17" height="15.5" rx="2.5" />
        <path d="M3.5 9.5h17M8 3.5v3.2M16 3.5v3.2" />
      </svg>
      <span>开奖</span>
    </router-link>

    <router-link to="/buy" class="tabbar-item" :class="{ active: route.path === '/buy' }">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
        <path d="M3 8.5A2 2 0 0 1 5 6.5h14a2 2 0 0 1 2 2v1.2a2 2 0 0 0 0 3.6V17a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-3.7a2 2 0 0 0 0-3.6V8.5Z" />
        <path d="M14 6.5v11" stroke-dasharray="2 2.2" />
      </svg>
      <span>购彩</span>
    </router-link>

    <router-link to="/profile" class="tabbar-item" :class="{ active: route.path === '/profile' }">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
        <circle cx="12" cy="8" r="3.6" />
        <path d="M5 20a7 7 0 0 1 14 0" />
      </svg>
      <span>我的</span>
    </router-link>
  </nav>
</template>
