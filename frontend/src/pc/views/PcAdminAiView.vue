<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue';
import { api } from '@/api/client';
import { useAppStore } from '@/stores/useAppStore';
import { resolveAvatarUrl } from '@/constants/avatars';
import { formatTokens } from '@/utils/format';
import {
  normalizeAiConfig,
  normalizeAdminUser,
  type NormalizedAdminUser,
} from '@/pc/utils/adminAdapter';
import type { AdminAiConfig, AiTestResponse } from '@/types';
import CustomDropdown from '@/components/shared/CustomDropdown.vue';
import type { DropdownOption } from '@/components/shared/CustomDropdown.vue';
import ToggleSwitch from '@/components/shared/ToggleSwitch.vue';

const store = useAppStore();

const PROVIDER_OPTIONS: DropdownOption[] = [
  { value: 'DeepSeek', label: 'DeepSeek' },
  { value: 'OpenAI 兼容接口', label: 'OpenAI 兼容接口' },
  { value: '通义千问 Qwen', label: '通义千问 Qwen' },
  { value: 'Google Gemini', label: 'Google Gemini' },
];

const form = reactive({
  enabled: true,
  provider: 'DeepSeek',
  baseUrl: 'https://api.deepseek.com/v1',
  apiKeyInput: '',
  hasApiKey: false,
  model: 'deepseek-chat',
  dailyLimit: 30,
  ocrEnabled: true,
  insightsEnabled: true,
});

const apiKeyVisible = ref(false);
const savingConfig = ref(false);
const testingConn = ref(false);
const testResult = ref<AiTestResponse | null>(null);

const searchQuery = ref('');
const loadingUsage = ref(false);
const totalInputTokens = ref(0);
const totalOutputTokens = ref(0);
const totalCalls = ref(0);
const todayCalls = ref(0);
const usageUsers = ref<NormalizedAdminUser[]>([]);

const limitModalOpen = ref(false);
const editingLimitUser = ref<NormalizedAdminUser | null>(null);
const limitInput = ref(30);
const savingUserLimit = ref(false);

const filteredUsers = computed(() => {
  const q = searchQuery.value.trim().toLowerCase();
  const list = [...usageUsers.value].sort(
    (a, b) => b.inputTokens + b.outputTokens - (a.inputTokens + a.outputTokens)
  );
  if (!q) return list;
  return list.filter(
    (u) => u.name.toLowerCase().includes(q) || u.email.toLowerCase().includes(q)
  );
});

async function loadAiConfig() {
  try {
    const raw = await api.getAdminAiConfig();
    const cfg = normalizeAiConfig(raw);
    form.enabled = cfg.enabled;
    form.provider = cfg.provider;
    form.baseUrl = cfg.baseUrl;
    form.hasApiKey = cfg.hasApiKey;
    form.apiKeyInput = '';
    form.model = cfg.model;
    form.dailyLimit = cfg.dailyLimit;
    form.ocrEnabled = cfg.ocrEnabled;
    form.insightsEnabled = cfg.insightsEnabled;
  } catch (err: any) {
    store.showToast(err?.message || '读取 AI 配置失败', 'error');
  }
}

async function loadAiUsage(search?: string) {
  loadingUsage.value = true;
  try {
    const res = await api.getAdminAiUsage(search);
    const anyRes = res as any;
    totalInputTokens.value = Number(res.total_input_tokens ?? 0);
    totalOutputTokens.value = Number(res.total_output_tokens ?? 0);
    totalCalls.value = Number(anyRes.total_calls ?? 0);
    todayCalls.value = Number(anyRes.today_calls ?? 0);
    usageUsers.value = Array.isArray(res.users) ? res.users.map(normalizeAdminUser) : [];
  } catch (err: any) {
    store.showToast(err?.message || '读取 AI 用量失败', 'error');
  } finally {
    loadingUsage.value = false;
  }
}

onMounted(() => {
  loadAiConfig();
  loadAiUsage();
});

async function handleSaveConfig() {
  if (savingConfig.value) return;
  savingConfig.value = true;
  try {
    const payload: Record<string, any> = {
      enabled: form.enabled,
      provider: form.provider,
      base_url: form.baseUrl.trim(),
      model: form.model.trim(),
      daily_limit: Math.max(1, Number(form.dailyLimit) || 30),
      ocr_enabled: form.ocrEnabled,
      insights_enabled: form.insightsEnabled,
    };
    // 仅当用户新输入了 API Key 时才更新，留空保持后端原值
    if (form.apiKeyInput.trim()) {
      payload.api_key = form.apiKeyInput.trim();
    }

    const updated = await api.updateAdminAiConfig(payload as Partial<AdminAiConfig>);
    const cfg = normalizeAiConfig(updated);
    form.hasApiKey = cfg.hasApiKey || Boolean(form.apiKeyInput.trim());
    form.apiKeyInput = '';
    store.showToast('AI 模型配置已保存', 'success');
  } catch (err: any) {
    store.showToast(err?.message || '保存 AI 配置失败', 'error');
  } finally {
    savingConfig.value = false;
  }
}

async function handleTestConnection() {
  if (testingConn.value) return;
  testingConn.value = true;
  try {
    const res = await api.testAiConnection({
      base_url: form.baseUrl.trim(),
      api_key: form.apiKeyInput.trim() || undefined,
      model: form.model.trim(),
    });
    testResult.value = res;
    store.showToast(res.message || (res.connected ? '接口连通正常' : '连接失败'), res.connected ? 'success' : 'error');
  } catch (err: any) {
    store.showToast(err?.message || '连通性测试失败', 'error');
  } finally {
    testingConn.value = false;
  }
}

function openEditLimitModal(u: NormalizedAdminUser) {
  editingLimitUser.value = u;
  limitInput.value = u.dailyAiLimit;
  limitModalOpen.value = true;
}

async function handleSaveUserLimit() {
  if (!editingLimitUser.value || savingUserLimit.value) return;
  savingUserLimit.value = true;
  try {
    const nextLimit = Math.max(0, Math.round(Number(limitInput.value) || 0));
    await api.updateAdminUser(editingLimitUser.value.id, {
      daily_ai_limit: nextLimit,
    });
    limitModalOpen.value = false;
    await loadAiUsage(searchQuery.value.trim() || undefined);
    store.showToast(`已调整「${editingLimitUser.value.name}」日限额为 ${nextLimit} 次`, 'success');
  } catch (err: any) {
    store.showToast(err?.message || '调整限额失败', 'error');
  } finally {
    savingUserLimit.value = false;
  }
}
</script>

<template>
  <div class="space-y-5">
    <!-- 上方：AI 模型配置 + Token 用量汇总 -->
    <section class="grid grid-cols-1 lg:grid-cols-5 gap-5">
      <form class="pc-card p-5 lg:col-span-3 space-y-4 text-xs" @submit.prevent="handleSaveConfig">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <div>
            <h2 class="text-sm font-bold text-[#18181B]">AI 模型与 API Key 配置</h2>
            <p class="text-[11px] text-[#64748B] mt-0.5">
              支持 OpenAI 兼容接口、语音转写解析与小票视觉 OCR
            </p>
          </div>

          <div class="flex items-center gap-2">
            <span class="text-[11px] font-semibold text-[#64748B]">
              {{ form.enabled ? 'AI 服务已启用' : 'AI 服务已停用' }}
            </span>
            <ToggleSwitch v-model="form.enabled" aria-label="启用全站 AI 服务" />
          </div>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <div>
            <label class="block text-[11px] font-semibold text-[#475569] mb-1">服务商</label>
            <CustomDropdown v-model="form.provider" :options="PROVIDER_OPTIONS" />
          </div>
          <div>
            <label class="block text-[11px] font-semibold text-[#475569] mb-1">模型名称</label>
            <input
              v-model="form.model"
              type="text"
              required
              placeholder="如：deepseek-chat / gpt-4o-mini"
              class="w-full h-9 bg-[#F4F5F8] border border-[#EAECEF] rounded-xl px-3 outline-none focus:border-[#DE8A1E] focus:bg-white"
            />
          </div>
        </div>

        <div>
          <label class="block text-[11px] font-semibold text-[#475569] mb-1">API Base URL</label>
          <input
            v-model="form.baseUrl"
            type="url"
            required
            placeholder="https://api.deepseek.com/v1"
            class="w-full h-9 bg-[#F4F5F8] border border-[#EAECEF] rounded-xl px-3 outline-none focus:border-[#DE8A1E] focus:bg-white"
          />
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
          <div class="sm:col-span-2">
            <div class="flex items-center justify-between mb-1">
              <label class="text-[11px] font-semibold text-[#475569]">API Key</label>
              <span class="text-[10px]" :class="form.hasApiKey ? 'text-[#3E8E63]' : 'text-[#94A3B8]'">
                {{ form.hasApiKey ? '已配置密钥（留空保持原值）' : '未配置外部 Key（使用内置解析）' }}
              </span>
            </div>
            <div class="flex gap-2">
              <input
                v-model="form.apiKeyInput"
                :type="apiKeyVisible ? 'text' : 'password'"
                :placeholder="form.hasApiKey ? '••••••••••••••••（留空不修改）' : '输入新 API Key'"
                class="flex-1 min-w-0 h-9 font-mono text-xs bg-[#F4F5F8] border border-[#EAECEF] rounded-xl px-3 outline-none focus:border-[#DE8A1E] focus:bg-white"
              />
              <button
                type="button"
                class="h-9 px-3 rounded-xl bg-[#F4F5F8] hover:bg-[#EAECEF] font-semibold text-[#475569] shrink-0"
                @click="apiKeyVisible = !apiKeyVisible"
              >
                {{ apiKeyVisible ? '隐藏' : '显示' }}
              </button>
            </div>
          </div>

          <div>
            <label class="block text-[11px] font-semibold text-[#475569] mb-1">全站默认日限额</label>
            <input
              v-model.number="form.dailyLimit"
              type="number"
              min="1"
              max="9999"
              class="w-full h-9 font-bold tabular-nums bg-[#F4F5F8] border border-[#EAECEF] rounded-xl px-3 outline-none focus:border-[#DE8A1E] focus:bg-white"
            />
          </div>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <div class="flex items-center justify-between px-3 py-2.5 rounded-xl bg-[#FAFBFD] border border-[#EAECEF]">
            <div>
              <p class="font-semibold text-[#18181B]">小票视觉 OCR 识别</p>
              <p class="text-[10px] text-[#94A3B8]">允许上传小票或账单截图自动识图</p>
            </div>
            <ToggleSwitch v-model="form.ocrEnabled" aria-label="小票视觉 OCR 识别" />
          </div>

          <div class="flex items-center justify-between px-3 py-2.5 rounded-xl bg-[#FAFBFD] border border-[#EAECEF]">
            <div>
              <p class="font-semibold text-[#18181B]">消费诊断与洞察建议</p>
              <p class="text-[10px] text-[#94A3B8]">在统计报表页提供 AI 智能建议卡片</p>
            </div>
            <ToggleSwitch v-model="form.insightsEnabled" aria-label="消费诊断与洞察建议" />
          </div>
        </div>

        <!-- 连通性测试结果条 -->
        <div
          v-if="testResult"
          class="px-3.5 py-2.5 rounded-xl border text-xs flex items-center justify-between gap-2"
          :class="
            testResult.connected
              ? 'bg-[#ECF6F0] border-[#C6E5D2] text-[#2F6F4C]'
              : 'bg-[#FDF0EE] border-[#F5C4BC] text-[#D6553E]'
          "
        >
          <span class="truncate">{{ testResult.message }}</span>
          <span class="font-mono font-semibold shrink-0">
            {{ testResult.mode }} · {{ testResult.latency_ms }}ms
          </span>
        </div>

        <div class="flex items-center justify-end gap-2 pt-2 border-t border-[#F1F3F6]">
          <button
            type="button"
            :disabled="testingConn"
            class="px-4 py-2 rounded-xl bg-[#F4F5F8] hover:bg-[#EAECEF] text-xs font-semibold text-[#18181B] disabled:opacity-60 transition"
            @click="handleTestConnection"
          >
            {{ testingConn ? '测试中…' : '在线测试连通性' }}
          </button>
          <button
            type="submit"
            :disabled="savingConfig"
            class="btn-amber px-4 py-2 rounded-xl text-white text-xs font-semibold disabled:opacity-60 transition"
          >
            {{ savingConfig ? '保存中…' : '保存 AI 配置' }}
          </button>
        </div>
      </form>

      <!-- 右侧 Token 汇总 -->
      <div class="pc-card p-5 lg:col-span-2 flex flex-col justify-between space-y-4">
        <div>
          <h2 class="text-sm font-bold text-[#18181B]">全站 AI Token 汇总</h2>
          <p class="text-3xl font-bold text-[#18181B] tabular-nums mt-3">
            {{ formatTokens(totalInputTokens + totalOutputTokens) }}
          </p>
          <p class="text-[11px] text-[#94A3B8] mt-1 tabular-nums">
            今日调用 {{ todayCalls }} 次 · 累计调用 {{ totalCalls }} 次
          </p>

          <div class="grid grid-cols-2 gap-3 mt-4 pt-4 border-t border-[#F1F3F6] text-xs">
            <div class="p-3 rounded-2xl bg-[#FAFBFD] border border-[#EAECEF]">
              <p class="text-[10px] text-[#94A3B8]">Input Token</p>
              <p class="font-bold text-[#18181B] tabular-nums mt-0.5">
                {{ formatTokens(totalInputTokens) }}
              </p>
            </div>
            <div class="p-3 rounded-2xl bg-[#FAFBFD] border border-[#EAECEF]">
              <p class="text-[10px] text-[#94A3B8]">Output Token</p>
              <p class="font-bold text-[#DE8A1E] tabular-nums mt-0.5">
                {{ formatTokens(totalOutputTokens) }}
              </p>
            </div>
          </div>
        </div>

        <div class="text-xs text-[#64748B] flex items-center justify-between pt-4 border-t border-[#F1F3F6]">
          <span>参考估算成本</span>
          <span class="font-bold text-[#3E8E63] tabular-nums">
            ¥{{ (((totalInputTokens + totalOutputTokens) / 1_000_000) * 6.8).toFixed(2) }}
          </span>
        </div>
      </div>
    </section>

    <!-- 下方：全站用户 AI 用量排行 -->
    <section class="pc-card overflow-hidden">
      <div class="px-5 py-3.5 border-b border-[#F1F3F6] flex flex-wrap items-center justify-between gap-2">
        <div>
          <h2 class="text-sm font-bold text-[#18181B]">分用户 Token 用量与日限额排行</h2>
          <p class="text-[11px] text-[#94A3B8]">支持按昵称或邮箱过滤并调整单用户日调用上限</p>
        </div>
        <div class="flex items-center gap-2">
          <input
            v-model="searchQuery"
            type="text"
            placeholder="按昵称或邮箱搜索…"
            class="w-56 h-8 text-xs bg-[#F4F5F8] border border-[#EAECEF] rounded-xl px-3 outline-none focus:border-[#DE8A1E] focus:bg-white"
            @keyup.enter="loadAiUsage(searchQuery.trim() || undefined)"
          />
          <button
            type="button"
            class="h-8 px-3 rounded-xl bg-[#F4F5F8] hover:bg-[#EAECEF] text-xs font-semibold text-[#475569]"
            @click="loadAiUsage(searchQuery.trim() || undefined)"
          >
            {{ loadingUsage ? '查询中…' : '查询' }}
          </button>
        </div>
      </div>

      <div v-if="filteredUsers.length === 0" class="py-10 text-center text-xs text-[#94A3B8]">
        暂无匹配的 AI 用量记录
      </div>
      <div v-else class="overflow-x-auto">
        <table class="w-full text-left text-xs border-collapse">
          <thead>
            <tr class="bg-[#FAFBFD] text-[#94A3B8] border-b border-[#F1F3F6]">
              <th class="py-2.5 px-5 font-semibold">用户</th>
              <th class="py-2.5 px-4 font-semibold">今日调用 / 上限</th>
              <th class="py-2.5 px-4 font-semibold text-right">Input</th>
              <th class="py-2.5 px-4 font-semibold text-right">Output</th>
              <th class="py-2.5 px-4 font-semibold text-right">合计 Token</th>
              <th class="py-2.5 px-5 font-semibold text-right">操作</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-[#F1F3F6]">
            <tr v-for="u in filteredUsers" :key="u.id" class="hover:bg-[#FAFBFD] transition">
              <td class="py-3 px-5">
                <div class="flex items-center gap-2.5">
                  <img
                    :src="resolveAvatarUrl(u.avatar)"
                    :alt="u.name"
                    class="w-7 h-7 rounded-full object-cover border border-[#EAECEF] bg-white shrink-0"
                  />
                  <div class="min-w-0">
                    <p class="font-semibold text-[#18181B] truncate">{{ u.name }}</p>
                    <p class="text-[10px] text-[#94A3B8] truncate">{{ u.email || '未绑定邮箱' }}</p>
                  </div>
                </div>
              </td>
              <td class="py-3 px-4">
                <div class="flex items-center gap-2">
                  <div class="w-20 h-1.5 rounded-full bg-[#ECEEF2] overflow-hidden">
                    <div
                      class="h-full rounded-full"
                      :class="
                        u.dailyAiLimit > 0 && (u.todayAiCount / u.dailyAiLimit) >= 0.85
                          ? 'bg-[#D6553E]'
                          : 'bg-[#DE8A1E]'
                      "
                      :style="{
                        width: `${Math.min(100, Math.round((u.todayAiCount / Math.max(1, u.dailyAiLimit)) * 100))}%`,
                      }"
                    />
                  </div>
                  <span class="tabular-nums font-semibold">
                    {{ u.todayAiCount }}/{{ u.dailyAiLimit }}
                  </span>
                </div>
              </td>
              <td class="py-3 px-4 text-right tabular-nums text-[#64748B]">
                {{ formatTokens(u.inputTokens) }}
              </td>
              <td class="py-3 px-4 text-right tabular-nums text-[#64748B]">
                {{ formatTokens(u.outputTokens) }}
              </td>
              <td class="py-3 px-4 text-right tabular-nums font-bold text-[#18181B]">
                {{ formatTokens(u.inputTokens + u.outputTokens) }}
              </td>
              <td class="py-3 px-5 text-right">
                <button
                  type="button"
                  class="text-[11px] font-semibold px-2.5 py-1 rounded-lg bg-[#FFF6E8] hover:bg-[#FBEBD1] text-[#DE8A1E]"
                  @click="openEditLimitModal(u)"
                >
                  调限额
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <!-- 调整用户 AI 日限额弹窗 -->
    <Teleport to="body">
      <div v-if="limitModalOpen" class="fixed inset-0 z-[70] flex items-center justify-center p-4">
        <div
          class="absolute inset-0 bg-[#0F172A]/40 backdrop-blur-[2px]"
          @click="limitModalOpen = false"
        />
        <div class="relative w-full max-w-[380px] bg-white rounded-[22px] border border-[#EAECEF] shadow-2xl p-5 space-y-4">
          <h3 class="text-sm font-bold text-[#18181B]">
            调整「{{ editingLimitUser?.name }}」每日 AI 次数上限
          </h3>
          <form class="space-y-3 text-xs" @submit.prevent="handleSaveUserLimit">
            <input
              v-model.number="limitInput"
              type="number"
              min="0"
              max="9999"
              required
              class="w-full h-10 text-base font-bold tabular-nums bg-[#F4F5F8] border border-[#EAECEF] rounded-xl px-3.5 outline-none focus:border-[#DE8A1E] focus:bg-white"
            />
            <div class="flex justify-end gap-2">
              <button
                type="button"
                class="px-4 py-2 rounded-xl bg-[#F4F5F8] text-[#475569] font-semibold"
                @click="limitModalOpen = false"
              >
                取消
              </button>
              <button
                type="submit"
                :disabled="savingUserLimit"
                class="btn-amber px-4 py-2 rounded-xl text-white font-semibold disabled:opacity-60"
              >
                {{ savingUserLimit ? '保存中…' : '保存' }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </Teleport>
  </div>
</template>
