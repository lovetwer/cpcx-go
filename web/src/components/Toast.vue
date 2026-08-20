<script setup>
import { toasts, dismiss } from '../store/toast'
</script>

<template>
  <div class="toast-wrap" aria-live="polite">
    <TransitionGroup name="toast">
      <div
        v-for="t in toasts"
        :key="t.id"
        class="toast"
        :class="'toast-' + t.type"
        @click="dismiss(t.id)"
      >
        <span class="toast-dot"></span>
        <span class="toast-msg">{{ t.message }}</span>
      </div>
    </TransitionGroup>
  </div>
</template>
