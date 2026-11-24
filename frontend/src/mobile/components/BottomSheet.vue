<script lang="ts">
// 模块级计数器：多个抽屉叠放或快速切换时安全锁定外层滚动
let openSheetLockCount = 0;
let previousBodyOverflow = '';

function acquireBodyLock(): void {
  if (typeof document === 'undefined') return;
  if (openSheetLockCount === 0) {
    previousBodyOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
  }
  openSheetLockCount += 1;
}

function releaseBodyLock(): void {
  if (typeof document === 'undefined') return;
  if (openSheetLockCount > 0) {
    openSheetLockCount -= 1;
    if (openSheetLockCount === 0) {
      document.body.style.overflow = previousBodyOverflow;
    }
  }
}
</script>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue';

export type SheetDetent = 'compact' | 'default' | 'fullscreen';

const props = withDefaults(
  defineProps<{
    open: boolean;
    multiDetent?: boolean;
    initialDetent?: SheetDetent;
    maxHeightClass?: string;
    bgClass?: string;
    zIndexClass?: string;
    closeOnBackdrop?: boolean;
  }>(),
  {
    multiDetent: false,
    initialDetent: 'default',
    maxHeightClass: 'max-h-[90dvh]',
    bgClass: 'bg-[#F8F9FB]',
    zIndexClass: 'z-40',
    closeOnBackdrop: true,
  }
);

const emit = defineEmits<{
  close: [];
  'update:open': [val: boolean];
  'detent-change': [detent: SheetDetent];
}>();

const panelRef = ref<HTMLElement | null>(null);
const backdropRef = ref<HTMLElement | null>(null);

const detent = ref<SheetDetent>(props.initialDetent);
const isDragging = ref(false);
const isDetentAnimating = ref(false);
const dragTransform = ref('');
const dragHeight = ref('');
const dragMaxHeight = ref('');
const backdropOpacity = ref<string | undefined>(undefined);

// 软键盘（visualViewport）适配状态：iOS WKWebView 弹键盘时 fixed inset-0 仍按全屏布局，
// 需要按 visualViewport 实时约束弹窗可视区，并将聚焦的输入框滚入视野。
const viewportHeight = ref(0);
const viewportOffsetTop = ref(0);
const keyboardOpen = ref(false);

let hasLockedBody = false;
let dragStartY = 0;
let dragStartHeight = 0;
let dragLastDy = 0;
let dragStartDetent: SheetDetent = 'default';
let activePointerId: number | null = null;
let resetTimer: ReturnType<typeof setTimeout> | null = null;
let detentAnimTimer: ReturnType<typeof setTimeout> | null = null;
let focusScrollTimer1: ReturnType<typeof setTimeout> | null = null;
let focusScrollTimer2: ReturnType<typeof setTimeout> | null = null;

function syncBodyLock(opened: boolean) {
  if (opened && !hasLockedBody) {
    acquireBodyLock();
    hasLockedBody = true;
  } else if (!opened && hasLockedBody) {
    releaseBodyLock();
    hasLockedBody = false;
  }
}

function clearInlineStyles() {
  dragTransform.value = '';
  dragHeight.value = '';
  dragMaxHeight.value = '';
  backdropOpacity.value = undefined;
}

function triggerDetentAnimation() {
  if (detentAnimTimer) clearTimeout(detentAnimTimer);
  isDetentAnimating.value = true;
  detentAnimTimer = setTimeout(() => {
    isDetentAnimating.value = false;
    detentAnimTimer = null;
  }, 280);
}

function setDetent(next: SheetDetent) {
  triggerDetentAnimation();
  detent.value = next;
  clearInlineStyles();
  emit('detent-change', next);
}

function isEditableElement(el: EventTarget | null): el is HTMLElement {
  if (!(el instanceof HTMLElement)) return false;
  const tag = el.tagName;
  if (tag === 'TEXTAREA' || tag === 'SELECT') return true;
  if (tag === 'INPUT') {
    const type = (el as HTMLInputElement).type;
    return !['checkbox', 'radio', 'file', 'button', 'submit', 'range', 'hidden'].includes(type);
  }
  return el.isContentEditable;
}

function updateViewportMetrics() {
  if (typeof window === 'undefined') return;
  const vv = window.visualViewport;
  const layoutH = window.innerHeight || document.documentElement.clientHeight || 0;
  if (!vv) {
    viewportHeight.value = 0;
    viewportOffsetTop.value = 0;
    keyboardOpen.value = false;
    return;
  }
  const activeEditable = isEditableElement(document.activeElement) && Boolean(panelRef.value?.contains(document.activeElement));
  const shrunk = layoutH > 0 && vv.height < layoutH - 100;
  keyboardOpen.value = shrunk || (activeEditable && vv.height < layoutH - 40);
  if (keyboardOpen.value) {
    viewportHeight.value = Math.round(vv.height);
    viewportOffsetTop.value = Math.round(vv.offsetTop);
  } else {
    viewportHeight.value = 0;
    viewportOffsetTop.value = 0;
  }
}

function scrollFocusedInputIntoView(target: HTMLElement) {
  const scrollInto = () => {
    if (!props.open || !panelRef.value?.contains(target)) return;
    updateViewportMetrics();
    try {
      target.scrollIntoView({ block: 'center', inline: 'nearest', behavior: 'smooth' });
    } catch {
      target.scrollIntoView(true);
    }
  };
  if (focusScrollTimer1) clearTimeout(focusScrollTimer1);
  if (focusScrollTimer2) clearTimeout(focusScrollTimer2);
  focusScrollTimer1 = setTimeout(scrollInto, 120);
  focusScrollTimer2 = setTimeout(scrollInto, 340);
}

function onPanelFocusIn(e: FocusEvent) {
  if (!props.open) return;
  if (isEditableElement(e.target)) {
    updateViewportMetrics();
    scrollFocusedInputIntoView(e.target);
  }
}

function onPanelFocusOut() {
  setTimeout(() => {
    if (!panelRef.value?.contains(document.activeElement)) {
      updateViewportMetrics();
    }
  }, 80);
}

onMounted(() => {
  if (typeof window !== 'undefined' && window.visualViewport) {
    window.visualViewport.addEventListener('resize', updateViewportMetrics);
    window.visualViewport.addEventListener('scroll', updateViewportMetrics);
  }
});

watch(
  () => props.open,
  (val) => {
    if (resetTimer) {
      clearTimeout(resetTimer);
      resetTimer = null;
    }
    syncBodyLock(val);
    if (val) {
      detent.value = props.initialDetent;
      isDragging.value = false;
      isDetentAnimating.value = false;
      clearInlineStyles();
      updateViewportMetrics();
    } else {
      keyboardOpen.value = false;
      viewportHeight.value = 0;
      viewportOffsetTop.value = 0;
      // 等待关闭位移动画完全结束后再清理状态，避免关闭尾声触发重排抖动
      resetTimer = setTimeout(() => {
        isDragging.value = false;
        isDetentAnimating.value = false;
        detent.value = props.initialDetent;
        clearInlineStyles();
        resetTimer = null;
      }, 300);
    }
  },
  { immediate: true }
);

onBeforeUnmount(() => {
  if (resetTimer) clearTimeout(resetTimer);
  if (detentAnimTimer) clearTimeout(detentAnimTimer);
  if (focusScrollTimer1) clearTimeout(focusScrollTimer1);
  if (focusScrollTimer2) clearTimeout(focusScrollTimer2);
  if (typeof window !== 'undefined' && window.visualViewport) {
    window.visualViewport.removeEventListener('resize', updateViewportMetrics);
    window.visualViewport.removeEventListener('scroll', updateViewportMetrics);
  }
  syncBodyLock(false);
});

function closeSheet() {
  emit('update:open', false);
  emit('close');
}

function onBackdropClick() {
  if (!props.closeOnBackdrop) return;
  closeSheet();
}

function getViewportHeight(): number {
  if (typeof window === 'undefined') return 790;
  return window.innerHeight || document.documentElement.clientHeight || 790;
}

function onHandlePointerDown(e: PointerEvent) {
  if (e.button !== undefined && e.button !== 0) return;
  const panel = panelRef.value;
  if (!panel) return;

  isDragging.value = true;
  activePointerId = e.pointerId;
  dragStartY = e.clientY;
  dragLastDy = 0;
  dragStartDetent = detent.value;
  dragStartHeight = panel.getBoundingClientRect().height;

  const target = e.currentTarget as HTMLElement | null;
  if (target?.setPointerCapture) {
    try {
      target.setPointerCapture(e.pointerId);
    } catch {
      // 忽略不支持指针捕获的设备异常
    }
  }
  e.preventDefault();
}

function onHandlePointerMove(e: PointerEvent) {
  if (!isDragging.value) return;
  const dy = e.clientY - dragStartY;
  dragLastDy = dy;

  const screenH = getViewportHeight();
  const fullH = screenH - 44;
  const compactH = Math.round(screenH * 0.44);

  if (!props.multiDetent) {
    if (dy >= 0) {
      dragTransform.value = `translateY(${dy}px)`;
      backdropOpacity.value = String(Math.max(0.1, 1 - dy / 320));
    } else {
      const resist = Math.max(-16, dy * 0.14);
      dragTransform.value = `translateY(${resist}px)`;
      backdropOpacity.value = undefined;
    }
    return;
  }

  // 多档高度控制：关闭 ↔ 缩小 ↔ 默认 ↔ 全屏
  if (dy < 0) {
    dragTransform.value = '';
    backdropOpacity.value = undefined;
    const rawH = dragStartHeight + Math.abs(dy);
    const targetH = rawH <= fullH ? rawH : fullH + (rawH - fullH) * 0.14;
    const finalH = `${Math.min(fullH + 14, targetH)}px`;
    dragHeight.value = finalH;
    dragMaxHeight.value = finalH;
  } else if (dragStartDetent === 'compact') {
    dragHeight.value = `${dragStartHeight}px`;
    dragMaxHeight.value = `${dragStartHeight}px`;
    dragTransform.value = `translateY(${dy}px)`;
    backdropOpacity.value = String(Math.max(0.1, 1 - dy / 240));
  } else {
    const shrunkH = dragStartHeight - dy;
    if (shrunkH >= compactH) {
      dragTransform.value = '';
      dragHeight.value = `${shrunkH}px`;
      dragMaxHeight.value = `${shrunkH}px`;
      backdropOpacity.value = undefined;
    } else {
      const overflowDy = (compactH - shrunkH) * 0.45;
      dragHeight.value = `${compactH}px`;
      dragMaxHeight.value = `${compactH}px`;
      dragTransform.value = `translateY(${overflowDy}px)`;
      backdropOpacity.value = String(Math.max(0.35, 1 - overflowDy / 260));
    }
  }
}

function onHandlePointerEnd(e: PointerEvent) {
  if (!isDragging.value) return;
  isDragging.value = false;

  const target = e.currentTarget as HTMLElement | null;
  if (target?.releasePointerCapture && activePointerId !== null) {
    try {
      target.releasePointerCapture(activePointerId);
    } catch {
      // 忽略异常
    }
  }
  activePointerId = null;
  backdropOpacity.value = undefined;

  if (!props.multiDetent) {
    clearInlineStyles();
    if (dragLastDy > 75) {
      closeSheet();
    }
    return;
  }

  const screenH = getViewportHeight();
  const compactH = Math.round(screenH * 0.44);
  const overflowPastCompact =
    dragStartHeight - dragLastDy < compactH
      ? compactH - (dragStartHeight - dragLastDy)
      : 0;

  if (dragStartDetent === 'fullscreen') {
    if (overflowPastCompact > 190) {
      setDetent('default');
      closeSheet();
    } else if (dragLastDy > 190) {
      setDetent('compact');
    } else if (dragLastDy > 55) {
      setDetent('default');
    } else {
      setDetent('fullscreen');
    }
  } else if (dragStartDetent === 'default') {
    if (dragLastDy < -50) {
      setDetent('fullscreen');
    } else if (overflowPastCompact > 165) {
      setDetent('default');
      closeSheet();
    } else if (dragLastDy > 48) {
      setDetent('compact');
    } else {
      setDetent('default');
    }
  } else {
    if (dragLastDy < -190) {
      setDetent('fullscreen');
    } else if (dragLastDy < -45) {
      setDetent('default');
    } else if (dragLastDy > 55) {
      setDetent('default');
      closeSheet();
    } else {
      setDetent('compact');
    }
  }
}

function onHandleDblClick() {
  if (!props.multiDetent) return;
  if (detent.value === 'default') {
    setDetent('fullscreen');
  } else if (detent.value === 'fullscreen') {
    setDetent('compact');
  } else {
    setDetent('default');
  }
}

const panelClass = computed(() => [
  props.bgClass,
  props.maxHeightClass,
  {
    'is-dragging': isDragging.value,
    'is-detent-animating': isDetentAnimating.value,
    'sheet-compact': detent.value === 'compact',
    'sheet-fullscreen': detent.value === 'fullscreen',
    'sheet-keyboard-open': keyboardOpen.value,
  },
]);

const rootStyle = computed(() => {
  if (!props.open || !keyboardOpen.value || viewportHeight.value <= 0) {
    return undefined;
  }
  return {
    top: `${viewportOffsetTop.value}px`,
    height: `${viewportHeight.value}px`,
    bottom: 'auto',
  };
});

const panelStyle = computed(() => {
  const style: Record<string, string> = {};
  if (dragTransform.value) style.transform = dragTransform.value;
  if (keyboardOpen.value && viewportHeight.value > 0) {
    style.maxHeight = `${Math.max(220, viewportHeight.value - 12)}px`;
  } else {
    if (dragHeight.value) style.height = dragHeight.value;
    if (dragMaxHeight.value) style.maxHeight = dragMaxHeight.value;
  }
  return style;
});

const backdropStyle = computed(() =>
  backdropOpacity.value !== undefined ? { opacity: backdropOpacity.value } : undefined
);
</script>

<template>
  <div
    class="sheet-root fixed inset-0 flex flex-col justify-end"
    :class="[zIndexClass, { 'sheet-hidden': !open }]"
    :style="rootStyle"
    :aria-hidden="!open"
  >
    <!-- 遮罩层：保留轻度毛玻璃模糊并通过 GPU 合成层做 opacity 过渡 -->
    <div
      ref="backdropRef"
      class="sheet-backdrop fixed inset-0 bg-[#18211C]/40 backdrop-blur-[2px]"
      :style="backdropStyle"
      @click="onBackdropClick"
      @wheel.prevent.stop
      @touchmove.prevent.stop
    />

    <!-- 抽屉面板 -->
    <div
      ref="panelRef"
      class="sheet-panel relative w-full rounded-t-[30px] flex flex-col shadow-2xl border-t border-white/70 overflow-hidden pb-safe"
      :class="panelClass"
      :style="panelStyle"
      @wheel.stop
      @focusin="onPanelFocusIn"
      @focusout="onPanelFocusOut"
    >
      <!-- 顶部可拖拽把手 -->
      <div
        class="sheet-drag-handle flex flex-col items-center pt-2.5 pb-2 shrink-0"
        :title="multiDetent ? '向下拖动关闭，向上拖动全屏' : '向下拖动关闭'"
        @pointerdown="onHandlePointerDown"
        @pointermove="onHandlePointerMove"
        @pointerup="onHandlePointerEnd"
        @pointercancel="onHandlePointerEnd"
        @dblclick="onHandleDblClick"
      >
        <div class="sheet-grabber-bar w-10 h-1.5 rounded-full bg-[#D4D9E1]" />
      </div>

      <!-- 标题栏插槽 -->
      <slot name="header" />

      <!-- 内容插槽 -->
      <div class="flex-1 min-h-0 flex flex-col overflow-hidden">
        <slot />
      </div>

      <!-- 底部固定操作插槽 -->
      <slot name="footer" />
    </div>
  </div>
</template>
