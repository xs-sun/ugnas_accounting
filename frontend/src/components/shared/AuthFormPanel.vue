<script setup lang="ts">
import { ref, computed, watch, onBeforeUnmount } from 'vue';
import { api } from '@/api/client';
import {
  CAT_AVATARS,
  DEFAULT_MASCOT,
  MASCOT_FALLBACK,
  getRandomCatAvatar,
  resolveAvatarUrl,
} from '@/constants/avatars';
import { useAppStore } from '@/stores/useAppStore';
import type { AuthResponse } from '@/types';
import VerificationCodeField, { type CodeDeliveryHint } from './VerificationCodeField.vue';

const props = withDefaults(
  defineProps<{
    initialView?: 'login' | 'register' | 'forgot';
    layout?: 'mobile' | 'pc';
    customAvatar?: string;
  }>(),
  {
    initialView: 'login',
    layout: 'mobile',
    customAvatar: '',
  }
);

const emit = defineEmits<{
  success: [auth: AuthResponse];
  'open-avatar': [currentAvatar: string];
  'view-change': [view: 'login' | 'register' | 'forgot'];
}>();

const store = useAppStore();

const view = ref<'login' | 'register' | 'forgot'>(props.initialView);
const loginMode = ref<'password' | 'code'>('password');
const submitting = ref(false);
const gatewayLoggingIn = ref(false);
const sendingCode = ref(false);
const errorMsg = ref('');

const gatewayBind = ref<{
  ticket: string;
  gatewayUid: string;
  gatewayName: string;
} | null>(null);

// 表单状态
const loginAccount = ref(store.user?.username || store.user?.email || '');
const loginPassword = ref('');
const loginCode = ref('');

const regUsername = ref('');
const regName = ref('');
const regEmail = ref('');
const regCode = ref('');
const regPassword = ref('');
const regInviteCode = ref('');
const regAgreed = ref(true);
const regAvatar = ref(CAT_AVATARS[0].publicPath);

const forgotEmail = ref(store.user?.email || '');
const forgotCode = ref('');
const forgotNewPwd = ref('');
const forgotConfirmPwd = ref('');

// 验证码 60s 倒计时与发送反馈
const cooldown = ref(0);
let cooldownTimer: ReturnType<typeof setInterval> | null = null;
const sentHintMap = ref<Partial<Record<'login' | 'register' | 'forgot', CodeDeliveryHint>>>({});

const resolvedRegAvatar = computed(() => resolveAvatarUrl(regAvatar.value));

watch(
  () => props.initialView,
  (val) => {
    view.value = val;
    errorMsg.value = '';
  }
);

watch(
  () => props.customAvatar,
  (val) => {
    if (val) {
      regAvatar.value = val;
    }
  }
);

function switchView(next: 'login' | 'register' | 'forgot') {
  view.value = next;
  errorMsg.value = '';
  if (next === 'login') {
    gatewayBind.value = null;
  }
  if (next === 'register' && !props.customAvatar && !regAvatar.value) {
    randomizeAvatar();
  }
  emit('view-change', next);
}

function cancelGatewayBind() {
  gatewayBind.value = null;
  errorMsg.value = '';
  switchView('login');
}

function randomizeAvatar() {
  const curSrc = resolveAvatarUrl(regAvatar.value);
  const pickedSrc = getRandomCatAvatar(curSrc);
  const preset = CAT_AVATARS.find((c) => c.src === pickedSrc);
  regAvatar.value = preset ? preset.publicPath : CAT_AVATARS[0].publicPath;
}

function handleOpenAvatar() {
  emit('open-avatar', regAvatar.value);
}

function onLogoError(e: Event) {
  const img = e.target as HTMLImageElement | null;
  if (!img || img.dataset.fb) return;
  img.dataset.fb = '1';
  img.src = MASCOT_FALLBACK;
}

function startCooldown() {
  cooldown.value = 60;
  if (cooldownTimer) clearInterval(cooldownTimer);
  cooldownTimer = setInterval(() => {
    cooldown.value = Math.max(0, cooldown.value - 1);
    if (cooldown.value === 0 && cooldownTimer) {
      clearInterval(cooldownTimer);
      cooldownTimer = null;
    }
  }, 1000);
}

async function handleSendCode(purpose: 'login' | 'register' | 'forgot') {
  if (cooldown.value > 0 || sendingCode.value) return;
  const email =
    purpose === 'login'
      ? loginAccount.value.trim()
      : purpose === 'register'
        ? regEmail.value.trim()
        : forgotEmail.value.trim();

  if (!email) {
    errorMsg.value = '请先输入邮箱地址';
    store.showToast('请先输入邮箱地址', 'error');
    return;
  }
  if (!email.includes('@')) {
    errorMsg.value = '发送验证码请输入绑定的邮箱地址';
    store.showToast('请输入有效的邮箱地址', 'error');
    return;
  }

  sendingCode.value = true;
  errorMsg.value = '';
  try {
    const res = await api.sendCode({ email, purpose });
    startCooldown();
    sentHintMap.value[purpose] = {
      email,
      demoCode: res.demo_code || undefined,
    };
    if (res.demo_code) {
      store.showToast(`验证码 ${res.demo_code} 已发送（演示模式）`, 'info', 4000);
    } else {
      store.showToast('验证码已发送至邮箱', 'success');
    }
  } catch (err: any) {
    const msg = err?.message || '发送验证码失败';
    errorMsg.value = msg;
    store.showToast(msg, 'error');
  } finally {
    sendingCode.value = false;
  }
}

async function submitLogin() {
  if (submitting.value || gatewayLoggingIn.value) return;
  const account = loginAccount.value.trim();
  if (!account) {
    errorMsg.value = loginMode.value === 'code' ? '请输入绑定邮箱地址' : '请输入用户名或邮箱地址';
    return;
  }
  errorMsg.value = '';
  submitting.value = true;
  try {
    const res = await api.login({
      account,
      email: account.includes('@') ? account : undefined,
      username: !account.includes('@') ? account : undefined,
      mode: loginMode.value,
      password: loginMode.value === 'password' ? loginPassword.value : undefined,
      code: loginMode.value === 'code' ? loginCode.value.trim() : undefined,
    });
    await store.setAuthToken(res.token, res.user, res.refresh_token);
    store.showToast('登录成功，欢迎回来！', 'success');
    emit('success', res);
  } catch (err: any) {
    const msg = err?.message || '登录失败，请检查账号或密码';
    errorMsg.value = msg;
    store.showToast(msg, 'error');
  } finally {
    submitting.value = false;
  }
}

async function handleGatewayLogin() {
  if (gatewayLoggingIn.value || submitting.value) return;
  errorMsg.value = '';
  gatewayLoggingIn.value = true;
  try {
    const res = await api.gatewayLogin();
    if (res.status === 'authenticated' && res.token && res.user) {
      await store.setAuthToken(res.token, res.user, res.refresh_token);
      store.showToast('绿联云授权登录成功！', 'success');
      emit('success', {
        token: res.token,
        refresh_token: res.refresh_token,
        user: res.user,
      });
      return;
    }
    if (res.status === 'need_register' && res.bind_ticket) {
      gatewayBind.value = {
        ticket: res.bind_ticket,
        gatewayUid: res.gateway_uid || '',
        gatewayName: res.suggested_name || res.suggested_username || res.gateway_uid || '',
      };
      regUsername.value = res.suggested_username || '';
      regName.value = res.suggested_name || res.suggested_username || '';
      view.value = 'register';
      emit('view-change', 'register');
      store.showToast('绿联云已授权，请完善账号与邮箱信息完成注册', 'info', 3500);
      return;
    }
    throw new Error('未获取到有效的绿联云网关授权信息');
  } catch (err: any) {
    const msg = err?.message || '未检测到绿联云网关登录信息，请使用用户名或邮箱密码登录';
    errorMsg.value = msg;
    store.showToast(msg, 'error');
  } finally {
    gatewayLoggingIn.value = false;
  }
}

async function submitRegister() {
  if (submitting.value || gatewayLoggingIn.value) return;
  const cleanUsername = regUsername.value.trim();
  if (cleanUsername.length < 2 || cleanUsername.length > 32) {
    errorMsg.value = '用户名长度需在 2~32 个字符之间';
    return;
  }
  if (cleanUsername.includes('@')) {
    errorMsg.value = '用户名不能包含 @ 符号';
    return;
  }
  if (!regAgreed.value) {
    errorMsg.value = '请先阅读并同意用户协议与隐私政策';
    return;
  }
  errorMsg.value = '';
  submitting.value = true;
  try {
    const isGatewayBind = Boolean(gatewayBind.value?.ticket);
    const res = await api.register({
      username: cleanUsername,
      name: regName.value.trim(),
      email: regEmail.value.trim(),
      code: regCode.value.trim() || undefined,
      password: regPassword.value,
      invite_code: isGatewayBind ? undefined : regInviteCode.value.trim() || undefined,
      avatar: regAvatar.value,
      bind_ticket: gatewayBind.value?.ticket,
    });
    gatewayBind.value = null;
    await store.setAuthToken(res.token, res.user, res.refresh_token);
    store.showToast(
      isGatewayBind ? '绑定绿联云并注册成功！' : '注册成功，已自动登录！',
      'success'
    );
    emit('success', res);
  } catch (err: any) {
    const msg = err?.message || '注册失败，请检查填写信息';
    errorMsg.value = msg;
    store.showToast(msg, 'error');
  } finally {
    submitting.value = false;
  }
}

async function submitForgot() {
  if (submitting.value) return;
  if (forgotNewPwd.value !== forgotConfirmPwd.value) {
    errorMsg.value = '两次输入的新密码不一致';
    return;
  }
  errorMsg.value = '';
  submitting.value = true;
  try {
    await api.forgotPassword({
      email: forgotEmail.value.trim(),
      code: forgotCode.value.trim(),
      new_password: forgotNewPwd.value,
    });
    store.showToast('密码重置成功，请使用新密码登录', 'success');
    loginAccount.value = forgotEmail.value.trim();
    loginMode.value = 'password';
    loginPassword.value = '';
    forgotCode.value = '';
    forgotNewPwd.value = '';
    forgotConfirmPwd.value = '';
    switchView('login');
  } catch (err: any) {
    const msg = err?.message || '重置密码失败';
    errorMsg.value = msg;
    store.showToast(msg, 'error');
  } finally {
    submitting.value = false;
  }
}

onBeforeUnmount(() => {
  if (cooldownTimer) {
    clearInterval(cooldownTimer);
    cooldownTimer = null;
  }
});
</script>

<template>
  <div :class="layout === 'pc' ? 'max-w-[360px] w-full mx-auto' : 'w-full'">
    <!-- Error Alert -->
    <div
      v-if="errorMsg"
      class="mb-3 px-3.5 py-2 rounded-xl bg-[#FDF0EE] border border-[#F5C4BC] text-[#D6553E] text-xs font-medium"
    >
      {{ errorMsg }}
    </div>

    <!-- ==================== LOGIN VIEW ==================== -->
    <template v-if="view === 'login'">
      <div v-if="layout === 'mobile'" class="pt-3 pb-4 text-center">
        <img
          :src="DEFAULT_MASCOT"
          alt="七猫记账"
          class="w-16 h-16 rounded-3xl mx-auto mb-3 object-contain bg-white p-1.5 border border-[#ECEEF2] shadow-xs"
          @error="onLogoError"
        />
        <h2 class="text-xl font-bold text-[#18181B]">欢迎回到七猫记账</h2>
        <p class="text-xs text-[#64748B] mt-1">
          {{
            store.firstInstall
              ? '首次安装使用，请先注册登录或通过绿联云授权激活'
              : store.authRequired
                ? '请登录账号以查看并管理您的记账数据'
                : '多端同步 · 家庭共享 · AI 轻松记账'
          }}
        </p>
      </div>
      <div v-else class="mb-4">
        <h3 class="text-xl font-bold text-[#18181B]">登录七猫记账</h3>
        <p class="text-xs text-[#64748B] mt-1">
          {{
            store.firstInstall
              ? '首次安装引导 · 请使用账号登录或通过下方绿联云授权激活'
              : '支持用户名 / 邮箱密码登录，或使用绿联云授权登录'
          }}
        </p>
      </div>

      <!-- 首次安装提示卡 -->
      <div
        v-if="store.firstInstall"
        class="mb-3.5 px-3.5 py-2.5 rounded-2xl bg-[#FFF6E8] border border-[#F2DFBE] text-[11px] text-[#8C5510] leading-relaxed"
      >
        <span class="font-bold text-[#DE8A1E]">首次使用提示：</span>
        当前系统尚无已注册账号，完成注册或绿联云授权绑定后将自动成为系统管理员。
      </div>

      <div :class="layout === 'mobile' ? 'app-card p-5 space-y-4' : 'space-y-4'">
        <div class="p-1 rounded-2xl bg-[#F4F5F8] grid grid-cols-2 text-xs font-semibold">
          <button
            type="button"
            class="py-1.5 rounded-xl transition"
            :class="
              loginMode === 'password'
                ? 'bg-white text-[#18181B] shadow-xs'
                : 'text-[#64748B]'
            "
            @click="loginMode = 'password'"
          >
            密码登录
          </button>
          <button
            type="button"
            class="py-1.5 rounded-xl transition"
            :class="
              loginMode === 'code'
                ? 'bg-white text-[#18181B] shadow-xs'
                : 'text-[#64748B]'
            "
            @click="loginMode = 'code'"
          >
            验证码登录
          </button>
        </div>

        <form class="space-y-3 text-xs" @submit.prevent="submitLogin">
          <div>
            <label class="block text-[11px] font-semibold text-[#475569] mb-1">
              {{ loginMode === 'code' ? '绑定邮箱' : '用户名 / 邮箱' }}
            </label>
            <input
              v-model="loginAccount"
              :type="loginMode === 'code' ? 'email' : 'text'"
              required
              :placeholder="loginMode === 'code' ? '请输入绑定的邮箱地址' : '请输入用户名或邮箱地址'"
              class="w-full h-10 text-xs bg-[#F5F6F8] border border-[#E6E9EF] rounded-xl px-3.5 outline-none focus:border-[#DE8A1E] focus:bg-white transition"
            />
          </div>

          <div v-if="loginMode === 'password'">
            <div class="flex items-center justify-between mb-1">
              <label class="text-[11px] font-semibold text-[#475569]">登录密码</label>
              <button
                type="button"
                class="text-[11px] font-semibold text-[#DE8A1E] hover:underline"
                @click="switchView('forgot')"
              >
                忘记密码？
              </button>
            </div>
            <input
              v-model="loginPassword"
              type="password"
              required
              minlength="6"
              placeholder="请输入密码（至少6位）"
              class="w-full h-10 text-xs bg-[#F5F6F8] border border-[#E6E9EF] rounded-xl px-3.5 outline-none focus:border-[#DE8A1E] focus:bg-white transition"
            />
          </div>

          <VerificationCodeField
            v-else
            v-model="loginCode"
            label="邮箱验证码"
            placeholder="请输入 6 位数字验证码"
            :required="true"
            :cooldown="cooldown"
            :sending="sendingCode"
            :sent-hint="sentHintMap.login || null"
            variant="auth"
            @send="handleSendCode('login')"
          />

          <button
            type="submit"
            :disabled="submitting || gatewayLoggingIn"
            class="btn-amber w-full h-11 rounded-2xl text-white text-xs font-semibold active:scale-[0.99] disabled:opacity-60 transition mt-1"
          >
            {{ submitting ? '登录中…' : '立即登录' }}
          </button>
        </form>

        <!-- 快捷登录分割线与绿联云授权入口 -->
        <div class="pt-1 space-y-3">
          <div class="flex items-center gap-2.5 text-[11px] text-[#94A3B8]">
            <span class="flex-1 h-px bg-[#ECEEF2]" />
            <span>快捷登录</span>
            <span class="flex-1 h-px bg-[#ECEEF2]" />
          </div>

          <button
            type="button"
            :disabled="gatewayLoggingIn || submitting"
            class="w-full h-10 rounded-2xl bg-[#F8F9FB] hover:bg-[#FFF6E8] border border-[#E6E9EF] hover:border-[#DE8A1E]/50 text-xs font-semibold text-[#18181B] hover:text-[#DE8A1E] flex items-center justify-center gap-2 disabled:opacity-60 transition"
            @click="handleGatewayLogin"
          >
            <svg class="w-4 h-4 text-[#DE8A1E]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M12 2L4 5.5v6c0 5.2 3.4 9.8 8 11 4.6-1.2 8-5.8 8-11v-6L12 2z" />
              <path d="M9 12l2 2 4-4" />
            </svg>
            <span>{{ gatewayLoggingIn ? '正在获取绿联云授权…' : '绿联云授权登录' }}</span>
          </button>
        </div>
      </div>

      <div class="mt-4 text-center text-xs text-[#64748B]">
        还没有账号？
        <button
          type="button"
          class="font-bold text-[#DE8A1E] hover:underline ml-1"
          @click="switchView('register')"
        >
          免费注册
        </button>
      </div>
    </template>

    <!-- ==================== REGISTER VIEW ==================== -->
    <template v-else-if="view === 'register'">
      <div class="pt-1 pb-3 flex items-center justify-between gap-2">
        <div>
          <h2 class="text-lg font-bold text-[#18181B]">
            {{ gatewayBind ? '完善信息并绑定绿联云' : layout === 'pc' ? '注册新账号' : '创建七猫记账账号' }}
          </h2>
          <p class="text-[11px] text-[#64748B] mt-0.5">
            {{ gatewayBind ? '设置用户名、邮箱与密码，后续各端通用' : '已为你随机分配一只小猫头像' }}
          </p>
        </div>
        <div class="flex items-center gap-2 shrink-0">
          <button
            type="button"
            title="点击更换或上传头像"
            class="relative group"
            @click="handleOpenAvatar"
          >
            <img
              :src="resolvedRegAvatar"
              alt="注册头像"
              class="w-13 h-13 w-12 h-12 rounded-full object-cover bg-white border-2 border-white shadow-sm group-hover:ring-2 group-hover:ring-[#DE8A1E]/50 transition"
            />
            <span
              class="absolute -bottom-0.5 -right-0.5 w-5 h-5 rounded-full bg-[#DE8A1E] text-white border-2 border-white flex items-center justify-center"
            >
              <svg class="w-2.5 h-2.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2">
                <path d="M12 20h9" />
                <path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4 12.5-12.5z" />
              </svg>
            </span>
          </button>
          <button
            type="button"
            class="text-[11px] font-semibold px-2.5 py-1.5 rounded-full bg-white border border-[#ECEEF2] text-[#475569] hover:text-[#DE8A1E] hover:border-[#DE8A1E]/40 transition"
            @click="randomizeAvatar"
          >
            换一只
          </button>
        </div>
      </div>

      <!-- 绿联云授权绑定状态提示卡 -->
      <div
        v-if="gatewayBind"
        class="mb-3 px-3.5 py-2.5 rounded-2xl bg-[#FFF6E8] border border-[#F2DFBE] flex items-center justify-between gap-2 text-[11px]"
      >
        <div class="min-w-0 text-[#8C5510]">
          <span class="font-bold text-[#DE8A1E]">绿联云已授权：</span>
          <span class="truncate">{{ gatewayBind.gatewayName || gatewayBind.gatewayUid }}</span>
        </div>
        <button
          type="button"
          class="shrink-0 font-semibold text-[#64748B] hover:text-[#18181B]"
          @click="cancelGatewayBind"
        >
          取消绑定
        </button>
      </div>

      <form
        :class="layout === 'mobile' ? 'app-card p-5 space-y-3 text-xs' : 'space-y-3 text-xs'"
        @submit.prevent="submitRegister"
      >
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
          <div>
            <label class="block text-[11px] font-semibold text-[#475569] mb-1">登录用户名</label>
            <input
              v-model="regUsername"
              type="text"
              required
              minlength="2"
              maxlength="32"
              placeholder="用于账号登录（不含@）"
              class="w-full h-10 text-xs bg-[#F5F6F8] border border-[#E6E9EF] rounded-xl px-3.5 outline-none focus:border-[#DE8A1E] focus:bg-white transition"
            />
          </div>
          <div>
            <label class="block text-[11px] font-semibold text-[#475569] mb-1">显示昵称</label>
            <input
              v-model="regName"
              type="text"
              required
              maxlength="16"
              placeholder="如：林小禾"
              class="w-full h-10 text-xs bg-[#F5F6F8] border border-[#E6E9EF] rounded-xl px-3.5 outline-none focus:border-[#DE8A1E] focus:bg-white transition"
            />
          </div>
        </div>

        <div>
          <label class="block text-[11px] font-semibold text-[#475569] mb-1">邮箱地址</label>
          <input
            v-model="regEmail"
            type="email"
            required
            placeholder="用于登录、找回密码与家庭邀请"
            class="w-full h-10 text-xs bg-[#F5F6F8] border border-[#E6E9EF] rounded-xl px-3.5 outline-none focus:border-[#DE8A1E] focus:bg-white transition"
          />
        </div>

        <VerificationCodeField
          v-model="regCode"
          label="邮箱验证码"
          :placeholder="store.authStatus?.require_email_verify ? '请输入 6 位验证码（必填）' : '6 位验证码（如开启验证则必填）'"
          :required="Boolean(store.authStatus?.require_email_verify)"
          :cooldown="cooldown"
          :sending="sendingCode"
          :sent-hint="sentHintMap.register || null"
          variant="auth"
          @send="handleSendCode('register')"
        />

        <div class="grid gap-2.5" :class="gatewayBind ? 'grid-cols-1' : 'grid-cols-1 sm:grid-cols-2'">
          <div>
            <label class="block text-[11px] font-semibold text-[#475569] mb-1">设置密码</label>
            <input
              v-model="regPassword"
              type="password"
              required
              minlength="6"
              placeholder="至少 6 位字母或数字"
              class="w-full h-10 text-xs bg-[#F5F6F8] border border-[#E6E9EF] rounded-xl px-3.5 outline-none focus:border-[#DE8A1E] focus:bg-white transition"
            />
          </div>
          <div v-if="!gatewayBind">
            <label class="block text-[11px] font-semibold text-[#475569] mb-1">邀请码（选填）</label>
            <input
              v-model="regInviteCode"
              type="text"
              maxlength="24"
              placeholder="如有邀请码请填写"
              class="w-full h-10 text-xs bg-[#F5F6F8] border border-[#E6E9EF] rounded-xl px-3.5 outline-none focus:border-[#DE8A1E] focus:bg-white transition"
            />
          </div>
        </div>

        <label class="flex items-center gap-2 pt-0.5 text-[11px] text-[#64748B] cursor-pointer">
          <input v-model="regAgreed" type="checkbox" class="accent-[#DE8A1E] rounded" />
          <span>我已阅读并同意《用户协议》与《隐私政策》</span>
        </label>

        <button
          type="submit"
          :disabled="submitting"
          class="btn-amber w-full h-11 rounded-2xl text-white text-xs font-semibold disabled:opacity-60 transition"
        >
          {{ submitting ? '提交中…' : gatewayBind ? '完成绑定并登录' : '注册并登录' }}
        </button>
      </form>

      <div class="mt-4 text-center text-xs text-[#64748B]">
        已有账号？
        <button
          type="button"
          class="font-bold text-[#DE8A1E] hover:underline ml-1"
          @click="switchView('login')"
        >
          返回登录
        </button>
      </div>
    </template>

    <!-- ==================== FORGOT PASSWORD VIEW ==================== -->
    <template v-else>
      <div class="pt-1 pb-4">
        <h2 class="text-xl font-bold text-[#18181B]">重置登录密码</h2>
        <p class="text-xs text-[#64748B] mt-1">通过已绑定的邮箱验证码快速重置密码</p>
      </div>

      <form
        :class="layout === 'mobile' ? 'app-card p-5 space-y-3 text-xs' : 'space-y-3 text-xs'"
        @submit.prevent="submitForgot"
      >
        <div>
          <label class="block text-[11px] font-semibold text-[#475569] mb-1">绑定邮箱</label>
          <input
            v-model="forgotEmail"
            type="email"
            required
            placeholder="请输入注册时绑定的邮箱"
            class="w-full h-10 text-xs bg-[#F5F6F8] border border-[#E6E9EF] rounded-xl px-3.5 outline-none focus:border-[#DE8A1E] focus:bg-white transition"
          />
        </div>

        <VerificationCodeField
          v-model="forgotCode"
          label="邮箱验证码"
          placeholder="请输入 6 位数字验证码"
          :required="true"
          :cooldown="cooldown"
          :sending="sendingCode"
          :sent-hint="sentHintMap.forgot || null"
          variant="auth"
          @send="handleSendCode('forgot')"
        />

        <div>
          <label class="block text-[11px] font-semibold text-[#475569] mb-1">新密码</label>
          <input
            v-model="forgotNewPwd"
            type="password"
            required
            minlength="6"
            placeholder="请输入新密码（至少6位）"
            class="w-full h-10 text-xs bg-[#F5F6F8] border border-[#E6E9EF] rounded-xl px-3.5 outline-none focus:border-[#DE8A1E] focus:bg-white transition"
          />
        </div>

        <div>
          <label class="block text-[11px] font-semibold text-[#475569] mb-1">确认新密码</label>
          <input
            v-model="forgotConfirmPwd"
            type="password"
            required
            minlength="6"
            placeholder="请再次输入新密码"
            class="w-full h-10 text-xs bg-[#F5F6F8] border border-[#E6E9EF] rounded-xl px-3.5 outline-none focus:border-[#DE8A1E] focus:bg-white transition"
          />
        </div>

        <button
          type="submit"
          :disabled="submitting"
          class="btn-amber w-full h-11 rounded-2xl text-white text-xs font-semibold disabled:opacity-60 transition"
        >
          {{ submitting ? '提交中…' : '确认重置并返回登录' }}
        </button>
      </form>

      <div class="mt-4 text-center text-xs text-[#64748B]">
        <button
          type="button"
          class="font-bold text-[#DE8A1E] hover:underline"
          @click="switchView('login')"
        >
          ← 返回密码登录
        </button>
      </div>
    </template>
  </div>
</template>
