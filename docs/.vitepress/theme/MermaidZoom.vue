<script setup lang="ts">
import { onMounted, onUnmounted, nextTick, watch, ref } from 'vue'
import { useRoute } from 'vitepress'

const route = useRoute()
const modalSvgHtml = ref<string | null>(null)
const modalScale = ref(1)
const modalTranslate = ref({ x: 0, y: 0 })
const isDraggingModal = ref(false)
const dragStart = ref({ x: 0, y: 0 })

// Instala la barra de herramientas y la interactividad en cada diagrama Mermaid
function setupMermaidContainers() {
  const containers = document.querySelectorAll<HTMLElement>('.mermaid')
  containers.forEach((container) => {
    // Solo inicializar si el SVG ya fue renderizado en el DOM
    const svg = container.querySelector<SVGElement>('svg')
    if (!svg) return

    if (container.dataset.zoomEnabled) return
    container.dataset.zoomEnabled = 'true'

    // Asegurar wrapper
    container.classList.add('mermaid-wrapper')

    // Escala y posición local
    let scale = 1
    let translateX = 0
    let translateY = 0
    let isDragging = false
    let startX = 0
    let startY = 0

    svg.style.transformOrigin = 'center center'
    svg.style.transition = 'transform 0.15s ease-out'

    const updateTransform = () => {
      const currentSvg = container.querySelector<SVGElement>('svg')
      if (currentSvg) {
        currentSvg.style.transform = `translate(${translateX}px, ${translateY}px) scale(${scale})`
      }
    }

    // Crear barra de herramientas flotante si no existe
    if (!container.querySelector('.mermaid-toolbar')) {
      const toolbar = document.createElement('div')
      toolbar.className = 'mermaid-toolbar'
      toolbar.innerHTML = `
        <button class="zoom-btn zoom-in" title="Acercar (Zoom In)">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"></circle><line x1="21" y1="21" x2="16.65" y2="16.65"></line><line x1="11" y1="8" x2="11" y2="14"></line><line x1="8" y1="11" x2="14" y2="11"></line></svg>
        </button>
        <button class="zoom-btn zoom-out" title="Alejar (Zoom Out)">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"></circle><line x1="21" y1="21" x2="16.65" y2="16.65"></line><line x1="8" y1="11" x2="14" y2="11"></line></svg>
        </button>
        <button class="zoom-btn zoom-reset" title="Restablecer">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"></path><path d="M3 3v5h5"></path></svg>
        </button>
        <button class="zoom-btn zoom-fullscreen" title="Pantalla completa">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M8 3H5a2 2 0 0 0-2 2v3m18 0V5a2 2 0 0 0-2-2h-3m0 18h3a2 2 0 0 0 2-2v-3M3 16v3a2 2 0 0 0 2 2h3"></path></svg>
        </button>
      `

      toolbar.querySelector('.zoom-in')?.addEventListener('click', (e) => {
        e.stopPropagation()
        scale = Math.min(scale + 0.25, 3.5)
        updateTransform()
      })

      toolbar.querySelector('.zoom-out')?.addEventListener('click', (e) => {
        e.stopPropagation()
        scale = Math.max(scale - 0.25, 0.5)
        updateTransform()
      })

      toolbar.querySelector('.zoom-reset')?.addEventListener('click', (e) => {
        e.stopPropagation()
        scale = 1
        translateX = 0
        translateY = 0
        updateTransform()
      })

      toolbar.querySelector('.zoom-fullscreen')?.addEventListener('click', (e) => {
        e.stopPropagation()
        const currentSvg = container.querySelector<SVGElement>('svg')
        if (currentSvg) {
          modalSvgHtml.value = currentSvg.outerHTML
          modalScale.value = 1.2
          modalTranslate.value = { x: 0, y: 0 }
        }
      })

      container.appendChild(toolbar)
    }

    // Arrastre con el mouse para paneo
    const onMouseDown = (e: MouseEvent) => {
      if ((e.target as HTMLElement).closest('.mermaid-toolbar')) return
      isDragging = true
      startX = e.clientX - translateX
      startY = e.clientY - translateY
      container.style.cursor = 'grabbing'

      const onMouseMove = (moveEvt: MouseEvent) => {
        if (!isDragging) return
        translateX = moveEvt.clientX - startX
        translateY = moveEvt.clientY - startY
        updateTransform()
      }

      const onMouseUp = () => {
        isDragging = false
        container.style.cursor = 'grab'
        window.removeEventListener('mousemove', onMouseMove)
        window.removeEventListener('mouseup', onMouseUp)
      }

      window.addEventListener('mousemove', onMouseMove)
      window.addEventListener('mouseup', onMouseUp)
    }

    container.addEventListener('mousedown', onMouseDown)

    // Zoom con rueda de ratón (Ctrl + Scroll o Shift + Scroll)
    container.addEventListener('wheel', (e) => {
      if (e.ctrlKey || e.metaKey) {
        e.preventDefault()
        const delta = e.deltaY < 0 ? 0.15 : -0.15
        scale = Math.min(Math.max(scale + delta, 0.4), 4)
        updateTransform()
      }
    }, { passive: false })

    // Doble clic para alternar zoom rápido
    container.addEventListener('dblclick', (e) => {
      if ((e.target as HTMLElement).closest('.mermaid-toolbar')) return
      scale = scale === 1 ? 1.6 : 1
      if (scale === 1) {
        translateX = 0
        translateY = 0
      }
      updateTransform()
    })
  })
}

// Modal handlers
function closeModal() {
  modalSvgHtml.value = null
  modalScale.value = 1
  modalTranslate.value = { x: 0, y: 0 }
}

function zoomModal(delta: number) {
  modalScale.value = Math.min(Math.max(modalScale.value + delta, 0.4), 5)
}

function onModalMouseDown(e: MouseEvent) {
  if ((e.target as HTMLElement).closest('.modal-controls')) return
  isDraggingModal.value = true
  dragStart.value = {
    x: e.clientX - modalTranslate.value.x,
    y: e.clientY - modalTranslate.value.y
  }
}

function onModalMouseMove(e: MouseEvent) {
  if (!isDraggingModal.value) return
  modalTranslate.value = {
    x: e.clientX - dragStart.value.x,
    y: e.clientY - dragStart.value.y
  }
}

function onModalMouseUp() {
  isDraggingModal.value = false
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && modalSvgHtml.value) {
    closeModal()
  }
}

onMounted(() => {
  window.addEventListener('keydown', onKeydown)
  nextTick(() => {
    setupMermaidContainers()
    // Reintentar periódicamente por si Mermaid renderiza de forma asíncrona
    setTimeout(setupMermaidContainers, 300)
    setTimeout(setupMermaidContainers, 800)
    setTimeout(setupMermaidContainers, 1500)
  })
})

onUnmounted(() => {
  window.removeEventListener('keydown', onKeydown)
})

watch(
  () => route.path,
  () => {
    nextTick(() => {
      setTimeout(setupMermaidContainers, 200)
      setTimeout(setupMermaidContainers, 600)
    })
  }
)
</script>

<template>
  <!-- Modal de pantalla completa para diagramas -->
  <teleport to="body">
    <div
      v-if="modalSvgHtml"
      class="mermaid-modal-backdrop"
      @click.self="closeModal"
      @mousedown="onModalMouseDown"
      @mousemove="onModalMouseMove"
      @mouseup="onModalMouseUp"
    >
      <div class="modal-controls">
        <button class="modal-btn" title="Acercar (+)" @click="zoomModal(0.25)">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"></circle><line x1="21" y1="21" x2="16.65" y2="16.65"></line><line x1="11" y1="8" x2="11" y2="14"></line><line x1="8" y1="11" x2="14" y2="11"></line></svg>
        </button>
        <button class="modal-btn" title="Alejar (-)" @click="zoomModal(-0.25)">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"></circle><line x1="21" y1="21" x2="16.65" y2="16.65"></line><line x1="8" y1="11" x2="14" y2="11"></line></svg>
        </button>
        <button class="modal-btn" title="Restablecer (0)" @click="() => { modalScale = 1; modalTranslate = { x: 0, y: 0 } }">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"></path><path d="M3 3v5h5"></path></svg>
        </button>
        <span class="zoom-level">{{ Math.round(modalScale * 100) }}%</span>
        <button class="modal-btn close-btn" title="Cerrar (Esc)" @click="closeModal">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"></line><line x1="6" y1="6" x2="18" y2="18"></line></svg>
        </button>
      </div>

      <div
        class="modal-canvas"
        :style="{
          transform: `translate(${modalTranslate.x}px, ${modalTranslate.y}px) scale(${modalScale})`
        }"
        v-html="modalSvgHtml"
      ></div>

      <div class="modal-hint">
        Arrastra para mover | Ctrl + Rueda para zoom | Esc para salir
      </div>
    </div>
  </teleport>
</template>

<style scoped>
.mermaid-modal-backdrop {
  position: fixed;
  top: 0;
  left: 0;
  width: 100vw;
  height: 100vh;
  background: rgba(15, 23, 42, 0.88);
  backdrop-filter: blur(8px);
  z-index: 9999;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: grab;
  user-select: none;
}
.mermaid-modal-backdrop:active {
  cursor: grabbing;
}
.modal-controls {
  position: absolute;
  top: 20px;
  right: 24px;
  display: flex;
  align-items: center;
  gap: 8px;
  background: var(--vp-c-bg-elv, #1e293b);
  border: 1px solid var(--vp-c-divider, #334155);
  border-radius: 9999px;
  padding: 4px 12px;
  box-shadow: 0 10px 25px rgba(0, 0, 0, 0.4);
  z-index: 10000;
}
.modal-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border-radius: 50%;
  color: var(--vp-c-text-1, #f8fafc);
  background: transparent;
  border: none;
  cursor: pointer;
  transition: all 0.2s ease;
}
.modal-btn:hover {
  background: var(--vp-c-brand-soft, rgba(56, 189, 248, 0.15));
  color: var(--vp-c-brand-1, #38bdf8);
}
.close-btn:hover {
  background: rgba(239, 68, 68, 0.15);
  color: #ef4444;
}
.zoom-level {
  font-size: 13px;
  font-weight: 600;
  color: var(--vp-c-text-2, #94a3b8);
  padding: 0 6px;
  min-width: 44px;
  text-align: center;
}
.modal-canvas {
  max-width: 90vw;
  max-height: 85vh;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: transform 0.1s ease-out;
  transform-origin: center center;
}
.modal-canvas :deep(svg) {
  width: auto !important;
  height: auto !important;
  max-width: 90vw !important;
  max-height: 85vh !important;
  filter: drop-shadow(0 15px 30px rgba(0, 0, 0, 0.3));
}
.modal-hint {
  position: absolute;
  bottom: 20px;
  font-size: 12px;
  color: var(--vp-c-text-3, #64748b);
  background: rgba(0, 0, 0, 0.4);
  padding: 4px 12px;
  border-radius: 20px;
  pointer-events: none;
}
</style>
