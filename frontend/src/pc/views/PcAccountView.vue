<script setup lang="ts">
import { ref, computed, watch, onBeforeUnmount } from 'vue';
import { api } from '@/api/client';
import { CAT_AVATARS, resolveAvatarUrl } from '@/constants/avatars';
import { useAppStore } from '@/stores/useAppStore';
import VerificationCodeField, { type CodeDeliveryHint } from '@/components/shared/VerificationCodeField.vue';

const emit = defineEmits<{
  'open-auth': [tab: 'login' | 'register' | 'forgot'];
}>();

const store = useAppStore();

const nickname = ref('');
const savingName = ref(false);

const bindEmailInput = ref('');
const bindCodeInput = ref('');
const sendingBindCode = ref(false);
const bindingEmail = ref(false);
const cooldown = ref(0);
const bindSentHint = ref<CodeDeliveryHint | null>(null);
let cooldownTimer: ReturnType<typeof setInterval> | null = null;

watch(
  () => store.user?.user_name,
  (val) => {
    nickname.value = val || '';
  },
  { immediate: true }
);

const currentAvatarUrl = computed(() => resolveAvatarUrl(store.user?.avatar));

const roleLabel = computed(() => {
  if (store.isRealAdmin) return '系统管理员';
  if (store.user?.role === 'guest') return '默认访客';
  return '正式成员';
});

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

async function applyAvatarUpdate(avatarValue: string) {
  try {
    const updated = await api.updateProfile({ avatar: avatarValue });
    store.user = updated;
    await store.refreshBooks();
  } catch (err: any) {
    store.showToast(err?.message || '保存头像失败', 'error');
  }
}

function handleOpenAvatarPicker() {
  store.openAvatarModal(
    'profile',
    async (picked: string) => {
      await applyAvatarUpdate(picked);
    },
    store.user?.avatar
  );
}

async function handlePickPresetAvatar(publicPath: string, catName: string) {
  await applyAvatarUpdate(publicPath);
  store.showToast(`已切换头像为「${catName}」`, 'success');
}

async function handleSaveNickname() {
  if (savingName.value) return;
  const trimmed = nickname.value.trim();
  if (!trimmed) {
    store.showToast('昵称不能为空', 'error');
    return;
  }

  savingName.value = true;
  try {
    const updated = await api.updateProfile({ name: trimmed });
    store.user = updated;
    await store.refreshBooks();
    store.showToast('个人昵称已更新', 'success');
  } catch (err: any) {
    store.showToast(err?.message || '更新昵称失败', 'error');
  } finally {
    savingName.value = false;
  }
}

async function handleSendBindCode() {
  if (cooldown.value > 0 || sendingBindCode.value) return;
  const email = bindEmailInput.value.trim();
  if (!email) {
    store.showToast('请先输入新的邮箱地址', 'error');
    return;
  }

  sendingBindCode.value = true;
  try {
    const res = await api.sendCode({ email, purpose: 'bind' });
    startCooldown();
    bindSentHint.value = {
      email,
      demoCode: res.demo_code || undefined,
    };
    if (res.demo_code) {
      store.showToast(`验证码 ${res.demo_code} 已发送（演示模式）`, 'info', 4000);
    } else {
      store.showToast('验证码已发送至邮箱', 'success');
    }
  } catch (err: any) {
    store.showToast(err?.message || '发送验证码失败', 'error');
  } finally {
    sendingBindCode.value = false;
  }
}

async function handleBindEmail() {
  if (bindingEmail.value) return;
  const email = bindEmailInput.value.trim();
  const code = bindCodeInput.value.trim();
  if (!email || !code) {
    store.showToast('请填写邮箱与 6 位验证码', 'error');
    return;
  }

  bindingEmail.value = true;
  try {
    const updated = await api.bindEmail({ email, code });
    store.user = updated;
    bindEmailInput.value = '';
    bindCodeInput.value = '';
    bindSentHint.value = null;
    store.showToast(`安全邮箱已更新为 ${updated.email || email}`, 'success');
  } catch (err: any) {
    store.showToast(err?.message || '绑定邮箱失败', 'error');
  } finally {
    bindingEmail.value = false;
  }
}

async function handleLogoutToGuest() {
  await store.logoutToGuest();
  store.showToast(
    store.authRequired ? '已退出登录，请重新登录' : '已退出当前账号并切回默认身份',
    'info'
  );
}

onBeforeUnmount(() => {
  if (cooldownTimer) {
    clearInterval(cooldownTimer);
    cooldownTimer = null;
  }
});
</script>

<template>
  <div class="grid grid-cols-1 lg:grid-cols-2 gap-5 items-start">
    <!-- 左侧：个人资料与 3D 小猫头像 -->
    <section class="pc-card p-5 space-y-5">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div class="flex items-center gap-3.5 min-w-0">
          <button
            type="button"
            title="点击更换或裁剪头像"
            class="relative group shrink-0"
            @click="handleOpenAvatarPicker"
          >
            <img
              :src="currentAvatarUrl"
              :alt="store.user?.user_name || '用户头像'"
              class="w-16 h-16 rounded-full object-cover border-2 border-white shadow-sm bg-white group-hover:ring-2 group-hover:ring-[#DE8A1E]/50 transition"
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

          <div class="min-w-0">
            <div class="flex items-center gap-2">
              <h2 class="text-base font-bold text-[#18181B] truncate">
                {{ store.user?.user_name || '七猫用户' }}
              </h2>
              <span
                class="text-[10px] font-semibold px-2 py-0.5 rounded-full shrink-0"
                :class="
                  store.isRealAdmin
                    ? 'bg-[#18181B] text-white'
                    : 'bg-[#FFF6E8] text-[#DE8A1E]'
                "
              >
                {{ roleLabel }}
              </span>
            </div>
            <p class="text-[11px] text-[#64748B] mt-0.5">
              {{ store.user?.username ? `@${store.user.username} · ` : '' }}{{
                store.user?.gateway_bound
                  ? '已关联绿联云授权 · 账号密码多端通用'
                  : '独立账号会话 · 支持跨设备登录同步'
              }}
            </p>
          </div>
        </div>

        <button
          type="button"
          class="px-3.5 py-2 rounded-xl bg-[#18181B] hover:bg-[#27272A] text-white text-xs font-semibold transition shrink-0"
          @click="handleOpenAvatarPicker"
        >
          上传裁剪头像
        </button>
      </div>

      <!-- 3D 小猫预设头像快速切换 -->
      <div class="space-y-2">
        <p class="text-[11px] font-semibold text-[#64748B]">3D 毛绒小猫头像库</p>
        <div class="grid grid-cols-3 sm:grid-cols-6 gap-2.5">
          <button
            v-for="cat in CAT_AVATARS"
            :key="cat.id"
            type="button"
            class="flex flex-col items-center gap-1 p-1.5 rounded-2xl transition"
            :class="
              currentAvatarUrl === cat.src
                ? 'bg-[#FFF6E8] ring-2 ring-[#DE8A1E]'
                : 'hover:bg-[#F4F5F8]'
            "
            @click="handlePickPresetAvatar(cat.publicPath, cat.name)"
          >
            <img
              :src="cat.src"
              :alt="cat.name"
              class="w-12 h-12 rounded-full object-cover shadow-xs bg-white"
            />
            <span class="text-[10px] font-semibold text-[#475569]">{{ cat.name }}</span>
          </button>
        </div>
      </div>

      <!-- 昵称修改 -->
      <form class="pt-3 border-t border-[#F1F3F6] space-y-2 text-xs" @submit.prevent="handleSaveNickname">
        <label class="block text-[11px] font-semibold text-[#64748B]">显示昵称</label>
        <div class="flex gap-2">
          <input
            v-model="nickname"
            type="text"
            required
            maxlength="20"
            placeholder="请输入显示昵称"
            class="flex-1 min-w-0 h-9 bg-[#F4F5F8] border border-[#EAECEF] rounded-xl px-3.5 outline-none focus:border-[#DE8A1E] focus:bg-white"
          />
          <button
            type="submit"
            :disabled="savingName"
            class="h-9 px-4 rounded-xl bg-[#DE8A1E] hover:bg-[#C67612] text-white font-semibold shrink-0 disabled:opacity-60 transition"
          >
            {{ savingName ? '保存中…' : '保存昵称' }}
          </button>
        </div>
      </form>

      <!-- 个人记账成就概览 -->
      <div class="grid grid-cols-3 gap-2.5 pt-2 border-t border-[#F1F3F6] text-xs">
        <div class="p-3 rounded-2xl bg-[#FAFBFD] border border-[#EAECEF]">
          <p class="text-[10px] text-[#94A3B8]">累计记账</p>
          <p class="font-bold text-[#18181B] tabular-nums mt-0.5">
            {{ store.user?.total_transactions_count ?? 0 }} 笔
          </p>
        </div>
        <div class="p-3 rounded-2xl bg-[#FAFBFD] border border-[#EAECEF]">
          <p class="text-[10px] text-[#94A3B8]">连续坚持</p>
          <p class="font-bold text-[#3E8E63] tabular-nums mt-0.5">
            {{ store.user?.streak_days ?? 0 }} 天
          </p>
        </div>
        <div class="p-3 rounded-2xl bg-[#FAFBFD] border border-[#EAECEF]">
          <p class="text-[10px] text-[#94A3B8]">今日 AI 配额</p>
          <p class="font-bold text-[#DE8A1E] tabular-nums mt-0.5">
            {{ store.user?.today_ai_count ?? 0 }}/{{ store.user?.daily_ai_limit ?? 30 }}
          </p>
        </div>
      </div>
    </section>

    <!-- 右侧：安全邮箱绑定与登录切换 -->
    <section class="pc-card p-5 space-y-5">
      <div class="flex items-center justify-between gap-2">
        <div>
          <h2 class="text-sm font-bold text-[#18181B]">安全邮箱绑定</h2>
          <p class="text-[11px] text-[#64748B] mt-0.5">
            {{
              store.user?.email
                ? `当前已绑定：${store.user.email}`
                : '尚未绑定安全邮箱，绑定后支持邮箱验证码登录与找回密码'
            }}
          </p>
        </div>
        <span
          class="text-[10px] font-semibold px-2.5 py-1 rounded-full shrink-0"
          :class="
            store.user?.email_bound
              ? 'bg-[#ECF6F0] text-[#3E8E63]'
              : 'bg-[#FEF6E7] text-[#D9822B]'
          "
        >
          {{ store.user?.email_bound ? '已绑定' : '未绑定' }}
        </span>
      </div>

      <form class="space-y-3 text-xs" @submit.prevent="handleBindEmail">
        <div>
          <label class="block text-[11px] font-semibold text-[#64748B] mb-1">
            {{ store.user?.email_bound ? '更换安全邮箱地址' : '安全邮箱地址' }}
          </label>
          <input
            v-model="bindEmailInput"
            type="email"
            required
            placeholder="请输入要绑定的邮箱地址"
            class="w-full h-10 bg-[#F4F5F8] border border-[#EAECEF] rounded-xl px-3.5 outline-none focus:border-[#DE8A1E] focus:bg-white transition"
          />
        </div>

        <VerificationCodeField
          v-model="bindCodeInput"
          label="邮箱验证码"
          placeholder="请输入 6 位邮箱验证码"
          :required="true"
          :cooldown="cooldown"
          :sending="sendingBindCode"
          :sent-hint="bindSentHint"
          variant="pc"
          @send="handleSendBindCode"
        />

        <button
          type="submit"
          :disabled="bindingEmail"
          class="btn-amber w-full h-10 rounded-xl text-white font-semibold disabled:opacity-60 transition"
        >
          {{
            bindingEmail
              ? '提交绑定中…'
              : store.user?.email_bound
                ? '确认更换绑定邮箱'
                : '确认绑定安全邮箱'
          }}
        </button>
      </form>

      <!-- 账号登录、注册与会话操作 -->
      <div class="pt-4 border-t border-[#F1F3F6] flex flex-wrap items-center justify-between gap-2 text-xs">
        <div class="flex flex-wrap items-center gap-2">
          <button
            type="button"
            class="px-3 py-1.5 rounded-xl bg-[#F4F5F8] hover:bg-[#EAECEF] font-semibold text-[#475569] transition"
            @click="emit('open-auth', 'login')"
          >
            登录 / 切换账号
          </button>
          <button
            type="button"
            class="px-3 py-1.5 rounded-xl bg-[#F4F5F8] hover:bg-[#EAECEF] font-semibold text-[#475569] transition"
            @click="emit('open-auth', 'register')"
          >
            注册新账号
          </button>
          <button
            type="button"
            class="px-3 py-1.5 rounded-xl bg-[#F4F5F8] hover:bg-[#EAECEF] font-semibold text-[#475569] transition"
            @click="emit('open-auth', 'forgot')"
          >
            重置密码
          </button>
        </div>

        <button
          v-if="store.isAuthenticated"
          type="button"
          class="px-3 py-1.5 rounded-xl bg-[#FDF0EE] hover:bg-[#FADCD7] font-semibold text-[#D6553E] transition"
          @click="handleLogoutToGuest"
        >
          退出登录
        </button>
      </div>
    </section>
  </div>
</template>
