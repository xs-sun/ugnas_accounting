<script setup lang="ts">
import { ref, watch } from 'vue';
import { CAT_AVATARS, DEFAULT_MASCOT, MASCOT_FALLBACK } from '@/constants/avatars';
import { useAppStore } from '@/stores/useAppStore';
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
  success: [auth: AuthResponse];
}>();

const store = useAppStore();

const currentView = ref<'login' | 'register' | 'forgot'>(props.initialView);
const customRegAvatar = ref('');

watch(
  () => [props.open, props.initialView] as const,
  ([isOpen, initView]) => {
    if (isOpen) {
      currentView.value = initView;
    }
  }
);

function onLogoError(e: Event) {
  const img = e.target as HTMLImageElement | null;
  if (!img || img.dataset.fb) return;
  img.dataset.fb = '1';
  img.src = MASCOT_FALLBACK;
}

function handleOpenRegAvatar(curAvatar: string) {
  store.openAvatarModal(
    'register',
    (picked: string) => {
      customRegAvatar.value = picked;
    },
    curAvatar
  );
}

async function handleAuthSuccess(auth: AuthResponse) {
  if (auth.token && store.user?.user_id !== auth.user?.user_id) {
    await store.setAuthToken(auth.token, auth.user, auth.refresh_token);
  }
  emit('success', auth);
  emit('close');
}
</script>

<template>
  <Teleport to="body">
    <div
      v-if="open"
      class="fixed inset-0 z-[65] bg-[#F4F5F8] overflow-y-auto thin-scroll"
    >
      <div class="min-h-screen flex items-center justify-center p-4 sm:p-6">
        <div
          class="w-full max-w-[920px] bg-white rounded-[28px] border border-[#EAECEF] shadow-2xl overflow-hidden grid grid-cols-1 md:grid-cols-12 relative"
        >
          <!-- 关闭并返回工作台 -->
          <button
            v-if="!required"
            type="button"
            class="absolute top-4 right-4 z-10 h-8 px-3 rounded-full bg-[#F4F5F8] hover:bg-[#EAECEF] text-[11px] font-semibold text-[#475569] flex items-center gap-1 transition"
            @click="emit('close')"
          >
            <span>返回工作台</span>
            <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2">
              <path d="M18 6L6 18M6 6l12 12" />
            </svg>
          </button>

          <!-- 左侧暖琥珀视觉品牌栏 -->
          <div
            class="hidden md:flex md:col-span-5 bg-gradient-to-br from-[#FFFBF4] via-[#FFF6E8] to-[#F4F5F8] p-8 flex-col justify-between border-r border-[#F1EDE4]"
          >
            <div class="space-y-4">
              <div class="flex items-center gap-2.5">
                <img
                  :src="DEFAULT_MASCOT"
                  alt="七猫记账"
                  class="w-10 h-10 rounded-2xl object-contain bg-white p-1 border border-[#F2E4CC] shadow-sm"
                  @error="onLogoError"
                />
                <div>
                  <p class="text-base font-bold text-[#18181B]">七猫记账</p>
                  <p class="text-[11px] text-[#64748B]">私有部署 · 轻盈智能记账</p>
                </div>
              </div>

              <div class="pt-4">
                <h2 class="text-2xl font-bold text-[#18181B] leading-snug">
                  把每一笔收支<br />都记清楚
                </h2>
                <p class="text-xs text-[#64748B] leading-relaxed mt-2.5">
                  PC 与移动端实时同步，支持自然语言 AI 记账、小票 OCR、多账本家庭协作与定时自动入账。
                </p>
              </div>

              <div class="space-y-2.5 pt-2 text-xs text-[#475569]">
                <div class="flex items-center gap-2">
                  <span
                    class="w-5 h-5 rounded-full bg-white text-[#DE8A1E] flex items-center justify-center text-[10px] font-bold shadow-xs"
                  >
                    ✦
                  </span>
                  <span>AI 聊着记、语音转写与小票识别</span>
                </div>
                <div class="flex items-center gap-2">
                  <span
                    class="w-5 h-5 rounded-full bg-white text-[#3E8E63] flex items-center justify-center text-[10px] font-bold shadow-xs"
                  >
                    ✓
                  </span>
                  <span>一/二级精致图标分类与月度预算水位</span>
                </div>
                <div class="flex items-center gap-2">
                  <span
                    class="w-5 h-5 rounded-full bg-white text-[#2F78A8] flex items-center justify-center text-[10px] font-bold shadow-xs"
                  >
                    🔒
                  </span>
                  <span>数据完全自主，支持 NAS 定时备份</span>
                </div>
              </div>
            </div>

            <!-- 6 款 3D 小猫头像展示 -->
            <div class="pt-6">
              <div class="flex -space-x-2">
                <img
                  v-for="cat in CAT_AVATARS"
                  :key="cat.id"
                  :src="cat.src"
                  :alt="cat.name"
                  :title="cat.name"
                  class="w-10 h-10 rounded-full object-cover ring-2 ring-white shadow-sm bg-white"
                />
              </div>
              <p class="text-[11px] text-[#75685D] mt-2">
                注册即随机赠送 3D 毛绒小猫头像，也可随时上传裁剪
              </p>
            </div>
          </div>

          <!-- 右侧交互表单区 -->
          <div class="md:col-span-7 p-6 sm:p-9 flex flex-col justify-center">
            <AuthFormPanel
              :initial-view="currentView"
              layout="pc"
              :custom-avatar="customRegAvatar"
              @open-avatar="handleOpenRegAvatar"
              @view-change="currentView = $event"
              @success="handleAuthSuccess"
            />
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>
