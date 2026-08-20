<script setup>
import { computed } from 'vue'

const props = defineProps({
  type: { type: String, default: 'ssq' },
  red: { type: String, default: '' },
  blue: { type: String, default: '' },
  hitRed: { type: Array, default: () => [] },
  hitBlue: { type: Array, default: () => [] },
  stacked: { type: Boolean, default: false },
})

const reds = computed(() =>
  (props.red || '').split(',').map((s) => s.trim()).filter(Boolean)
)
const blues = computed(() =>
  (props.blue || '').split(',').map((s) => s.trim()).filter(Boolean)
)
const isDlt = computed(() => props.type === 'dlt')
</script>

<template>
  <div class="balls" :class="{ 'balls-stacked': stacked }">
    <template v-if="isDlt">
      <span class="ball-label">前区</span>
    </template>
    <span
      v-for="b in reds"
      :key="'r' + b"
      class="ball ball-red"
      :class="{ 'is-hit': hitRed.includes(b) }"
    >
      {{ b }}
      <span v-if="hitRed.includes(b)" class="hit-badge" aria-label="命中">✓</span>
    </span>
    <template v-if="blues.length">
      <span v-if="!stacked" class="ball-sep">+</span>
      <span v-if="stacked" class="ball-break"></span>
      <template v-if="isDlt">
        <span class="ball-label">后区</span>
      </template>
      <span
        v-for="b in blues"
        :key="'b' + b"
        class="ball ball-blue"
        :class="{ 'is-hit': hitBlue.includes(b) }"
      >
        {{ b }}
        <span v-if="hitBlue.includes(b)" class="hit-badge" aria-label="命中">✓</span>
      </span>
    </template>
  </div>
</template>
