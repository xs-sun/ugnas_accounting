<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import { useAppStore } from '@/stores/useAppStore';
import { DEFAULT_MASCOT, MASCOT_FALLBACK } from '@/constants/avatars';
import type { AuthResponse } from '@/types';
import AuthFormPanel from '@/components/shared/AuthFormPanel.vue';

const props = withDefaults(
  defineProps<{
    open: boolean;
    initialView?: 'login' | 'register' | 'forgot';
    required?: boolean;
  }>(),
  {
    initialView: 'login',
    required: false,
  }
);

const emit = defineEmits<{
  close: [];
  'update:open': [val: boolean];
  success: [auth: AuthResponse];
}>();

const store = useAppStore();

const activeView = ref<'login' | 'register' | 'forgot'>(props.initialView);
const customRegAvatar = ref('');

const headerTitle = computed(() => {
  if (activeView.value === 'register') return '注册新账号';
  if (activeView.value === 'forgot') return '找回密码';
  return '账号登录';
});

watch(
  () => props.open,
  (opened) => {
    if (opened) {
      activeView.value = props.initialView || 'login';
      customRegAvatar.value = '';
    }
  },
  { immediate: true }
);

watch(
  () => props.initialView,
  (val) => {
    if (val) activeView.value = val;
  }
);

function closeScreen() {
  emit('update:open', false);
  emit('close');
}

function handleBack() {
  if (activeView.value !== 'login') {
    activeView.value = 'login';
  } else {
    closeScreen();
  }
}

function handleViewChange(next: 'login' | 'register' | 'forgot') {
  activeView.value = next;
}

function handleOpenAvatar(currentAvatar: string) {
  store.openAvatarModal(
    'register',
    (avatar) => {
      customRegAvatar.value = avatar;
    },
    currentAvatar
  );
}

async function handleAuthSuccess(auth: AuthResponse) {
  // AuthFormPanel 内部通常会调用 store.setAuthToken，此处按契约做幂等兜底同步
  if (!store.hasAuthToken() || store.user?.user_id !== auth.user.user_id) {
    await store.setAuthToken(auth.token, auth.user);
  }
  emit('success', auth);
  closeScreen();
}

function onMascotError(e: Event) {
  const img = e.target as HTMLImageElement | null;
  if (!img || img.dataset.fb) return;
  img.dataset.fb = '1';
  img.src = MASCOT_FALLBACK;
}
</script>

<template>
  <div
    v-if="open"
    class="fixed inset-0 z-50 bg-[#F5F6F8] flex flex-col overflow-hidden pt-safe pb-safe"
  >
    <!-- 顶部导航条 -->
    <header class="shrink-0 pt-3 px-4 pb-2 flex items-center justify-between gap-2">
      <button
        v-if="!required || activeView !== 'login'"
        type="button"
        aria-label="返回"
        class="w-8 h-8 rounded-full bg-white border border-[#ECEEF2] flex items-center justify-center text-[#475569] hover:text-[#18181B] transition shrink-0"
        @click="handleBack"
      >
        <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2">
          <path d="M15 18l-6-6 6-6" />
        </svg>
      </button>
      <span v-else class="w-8 h-8 shrink-0" />

      <div class="flex items-center gap-1.5 min-w-0">
        <img
          :src="DEFAULT_MASCOT"
          alt="七猫记账"
          class="w-5 h-5 rounded-full object-contain bg-white border border-[#ECEEF2]"
          @error="onMascotError"
        />
        <span class="text-xs font-semibold text-[#64748B] truncate">{{ headerTitle }}</span>
      </div>

      <button
        v-if="!required"
        type="button"
        title="返回应用"
        aria-label="关闭登录页"
        class="w-8 h-8 rounded-full bg-white border border-[#ECEEF2] flex items-center justify-center text-[#64748B] hover:text-[#18181B] transition shrink-0"
        @click="closeScreen"
      >
        <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2">
          <path d="M18 6L6 18M6 6l12 12" />
        </svg>
      </button>
      <span v-else class="w-8 h-8 shrink-0" />
    </header>

    <!-- 认证表单主体 -->
    <div class="flex-1 min-h-0 overflow-y-auto phone-scroll px-5 pt-2 pb-8">
      <AuthFormPanel
        :initial-view="activeView"
        layout="mobile"
        :custom-avatar="customRegAvatar"
        @open-avatar="handleOpenAvatar"
        @view-change="handleViewChange"
        @success="handleAuthSuccess"
      />
    </div>
  </div>
</template>
