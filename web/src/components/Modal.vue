<script setup>
defineProps({
  show: { type: Boolean, default: false },
  title: { type: String, default: '' },
})
defineEmits(['close'])
</script>

<template>
  <Transition name="modal">
    <div v-if="show" class="modal-backdrop" @click.self="$emit('close')">
      <div class="modal-panel" role="dialog" aria-modal="true">
        <header class="modal-head">
          <h3>{{ title }}</h3>
          <button class="icon-btn" aria-label="关闭" @click="$emit('close')">×</button>
        </header>
        <div class="modal-body">
          <slot />
        </div>
      </div>
    </div>
  </Transition>
</template>
