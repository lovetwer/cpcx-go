<script setup>
import { ref, watch, onUnmounted } from 'vue'

const props = defineProps({
  show: { type: Boolean, default: false },
})

const step = ref(0)
let stepTimer = null

function startSteps() {
  step.value = 0
  stopSteps()
  stepTimer = setInterval(() => {
    step.value = (step.value + 1) % 6
  }, 1200)
}

function stopSteps() {
  if (stepTimer) {
    clearInterval(stepTimer)
    stepTimer = null
  }
}

watch(
  () => props.show,
  (v) => {
    if (v) startSteps()
    else stopSteps()
  }
)

onUnmounted(() => stopSteps())
</script>

<template>
  <Teleport to="body">
    <Transition name="ai-overlay">
      <div v-if="show" class="ai-overlay">
        <div class="ai-overlay-inner">
          <!-- 直接使用 Loader.svg 作为图片，SMIL 动画会自动播放 -->
          <img src="/Loader.svg" alt="loading" class="ai-loader-img" />
          <div class="ai-overlay-text">AI 正在分析近30期开奖数据…</div>
          <div class="ai-overlay-sub">冷热号 · 遗漏值 · 区间分区 · 重邻孤</div>
          <!-- 分析步骤滚动 -->
          <div class="ai-steps">
            <Transition name="step" mode="out-in">
              <span key="0" v-if="step === 0">正在获取开奖记录…</span>
              <span key="1" v-else-if="step === 1">冷热号分析中…</span>
              <span key="2" v-else-if="step === 2">遗漏值分析中…</span>
              <span key="3" v-else-if="step === 3">区间与分区分析中…</span>
              <span key="4" v-else-if="step === 4">和值与跨度分析中…</span>
              <span key="5" v-else>重邻孤分析中…</span>
            </Transition>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.ai-overlay {
  position: fixed;
  inset: 0;
  z-index: 200;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(15, 15, 20, 0.75);
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
}
.ai-overlay-inner {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 20px;
}

/* SVG 动画图片 */
.ai-loader-img {
  width: 280px;
  height: 280px;
  object-fit: contain;
}

.ai-overlay-text {
  color: #fff;
  font-size: 16px;
  font-weight: 700;
  letter-spacing: 0.02em;
  animation: textPulse 1.5s ease-in-out infinite;
}
.ai-overlay-sub {
  color: rgba(255, 255, 255, 0.55);
  font-size: 13px;
  font-weight: 500;
}
.ai-steps {
  height: 20px;
  overflow: hidden;
}
.ai-steps span {
  color: rgba(255, 255, 255, 0.7);
  font-size: 13px;
  font-weight: 500;
}
@keyframes textPulse {
  0%, 100% { opacity: 0.6; }
  50% { opacity: 1; }
}

/* 步骤切换过渡 */
.step-enter-active,
.step-leave-active {
  transition: transform 250ms cubic-bezier(0.23, 1, 0.32, 1), opacity 200ms ease;
}
.step-enter-from {
  opacity: 0;
  transform: translateY(10px);
}
.step-leave-to {
  opacity: 0;
  transform: translateY(-10px);
}

/* 遮罩过渡 */
.ai-overlay-enter-active,
.ai-overlay-leave-active {
  transition: opacity 300ms ease;
}
.ai-overlay-enter-from,
.ai-overlay-leave-to {
  opacity: 0;
}
.ai-overlay-enter-active .ai-overlay-inner,
.ai-overlay-leave-active .ai-overlay-inner {
  transition: transform 400ms cubic-bezier(0.23, 1, 0.32, 1), opacity 300ms ease;
}
.ai-overlay-enter-from .ai-overlay-inner,
.ai-overlay-leave-to .ai-overlay-inner {
  transform: scale(0.9);
  opacity: 0;
}
</style>
