<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue';
import { useAppStore } from '@/stores/useAppStore';
import PcShell from '@/pc/PcShell.vue';
import MobileShell from '@/mobile/MobileShell.vue';
import AppToast from '@/components/shared/AppToast.vue';
import AvatarPickerModal from '@/components/shared/AvatarPickerModal.vue';

const store = useAppStore();

const isMobileViewport = ref(
  typeof window !== 'undefined'
    ? window.matchMedia('(max-width: 767px)').matches
    : false
);

let mediaQueryList: MediaQueryList | null = null;

function handleViewportChange(e: MediaQueryListEvent | MediaQueryList) {
  isMobileViewport.value = e.matches;
}

onMounted(() => {
  if (typeof window !== 'undefined') {
    mediaQueryList = window.matchMedia('(max-width: 767px)');
    isMobileViewport.value = mediaQueryList.matches;
    if (typeof mediaQueryList.addEventListener === 'function') {
      mediaQueryList.addEventListener('change', handleViewportChange);
    } else if (typeof mediaQueryList.addListener === 'function') {
      mediaQueryList.addListener(handleViewportChange);
    }
  }
  if (!store.initialized && !store.loading) {
    void store.initApp();
  }
});

onBeforeUnmount(() => {
  if (!mediaQueryList) return;
  if (typeof mediaQueryList.removeEventListener === 'function') {
    mediaQueryList.removeEventListener('change', handleViewportChange);
  } else if (typeof mediaQueryList.removeListener === 'function') {
    mediaQueryList.removeListener(handleViewportChange);
  }
  mediaQueryList = null;
});
</script>

<template>
  <MobileShell v-if="isMobileViewport" />
  <PcShell v-else />

  <!-- 全局轻提示与 3D 小猫头像选择器 -->
  <AppToast />
  <AvatarPickerModal />
</template>
