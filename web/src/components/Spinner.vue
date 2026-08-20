<template>
  <span class="spinner" :class="{ 'spinner-light': light }" aria-label="加载中"></span>
</template>

<script setup>
defineProps({ light: { type: Boolean, default: false } })
</script>
