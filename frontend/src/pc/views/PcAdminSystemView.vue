<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { api } from '@/api/client';
import { useAppStore } from '@/stores/useAppStore';
import { formatFileSize } from '@/utils/format';
import { normalizeSystemConfig, downloadWithAuth } from '@/pc/utils/adminAdapter';
import type { AdminSystemConfig, Snapshot } from '@/types';
import CustomDropdown from '@/components/shared/CustomDropdown.vue';
import type { DropdownOption } from '@/components/shared/CustomDropdown.vue';
import ToggleSwitch from '@/components/shared/ToggleSwitch.vue';

const store = useAppStore();

const BACKUP_CYCLES: DropdownOption[] = [
  { value: '每天 03:00', label: '每天 03:00' },
  { value: '每周日 03:00', label: '每周日 03:00' },
  { value: '每月 1 日 03:00', label: '每月 1 日 03:00' },
];

const form = reactive({
  smtpHost: '',
  smtpPort: 465,
  smtpTls: true,
  smtpUser: '',
  smtpFrom: '',
  smtpPassInput: '',
  hasSmtpPass: false,
  autoBackupEnabled: false,
  autoBackupCycle: '每天 03:00',
  autoBackupRetain: 14,
  backupPath: '/data/snapshots',
});

const testEmailTarget = ref('');
const savingSmtp = ref(false);
const testingSmtp = ref(false);
const savingBackup = ref(false);

const snapshots = ref<Snapshot[]>([]);
const loadingSnapshots = ref(false);
const creatingSnapshot = ref(false);
const downloadingId = ref<string | null>(null);
const confirmDeleteSnapId = ref<string | null>(null);

const restoreFile = ref<File | null>(null);
const restoreModalOpen = ref(false);
const restoring = ref(false);

async function loadSystemConfig() {
  try {
    const raw = await api.getAdminSystemConfig();
    const cfg = normalizeSystemConfig(raw);
    form.smtpHost = cfg.smtpHost;
    form.smtpPort = cfg.smtpPort;
    form.smtpTls = cfg.smtpTls;
    form.smtpUser = cfg.smtpUser;
    form.smtpFrom = cfg.smtpFrom;
    form.hasSmtpPass = cfg.hasSmtpPass;
    form.smtpPassInput = '';
    form.autoBackupEnabled = cfg.autoBackupEnabled;
    form.autoBackupCycle = cfg.autoBackupCycle;
    form.autoBackupRetain = cfg.autoBackupRetain;
    form.backupPath = cfg.backupPath;
    if (!testEmailTarget.value) {
      testEmailTarget.value = store.user?.email || cfg.smtpUser || '';
    }
  } catch (err: any) {
    store.showToast(err?.message || '读取系统配置失败', 'error');
  }
}

async function loadSnapshots() {
  loadingSnapshots.value = true;
  try {
    const list = await api.listSnapshots();
    snapshots.value = Array.isArray(list) ? list : [];
  } catch (err: any) {
    store.showToast(err?.message || '读取备份快照列表失败', 'error');
  } finally {
    loadingSnapshots.value = false;
  }
}

onMounted(() => {
  loadSystemConfig();
  loadSnapshots();
});

async function handleSaveSmtp() {
  if (savingSmtp.value) return;
  savingSmtp.value = true;
  try {
    const payload: Record<string, any> = {
      smtp_host: form.smtpHost.trim(),
      smtp_port: Number(form.smtpPort) || 465,
      smtp_tls: form.smtpTls,
      smtp_user: form.smtpUser.trim(),
      smtp_from: form.smtpFrom.trim(),
    };
    if (form.smtpPassInput) {
      payload.smtp_pass = form.smtpPassInput;
    }

    const updated = await api.updateAdminSystemConfig(payload as Partial<AdminSystemConfig>);
    const cfg = normalizeSystemConfig(updated);
    form.hasSmtpPass = cfg.hasSmtpPass || Boolean(form.smtpPassInput);
    form.smtpPassInput = '';
    store.showToast('SMTP 发信邮箱配置已保存', 'success');
  } catch (err: any) {
    store.showToast(err?.message || '保存 SMTP 配置失败', 'error');
  } finally {
    savingSmtp.value = false;
  }
}

async function handleTestSmtp() {
  if (testingSmtp.value) return;
  testingSmtp.value = true;
  try {
    const payload: Record<string, any> = {
      email: testEmailTarget.value.trim() || form.smtpUser.trim(),
      smtp_host: form.smtpHost.trim(),
      smtp_port: Number(form.smtpPort) || 465,
      smtp_tls: form.smtpTls,
      smtp_user: form.smtpUser.trim(),
      smtp_from: form.smtpFrom.trim(),
    };
    if (form.smtpPassInput) {
      payload.smtp_pass = form.smtpPassInput;
    }

    const res = await api.testAdminSmtp(payload as any);
    const anyRes = res as any;
    const isOk = Boolean(anyRes?.ok ?? anyRes?.sent);
    store.showToast(
      anyRes?.message || (isOk ? '测试邮件发送成功' : '测试邮件发送失败'),
      isOk ? 'success' : 'error',
      3500
    );
  } catch (err: any) {
    store.showToast(err?.message || '测试发信失败', 'error');
  } finally {
    testingSmtp.value = false;
  }
}

async function handleSaveBackupConfig(silent = false) {
  if (savingBackup.value) return;
  savingBackup.value = true;
  try {
    const payload: Record<string, any> = {
      auto_backup_enabled: form.autoBackupEnabled,
      auto_backup: form.autoBackupEnabled,
      auto_backup_cycle: form.autoBackupCycle,
      backup_cycle: form.autoBackupCycle,
      auto_backup_retain: Math.max(1, Number(form.autoBackupRetain) || 14),
      keep_count: Math.max(1, Number(form.autoBackupRetain) || 14),
    };
    const updated = await api.updateAdminSystemConfig(payload as Partial<AdminSystemConfig>);
    const cfg = normalizeSystemConfig(updated);
    form.autoBackupEnabled = cfg.autoBackupEnabled;
    form.autoBackupCycle = cfg.autoBackupCycle;
    form.autoBackupRetain = cfg.autoBackupRetain;
    if (!silent) {
      store.showToast('自动备份策略已保存', 'success');
    } else {
      store.showToast(
        form.autoBackupEnabled ? '已开启定时自动备份' : '已关闭定时自动备份',
        'info'
      );
    }
  } catch (err: any) {
    store.showToast(err?.message || '保存备份策略失败', 'error');
  } finally {
    savingBackup.value = false;
  }
}

async function handleCreateSnapshot() {
  if (creatingSnapshot.value) return;
  creatingSnapshot.value = true;
  try {
    await api.createSnapshot();
    await loadSnapshots();
    store.showToast('已创建最新数据库热快照', 'success');
  } catch (err: any) {
    store.showToast(err?.message || '创建快照失败', 'error');
  } finally {
    creatingSnapshot.value = false;
  }
}

async function handleDownloadSnapshot(snap: Snapshot) {
  if (downloadingId.value) return;
  downloadingId.value = snap.id;
  try {
    await downloadWithAuth(
      api.getSnapshotDownloadUrl(snap.id),
      snap.file_name || `cat7_snapshot_${snap.id}.db`
    );
    store.showToast(`已开始下载 ${snap.file_name}`, 'success');
  } catch (err: any) {
    store.showToast(err?.message || '下载快照失败', 'error');
  } finally {
    downloadingId.value = null;
  }
}

async function handleDeleteSnapshot(snap: Snapshot) {
  if (confirmDeleteSnapId.value !== snap.id) {
    confirmDeleteSnapId.value = snap.id;
    return;
  }
  try {
    await api.deleteSnapshot(snap.id);
    confirmDeleteSnapId.value = null;
    await loadSnapshots();
    store.showToast(`已删除快照「${snap.file_name}」`, 'success');
  } catch (err: any) {
    store.showToast(err?.message || '删除快照失败', 'error');
  }
}

function onRestoreFileSelected(e: Event) {
  const input = e.target as HTMLInputElement;
  const file = input.files?.[0];
  if (!file) return;
  restoreFile.value = file;
  restoreModalOpen.value = true;
  input.value = '';
}

async function handleConfirmRestore() {
  if (!restoreFile.value || restoring.value) return;
  restoring.value = true;
  try {
    await api.restoreSnapshot(restoreFile.value);
    restoreModalOpen.value = false;
    restoreFile.value = null;
    await Promise.all([loadSnapshots(), store.initApp()]);
    store.showToast('数据库快照恢复成功，已重新加载工作台数据', 'success', 3500);
  } catch (err: any) {
    store.showToast(err?.message || '恢复数据库快照失败', 'error');
  } finally {
    restoring.value = false;
  }
}
</script>

<template>
  <div class="space-y-5">
    <!-- 上方：SMTP 邮箱配置与自动备份策略 -->
    <section class="grid grid-cols-1 lg:grid-cols-2 gap-5">
      <!-- SMTP 配置 -->
      <form class="pc-card p-5 space-y-3.5 text-xs" @submit.prevent="handleSaveSmtp">
        <div class="flex items-center justify-between">
          <div>
            <h2 class="text-sm font-bold text-[#18181B]">发信邮箱（SMTP）配置</h2>
            <p class="text-[11px] text-[#64748B] mt-0.5">
              用于验证码登录、密码重置以及家庭账本邮件邀请
            </p>
          </div>
          <div class="flex items-center gap-2">
            <span class="text-[11px] font-semibold text-[#64748B]">TLS 加密</span>
            <ToggleSwitch v-model="form.smtpTls" aria-label="启用 SMTP TLS 加密" />
          </div>
        </div>

        <div class="grid grid-cols-3 gap-2.5">
          <div class="col-span-2">
            <label class="block text-[11px] font-semibold text-[#475569] mb-1">SMTP 服务器</label>
            <input
              v-model="form.smtpHost"
              type="text"
              placeholder="如：smtp.qq.com"
              class="w-full h-9 bg-[#F4F5F8] border border-[#EAECEF] rounded-xl px-3 outline-none focus:border-[#DE8A1E] focus:bg-white"
            />
          </div>
          <div>
            <label class="block text-[11px] font-semibold text-[#475569] mb-1">端口</label>
            <input
              v-model.number="form.smtpPort"
              type="number"
              min="1"
              max="65535"
              class="w-full h-9 tabular-nums bg-[#F4F5F8] border border-[#EAECEF] rounded-xl px-3 outline-none focus:border-[#DE8A1E] focus:bg-white"
            />
          </div>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
          <div>
            <label class="block text-[11px] font-semibold text-[#475569] mb-1">发件账号（User）</label>
            <input
              v-model="form.smtpUser"
              type="text"
              placeholder="如：noreply@example.com"
              class="w-full h-9 bg-[#F4F5F8] border border-[#EAECEF] rounded-xl px-3 outline-none focus:border-[#DE8A1E] focus:bg-white"
            />
          </div>
          <div>
            <label class="block text-[11px] font-semibold text-[#475569] mb-1">发件人显示（From）</label>
            <input
              v-model="form.smtpFrom"
              type="text"
              placeholder="可选，默认同发件账号"
              class="w-full h-9 bg-[#F4F5F8] border border-[#EAECEF] rounded-xl px-3 outline-none focus:border-[#DE8A1E] focus:bg-white"
            />
          </div>
        </div>

        <div>
          <div class="flex items-center justify-between mb-1">
            <label class="text-[11px] font-semibold text-[#475569]">SMTP 授权码 / 密码</label>
            <span class="text-[10px]" :class="form.hasSmtpPass ? 'text-[#3E8E63]' : 'text-[#94A3B8]'">
              {{ form.hasSmtpPass ? '已配置授权码（留空保持原值）' : '尚未配置' }}
            </span>
          </div>
          <input
            v-model="form.smtpPassInput"
            type="password"
            :placeholder="form.hasSmtpPass ? '••••••••••••（留空不覆盖）' : '输入邮箱授权码'"
            class="w-full h-9 bg-[#F4F5F8] border border-[#EAECEF] rounded-xl px-3 outline-none focus:border-[#DE8A1E] focus:bg-white"
          />
        </div>

        <div class="pt-2 border-t border-[#F1F3F6] flex flex-wrap items-center justify-between gap-2">
          <div class="flex items-center gap-2 flex-1 min-w-[200px]">
            <input
              v-model="testEmailTarget"
              type="email"
              placeholder="测试收件人邮箱"
              class="flex-1 min-w-0 h-9 bg-[#F4F5F8] border border-[#EAECEF] rounded-xl px-3 outline-none focus:border-[#DE8A1E] focus:bg-white"
            />
            <button
              type="button"
              :disabled="testingSmtp"
              class="h-9 px-3 rounded-xl bg-[#F4F5F8] hover:bg-[#EAECEF] font-semibold text-[#18181B] shrink-0 disabled:opacity-60 transition"
              @click="handleTestSmtp"
            >
              {{ testingSmtp ? '发送中…' : '发送测试邮件' }}
            </button>
          </div>
          <button
            type="submit"
            :disabled="savingSmtp"
            class="btn-amber h-9 px-4 rounded-xl text-white font-semibold shrink-0 disabled:opacity-60 transition"
          >
            {{ savingSmtp ? '保存中…' : '保存邮箱配置' }}
          </button>
        </div>
      </form>

      <!-- 定时自动备份策略 -->
      <div class="pc-card p-5 space-y-4 text-xs flex flex-col justify-between">
        <div class="space-y-4">
          <div class="flex items-center justify-between gap-3">
            <div>
              <h2 class="text-sm font-bold text-[#18181B]">数据库定时自动备份</h2>
              <p class="text-[11px] text-[#64748B] font-mono mt-0.5 truncate">
                存储路径：{{ form.backupPath }}
              </p>
            </div>
            <ToggleSwitch
              v-model="form.autoBackupEnabled"
              aria-label="开启数据库定时自动备份"
              @change="handleSaveBackupConfig(true)"
            />
          </div>

          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div>
              <label class="block text-[11px] font-semibold text-[#475569] mb-1">备份周期</label>
              <CustomDropdown v-model="form.autoBackupCycle" :options="BACKUP_CYCLES" />
            </div>
            <div>
              <label class="block text-[11px] font-semibold text-[#475569] mb-1">最大保留份数</label>
              <input
                v-model.number="form.autoBackupRetain"
                type="number"
                min="1"
                max="180"
                class="w-full h-9 font-bold tabular-nums bg-[#F4F5F8] border border-[#EAECEF] rounded-xl px-3 outline-none focus:border-[#DE8A1E] focus:bg-white"
              />
            </div>
          </div>

          <div class="p-3.5 rounded-2xl bg-[#FAFBFD] border border-[#EAECEF] text-[11px] text-[#64748B] leading-relaxed">
            开启后后台任务会按选定周期自动生成 SQLite 在线热快照（VACUUM INTO），并在超出保留份数后自动清理最早的旧快照，保障 NAS 数据长期可回溯。
          </div>
        </div>

        <div class="pt-2 border-t border-[#F1F3F6] flex justify-end">
          <button
            type="button"
            :disabled="savingBackup"
            class="btn-amber h-9 px-4 rounded-xl text-white font-semibold disabled:opacity-60 transition"
            @click="handleSaveBackupConfig(false)"
          >
            {{ savingBackup ? '保存中…' : '保存备份策略' }}
          </button>
        </div>
      </div>
    </section>

    <!-- 下方：SQLite 数据库热快照管理与上传恢复 -->
    <section class="pc-card overflow-hidden">
      <div class="px-5 py-4 border-b border-[#F1F3F6] flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 class="text-sm font-bold text-[#18181B]">SQLite 数据库热快照管理</h2>
          <p class="text-[11px] text-[#64748B] mt-0.5">
            支持在线零停机创建热快照、带鉴权导出下载，或上传历史快照文件恢复全库
          </p>
        </div>

        <div class="flex flex-wrap items-center gap-2">
          <label
            class="cursor-pointer h-9 px-3.5 rounded-xl bg-[#FDF0EE] hover:bg-[#FADCD7] text-[#D6553E] text-xs font-semibold inline-flex items-center gap-1.5 transition"
          >
            <span>上传快照恢复</span>
            <input
              type="file"
              accept=".db,.sqlite,.sqlite3,.gz"
              class="hidden"
              @change="onRestoreFileSelected"
            />
          </label>
          <button
            type="button"
            :disabled="creatingSnapshot"
            class="btn-amber h-9 px-4 rounded-xl text-white text-xs font-semibold disabled:opacity-60 transition"
            @click="handleCreateSnapshot"
          >
            {{ creatingSnapshot ? '创建中…' : '+ 立即创建热快照' }}
          </button>
        </div>
      </div>

      <div v-if="loadingSnapshots" class="py-10 text-center text-xs text-[#94A3B8]">
        正在读取快照列表…
      </div>
      <div v-else-if="snapshots.length === 0" class="py-12 text-center text-xs text-[#94A3B8]">
        暂无数据库快照，点击右上角「+ 立即创建热快照」生成第一份备份
      </div>
      <div v-else class="overflow-x-auto">
        <table class="w-full text-left text-xs border-collapse">
          <thead>
            <tr class="bg-[#FAFBFD] text-[#94A3B8] border-b border-[#F1F3F6]">
              <th class="py-2.5 px-5 font-semibold">快照文件名</th>
              <th class="py-2.5 px-4 font-semibold">大小</th>
              <th class="py-2.5 px-4 font-semibold">SHA-256 校验</th>
              <th class="py-2.5 px-4 font-semibold">创建方式 / 时间</th>
              <th class="py-2.5 px-5 font-semibold text-right">操作</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-[#F1F3F6]">
            <tr v-for="snap in snapshots" :key="snap.id" class="hover:bg-[#FAFBFD] transition">
              <td class="py-3 px-5 font-mono font-semibold text-[#18181B]">
                {{ snap.file_name }}
              </td>
              <td class="py-3 px-4 tabular-nums text-[#64748B]">
                {{ formatFileSize(snap.file_size) }}
              </td>
              <td class="py-3 px-4 font-mono text-[11px] text-[#94A3B8]">
                {{ snap.sha256 ? `${snap.sha256.slice(0, 12)}…` : '-' }}
              </td>
              <td class="py-3 px-4 text-[#64748B]">
                <span>{{ snap.created_by === 'system_auto' ? '自动定时' : snap.created_by || '手动' }}</span>
                <span class="text-[#94A3B8] ml-1.5">{{ snap.created_at }}</span>
              </td>
              <td class="py-3 px-5 text-right space-x-3 whitespace-nowrap">
                <button
                  type="button"
                  :disabled="downloadingId === snap.id"
                  class="text-[11px] font-semibold text-[#DE8A1E] hover:underline disabled:opacity-50"
                  @click="handleDownloadSnapshot(snap)"
                >
                  {{ downloadingId === snap.id ? '下载中…' : '下载' }}
                </button>
                <button
                  type="button"
                  class="text-[11px] font-semibold transition"
                  :class="
                    confirmDeleteSnapId === snap.id
                      ? 'text-[#D6553E] underline'
                      : 'text-[#94A3B8] hover:text-[#D6553E]'
                  "
                  @click="handleDeleteSnapshot(snap)"
                >
                  {{ confirmDeleteSnapId === snap.id ? '确认删除？' : '删除' }}
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <!-- 快照恢复二次确认弹窗 -->
    <Teleport to="body">
      <div v-if="restoreModalOpen" class="fixed inset-0 z-[75] flex items-center justify-center p-4">
        <div
          class="absolute inset-0 bg-[#0F172A]/45 backdrop-blur-[2px]"
          @click="!restoring && (restoreModalOpen = false)"
        />
        <div class="relative w-full max-w-[440px] bg-white rounded-[24px] border border-[#EAECEF] shadow-2xl p-6 space-y-4">
          <div class="flex items-start gap-3">
            <div class="w-10 h-10 rounded-2xl bg-[#FDF0EE] text-[#D6553E] flex items-center justify-center shrink-0 font-bold text-base">
              !
            </div>
            <div class="space-y-1">
              <h3 class="text-sm font-bold text-[#18181B]">确认从快照文件恢复全库？</h3>
              <p class="text-xs text-[#64748B] leading-relaxed">
                即将使用文件 <strong class="text-[#18181B]">{{ restoreFile?.name }}</strong> 覆盖当前数据库。建议在执行恢复前先点击「+ 立即创建热快照」留存一份当前数据副本。
              </p>
            </div>
          </div>

          <div class="flex justify-end gap-2 pt-2">
            <button
              type="button"
              :disabled="restoring"
              class="px-4 py-2 rounded-xl bg-[#F4F5F8] text-xs font-semibold text-[#475569] disabled:opacity-60"
              @click="restoreModalOpen = false; restoreFile = null"
            >
              取消
            </button>
            <button
              type="button"
              :disabled="restoring"
              class="px-4 py-2 rounded-xl bg-[#D6553E] hover:bg-[#BE4530] text-white text-xs font-semibold disabled:opacity-60 transition"
              @click="handleConfirmRestore"
            >
              {{ restoring ? '正在恢复数据库…' : '确认覆盖恢复' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
