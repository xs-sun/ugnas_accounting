<script setup lang="ts">
import { ref, reactive, computed, watch } from 'vue';
import { api } from '@/api/client';
import { useAppStore } from '@/stores/useAppStore';
import { resolveAvatarUrl } from '@/constants/avatars';
import { LEDGER_COLOR_PRESETS } from '@/constants/icons';
import { centsToYuan, yuanToCents, formatMoney } from '@/utils/format';
import type { Book, BookMember } from '@/types';
import CustomDropdown from '@/components/shared/CustomDropdown.vue';
import type { DropdownOption } from '@/components/shared/CustomDropdown.vue';

const store = useAppStore();

const selectedBookId = ref<string>(store.activeBookId || '');
const createBookOpen = ref(false);
const creatingBook = ref(false);
const joiningByCode = ref(false);
const joinCodeInput = ref('');

const newBookForm = reactive({
  name: '',
  description: '',
  color: LEDGER_COLOR_PRESETS[0],
  budgetYuan: '4500',
});

const editBookForm = reactive({
  name: '',
  description: '',
  color: LEDGER_COLOR_PRESETS[0],
  budgetYuan: '4500',
});
const savingEditBook = ref(false);
const confirmDeleteBook = ref(false);
const deletingBook = ref(false);
const settingDefaultId = ref('');

const members = ref<BookMember[]>([]);
const loadingMembers = ref(false);

const inviteEmail = ref('');
const inviteRole = ref<'editor' | 'viewer'>('editor');
const invitingEmail = ref(false);
const regeneratingInviteCode = ref(false);

const virtualName = ref('');
const virtualColor = ref('#3E8E63');
const creatingVirtual = ref(false);

const ROLE_OPTIONS: DropdownOption[] = [
  { value: 'editor', label: '可记账成员' },
  { value: 'viewer', label: '仅查看成员' },
];

const selectedBook = computed<Book | null>(() => {
  if (!store.books.length) return null;
  return (
    store.books.find((b) => b.id === selectedBookId.value) ||
    store.activeBook ||
    store.books[0] ||
    null
  );
});

const isOwnerOfSelected = computed<boolean>(() => {
  if (!selectedBook.value) return false;
  if (selectedBook.value.role === 'owner') return true;
  return Boolean(store.user?.user_id && selectedBook.value.owner_id === store.user.user_id);
});

function syncEditForm(book: Book | null) {
  confirmDeleteBook.value = false;
  if (!book) return;
  editBookForm.name = book.name;
  editBookForm.description = book.description || '';
  editBookForm.color = book.color || LEDGER_COLOR_PRESETS[0];
  editBookForm.budgetYuan = centsToYuan(book.budget).toFixed(0);
}

async function loadMembers(bookId: string) {
  if (!bookId) {
    members.value = [];
    return;
  }
  loadingMembers.value = true;
  try {
    members.value = await api.listMembers(bookId);
  } catch {
    members.value = selectedBook.value?.members || [];
  } finally {
    loadingMembers.value = false;
  }
}

watch(
  () => store.activeBookId,
  (bid) => {
    if (bid && !store.books.some((b) => b.id === selectedBookId.value)) {
      selectedBookId.value = bid;
    }
  },
  { immediate: true }
);

watch(
  selectedBook,
  (book) => {
    syncEditForm(book);
    if (book?.id) {
      selectedBookId.value = book.id;
      loadMembers(book.id);
    } else {
      members.value = [];
    }
  },
  { immediate: true }
);

async function handleSelectBook(book: Book) {
  selectedBookId.value = book.id;
  if (book.id !== store.activeBookId) {
    await store.switchBook(book.id);
    store.showToast(`已切换至「${book.name}」`, 'success');
  }
}

async function handleToggleDefaultBook(book: Book) {
  if (settingDefaultId.value) return;
  settingDefaultId.value = book.id;
  try {
    if (book.is_default) {
      await store.clearDefaultBook();
      store.showToast(`已取消「${book.name}」的默认账本设置`, 'info');
    } else {
      await store.setDefaultBook(book.id);
      store.showToast(`已将「${book.name}」设为默认账本，打开应用时自动加载`, 'success');
    }
  } catch (err: any) {
    store.showToast(err?.message || '设置默认账本失败', 'error');
  } finally {
    settingDefaultId.value = '';
  }
}

async function handleCreateBook() {
  if (creatingBook.value) return;
  const name = newBookForm.name.trim();
  if (!name) return;

  creatingBook.value = true;
  try {
    const created = await api.createBook({
      name,
      description: newBookForm.description.trim() || '自定义新建账本',
      color: newBookForm.color,
      budget: Math.max(0, yuanToCents(newBookForm.budgetYuan)),
    });
    newBookForm.name = '';
    newBookForm.description = '';
    createBookOpen.value = false;
    await store.refreshBooks();
    selectedBookId.value = created.id;
    await store.switchBook(created.id);
    store.showToast(`已创建并切换至「${created.name}」`, 'success');
  } catch (err: any) {
    store.showToast(err?.message || '创建账本失败', 'error');
  } finally {
    creatingBook.value = false;
  }
}

async function handleJoinByCode() {
  if (joiningByCode.value) return;
  const code = joinCodeInput.value.trim().toUpperCase();
  if (!code) return;

  joiningByCode.value = true;
  try {
    const joined = await api.joinBookByCode({ invite_code: code });
    joinCodeInput.value = '';
    await store.refreshBooks();
    selectedBookId.value = joined.id;
    await store.switchBook(joined.id);
    store.showToast(`已成功加入「${joined.name}」`, 'success');
  } catch (err: any) {
    store.showToast(err?.message || '口令无效或加入失败', 'error');
  } finally {
    joiningByCode.value = false;
  }
}

async function handleUpdateSelectedBook() {
  if (!selectedBook.value || savingEditBook.value) return;
  const name = editBookForm.name.trim();
  if (!name) return;

  savingEditBook.value = true;
  try {
    await api.updateBook(selectedBook.value.id, {
      name,
      description: editBookForm.description.trim(),
      color: editBookForm.color,
      budget: Math.max(0, yuanToCents(editBookForm.budgetYuan)),
    });
    await store.refreshBooks();
    if (selectedBook.value.id === store.activeBookId) {
      await store.refreshBudgets(store.activeBookId, store.currentYear, store.currentMonth);
    }
    store.showToast('账本信息已保存', 'success');
  } catch (err: any) {
    store.showToast(err?.message || '保存账本失败', 'error');
  } finally {
    savingEditBook.value = false;
  }
}

async function handleDeleteBook() {
  if (!selectedBook.value || deletingBook.value) return;
  if (store.books.length <= 1) {
    store.showToast('系统至少保留一个账本', 'error');
    return;
  }
  if (!confirmDeleteBook.value) {
    confirmDeleteBook.value = true;
    return;
  }

  deletingBook.value = true;
  try {
    const targetId = selectedBook.value.id;
    const targetName = selectedBook.value.name;
    await api.deleteBook(targetId);
    confirmDeleteBook.value = false;
    await store.refreshBooks();
    if (store.activeBookId) {
      selectedBookId.value = store.activeBookId;
      await store.refreshBookData(store.activeBookId);
    }
    store.showToast(`已删除账本「${targetName}」`, 'success');
  } catch (err: any) {
    store.showToast(err?.message || '删除账本失败', 'error');
  } finally {
    deletingBook.value = false;
  }
}

async function copyInviteCode(code: string) {
  if (!code) return;
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(code);
    }
    store.showToast(`已复制一次性口令：${code}（仅限使用一次）`, 'success');
  } catch {
    store.showToast(`一次性口令：${code}`, 'info');
  }
}

async function handleRegenerateInviteCode() {
  if (!selectedBook.value || regeneratingInviteCode.value) return;
  regeneratingInviteCode.value = true;
  try {
    const res = await api.regenerateInviteCode(selectedBook.value.id, {
      role: inviteRole.value,
    });
    await store.refreshBooks();
    store.showToast(`已生成新的一次性口令：${res.invite_code}（旧码已作废）`, 'success');
  } catch (err: any) {
    store.showToast(err?.message || '生成新口令失败', 'error');
  } finally {
    regeneratingInviteCode.value = false;
  }
}

async function handleInviteByEmail() {
  if (!selectedBook.value || invitingEmail.value) return;
  const email = inviteEmail.value.trim();
  if (!email) return;

  invitingEmail.value = true;
  try {
    const res = await api.inviteMemberByEmail(selectedBook.value.id, {
      email,
      role: inviteRole.value,
    });
    inviteEmail.value = '';
    await Promise.all([loadMembers(selectedBook.value.id), store.refreshBooks()]);
    store.showToast(
      res.smtp_sent
        ? `已向 ${email} 发送邮件邀请`
        : `已添加邀请成员（口令 ${res.invite_code}）`,
      'success'
    );
  } catch (err: any) {
    store.showToast(err?.message || '邀请成员失败', 'error');
  } finally {
    invitingEmail.value = false;
  }
}

async function handleCreateVirtualMember() {
  if (!selectedBook.value || creatingVirtual.value) return;
  const name = virtualName.value.trim();
  if (!name) return;

  creatingVirtual.value = true;
  try {
    await api.createVirtualMember(selectedBook.value.id, {
      name,
      color: virtualColor.value,
    });
    virtualName.value = '';
    await Promise.all([loadMembers(selectedBook.value.id), store.refreshBooks()]);
    store.showToast(`已添加虚拟成员「${name}」`, 'success');
  } catch (err: any) {
    store.showToast(err?.message || '添加虚拟成员失败', 'error');
  } finally {
    creatingVirtual.value = false;
  }
}

async function handleRemoveMember(m: BookMember) {
  if (!selectedBook.value) return;
  try {
    await api.removeMember(selectedBook.value.id, m.user_id);
    await Promise.all([loadMembers(selectedBook.value.id), store.refreshBooks()]);
    store.showToast(`已移除成员「${m.user_name}」`, 'success');
  } catch (err: any) {
    store.showToast(err?.message || '移除成员失败', 'error');
  }
}

function getMemberRoleLabel(role: string): string {
  if (role === 'owner') return '管理员';
  if (role === 'viewer') return '仅查看';
  return '可记账';
}
</script>

<template>
  <div class="grid grid-cols-1 lg:grid-cols-12 gap-5 items-start">
    <!-- 左侧 5 列：账本列表、新建账本与口令加入 -->
    <section class="lg:col-span-5 space-y-4">
      <div class="pc-card p-5 space-y-3.5">
        <div class="flex items-center justify-between">
          <h2 class="text-sm font-bold text-[#18181B]">
            我的账本 ({{ store.books.length }})
          </h2>
          <button
            type="button"
            class="text-xs font-semibold text-[#DE8A1E] hover:underline"
            @click="createBookOpen = !createBookOpen"
          >
            {{ createBookOpen ? '收起表单' : '+ 新建账本' }}
          </button>
        </div>

        <!-- 新建账本表单 -->
        <form
          v-if="createBookOpen"
          class="p-4 rounded-2xl bg-[#FAFBFD] border border-[#EAECEF] space-y-3 text-xs"
          @submit.prevent="handleCreateBook"
        >
          <p class="font-bold text-[#18181B]">创建新账本</p>
          <input
            v-model="newBookForm.name"
            type="text"
            required
            maxlength="20"
            placeholder="账本名称（如：家庭共同账本、旅行账本）"
            class="w-full h-9 bg-white border border-[#EAECEF] rounded-xl px-3 outline-none focus:border-[#DE8A1E]"
          />
          <input
            v-model="newBookForm.description"
            type="text"
            maxlength="40"
            placeholder="简短描述（选填）"
            class="w-full h-9 bg-white border border-[#EAECEF] rounded-xl px-3 outline-none focus:border-[#DE8A1E]"
          />
          <div class="flex items-center justify-between gap-2">
            <div class="flex items-center gap-1.5">
              <button
                v-for="c in LEDGER_COLOR_PRESETS"
                :key="c"
                type="button"
                class="w-6 h-6 rounded-full transition"
                :class="newBookForm.color === c ? 'ring-2 ring-offset-2 ring-[#18181B]' : ''"
                :style="{ backgroundColor: c }"
                @click="newBookForm.color = c"
              />
            </div>
            <div class="flex items-center gap-1.5">
              <span class="text-[11px] text-[#64748B]">月预算</span>
              <input
                v-model="newBookForm.budgetYuan"
                type="number"
                min="0"
                step="100"
                class="w-24 h-8 bg-white border border-[#EAECEF] rounded-lg px-2 font-bold tabular-nums outline-none"
              />
            </div>
          </div>
          <button
            type="submit"
            :disabled="creatingBook"
            class="btn-amber w-full h-9 rounded-xl text-white font-semibold disabled:opacity-60"
          >
            {{ creatingBook ? '创建中…' : '确认创建并切换' }}
          </button>
        </form>

        <!-- 账本列表 -->
        <div class="space-y-2.5">
          <div
            v-for="book in store.books"
            :key="book.id"
            role="button"
            tabindex="0"
            class="w-full p-4 rounded-2xl border text-left transition cursor-pointer"
            :class="
              selectedBook?.id === book.id
                ? 'bg-[#FFFBF4] border-[#DE8A1E]'
                : 'bg-[#FAFBFD] border-[#EAECEF] hover:bg-white'
            "
            @click="handleSelectBook(book)"
            @keydown.enter.prevent="handleSelectBook(book)"
          >
            <div class="flex items-center justify-between gap-2">
              <div class="flex items-center gap-2 min-w-0">
                <span
                  class="w-2.5 h-2.5 rounded-full shrink-0"
                  :style="{ backgroundColor: book.color || '#DE8A1E' }"
                />
                <span class="text-sm font-bold text-[#18181B] truncate">{{ book.name }}</span>
              </div>
              <div class="flex items-center gap-1.5 shrink-0">
                <span
                  v-if="book.id === store.activeBookId"
                  class="text-[10px] font-semibold px-2 py-0.5 rounded-full bg-[#FFF6E8] text-[#DE8A1E]"
                >
                  当前使用
                </span>
                <button
                  type="button"
                  :disabled="Boolean(settingDefaultId)"
                  class="text-[10px] font-semibold px-2 py-0.5 rounded-full border transition"
                  :class="
                    book.is_default
                      ? 'bg-[#ECF6F0] border-[#BDE2CE] text-[#2B7A52]'
                      : 'bg-white border-[#E6E9EF] text-[#64748B] hover:border-[#DE8A1E] hover:text-[#DE8A1E]'
                  "
                  :title="book.is_default ? '点击取消默认账本' : '设为打开应用时自动加载的默认账本'"
                  @click.stop="handleToggleDefaultBook(book)"
                >
                  {{ settingDefaultId === book.id ? '设置中…' : book.is_default ? '默认账本' : '设为默认' }}
                </button>
              </div>
            </div>

            <p class="text-[11px] text-[#64748B] mt-1 truncate">
              {{ book.description || '多成员协作记账本' }} · 月预算
              {{ formatMoney(book.budget, { privacy: store.privacyHidden }) }}
            </p>

            <div class="flex items-center justify-between mt-2.5">
              <div class="flex items-center gap-1.5">
                <img
                  v-for="m in (book.members || []).slice(0, 5)"
                  :key="m.user_id"
                  :src="resolveAvatarUrl(m.avatar)"
                  :alt="m.user_name"
                  :title="m.user_name"
                  class="w-6 h-6 rounded-full object-cover ring-2 ring-white bg-white"
                />
                <span class="text-[10px] text-[#94A3B8] ml-1">
                  {{ book.members?.length || 1 }} 人参与
                </span>
              </div>
              <span class="text-[10px] text-[#94A3B8] tabular-nums">
                {{ book.tx_count ?? 0 }} 笔流水
              </span>
            </div>
          </div>
        </div>
      </div>

      <!-- 通过口令加入账本 -->
      <form class="pc-card p-5 space-y-3 text-xs" @submit.prevent="handleJoinByCode">
        <div>
          <h3 class="text-sm font-bold text-[#18181B]">通过口令加入他人账本</h3>
          <p class="text-[11px] text-[#64748B] mt-0.5">输入家人或伙伴分享的一次性邀请口令（一码一用）</p>
        </div>
        <div class="flex gap-2">
          <input
            v-model="joinCodeInput"
            type="text"
            required
            maxlength="24"
            placeholder="输入一次性口令，如 CAT7-XXXXXXXX"
            class="flex-1 min-w-0 h-9 uppercase font-mono bg-[#F4F5F8] border border-[#EAECEF] rounded-xl px-3 outline-none focus:border-[#DE8A1E] focus:bg-white"
          />
          <button
            type="submit"
            :disabled="joiningByCode"
            class="h-9 px-4 rounded-xl bg-[#18181B] hover:bg-[#27272A] text-white font-semibold shrink-0 disabled:opacity-60 transition"
          >
            {{ joiningByCode ? '加入中…' : '加入账本' }}
          </button>
        </div>
      </form>
    </section>

    <!-- 右侧 7 列：选中账本编辑、邀请口令与成员管理 -->
    <section class="lg:col-span-7 space-y-5">
      <template v-if="selectedBook">
        <!-- 基本信息编辑 -->
        <form class="pc-card p-5 space-y-4 text-xs" @submit.prevent="handleUpdateSelectedBook">
          <div class="flex flex-wrap items-center justify-between gap-2 border-b border-[#F1F3F6] pb-3">
            <div>
              <h2 class="text-sm font-bold text-[#18181B]">
                {{ selectedBook.name }} · 账本设置
              </h2>
              <p class="text-[11px] text-[#64748B]">编辑账本名称、配色标识与月度总预算</p>
            </div>

            <div class="flex flex-wrap items-center gap-2">
              <button
                type="button"
                :disabled="Boolean(settingDefaultId)"
                class="text-xs font-semibold px-3 py-1.5 rounded-xl border transition"
                :class="
                  selectedBook.is_default
                    ? 'bg-[#ECF6F0] border-[#BDE2CE] text-[#2B7A52]'
                    : 'bg-[#F4F5F8] border-transparent text-[#475569] hover:border-[#DE8A1E] hover:text-[#DE8A1E]'
                "
                @click="handleToggleDefaultBook(selectedBook)"
              >
                {{
                  settingDefaultId === selectedBook.id
                    ? '设置中…'
                    : selectedBook.is_default
                      ? '✓ 默认启动账本'
                      : '设为默认账本'
                }}
              </button>
              <template v-if="isOwnerOfSelected">
                <button
                  v-if="selectedBook.invite_code"
                  type="button"
                  class="text-xs font-semibold px-3 py-1.5 rounded-xl bg-[#FFF6E8] hover:bg-[#FBEBD1] text-[#DE8A1E] transition font-mono"
                  title="一次性口令，使用一次后自动失效"
                  @click="copyInviteCode(selectedBook.invite_code)"
                >
                  复制一次性口令：{{ selectedBook.invite_code }}
                </button>
                <button
                  type="button"
                  :disabled="regeneratingInviteCode"
                  class="text-xs font-semibold px-3 py-1.5 rounded-xl bg-[#F4F5F8] hover:bg-[#EAECEF] text-[#18181B] disabled:opacity-60 transition"
                  @click="handleRegenerateInviteCode"
                >
                  {{
                    regeneratingInviteCode
                      ? '生成中…'
                      : selectedBook.invite_code
                        ? '换新一次性口令'
                        : '生成一次性口令'
                  }}
                </button>
              </template>
            </div>
          </div>

          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div>
              <label class="block text-[11px] font-semibold text-[#64748B] mb-1">账本名称</label>
              <input
                v-model="editBookForm.name"
                type="text"
                required
                maxlength="20"
                class="w-full h-9 bg-[#F4F5F8] border border-[#EAECEF] rounded-xl px-3 outline-none focus:border-[#DE8A1E] focus:bg-white"
              />
            </div>
            <div>
              <label class="block text-[11px] font-semibold text-[#64748B] mb-1">月度预算（元）</label>
              <input
                v-model="editBookForm.budgetYuan"
                type="number"
                min="0"
                step="100"
                class="w-full h-9 font-bold tabular-nums bg-[#F4F5F8] border border-[#EAECEF] rounded-xl px-3 outline-none focus:border-[#DE8A1E] focus:bg-white"
              />
            </div>
          </div>

          <div>
            <label class="block text-[11px] font-semibold text-[#64748B] mb-1">账本描述</label>
            <input
              v-model="editBookForm.description"
              type="text"
              maxlength="60"
              placeholder="如：日常衣食住行 · 共同分摊"
              class="w-full h-9 bg-[#F4F5F8] border border-[#EAECEF] rounded-xl px-3 outline-none focus:border-[#DE8A1E] focus:bg-white"
            />
          </div>

          <div class="flex flex-wrap items-center justify-between gap-3 pt-1">
            <div class="flex items-center gap-2">
              <span class="text-[11px] font-semibold text-[#64748B]">账本色标</span>
              <div class="flex items-center gap-1.5">
                <button
                  v-for="c in LEDGER_COLOR_PRESETS"
                  :key="c"
                  type="button"
                  class="w-6 h-6 rounded-full transition"
                  :class="editBookForm.color === c ? 'ring-2 ring-offset-2 ring-[#18181B]' : ''"
                  :style="{ backgroundColor: c }"
                  @click="editBookForm.color = c"
                />
              </div>
            </div>

            <div class="flex items-center gap-2">
              <template v-if="isOwnerOfSelected && store.books.length > 1">
                <button
                  v-if="confirmDeleteBook"
                  type="button"
                  class="px-3 py-2 rounded-xl bg-[#F4F5F8] text-[#64748B] font-semibold"
                  @click="confirmDeleteBook = false"
                >
                  取消删除
                </button>
                <button
                  type="button"
                  :disabled="deletingBook"
                  class="px-3.5 py-2 rounded-xl font-semibold transition"
                  :class="
                    confirmDeleteBook
                      ? 'bg-[#D6553E] text-white'
                      : 'bg-[#FDF0EE] text-[#D6553E] hover:bg-[#FADCD7]'
                  "
                  @click="handleDeleteBook"
                >
                  {{ deletingBook ? '删除中…' : confirmDeleteBook ? '确认永久删除' : '删除账本' }}
                </button>
              </template>
              <button
                type="submit"
                :disabled="savingEditBook"
                class="btn-amber px-4 py-2 rounded-xl text-white font-semibold disabled:opacity-60"
              >
                {{ savingEditBook ? '保存中…' : '保存设置' }}
              </button>
            </div>
          </div>
        </form>

        <!-- 成员列表与邀请 -->
        <div class="pc-card p-5 space-y-4">
          <div class="flex items-center justify-between border-b border-[#F1F3F6] pb-3">
            <div>
              <h3 class="text-sm font-bold text-[#18181B]">协作成员管理</h3>
              <p class="text-[11px] text-[#64748B]">支持邮箱直邀、专属口令以及添加虚拟分摊成员</p>
            </div>
            <span class="text-xs font-semibold text-[#64748B] tabular-nums">
              {{ members.length }} 名成员
            </span>
          </div>

          <div v-if="loadingMembers" class="py-6 text-center text-xs text-[#94A3B8]">
            正在加载成员列表…
          </div>
          <div v-else class="divide-y divide-[#F1F3F6]">
            <div
              v-for="m in members"
              :key="m.user_id"
              class="py-3 flex items-center justify-between gap-2"
            >
              <div class="flex items-center gap-3 min-w-0">
                <img
                  :src="resolveAvatarUrl(m.avatar)"
                  :alt="m.user_name"
                  class="w-9 h-9 rounded-full object-cover border border-[#EAECEF] bg-white shrink-0"
                />
                <div class="min-w-0">
                  <div class="flex items-center gap-1.5">
                    <p class="text-xs font-bold text-[#18181B] truncate">{{ m.user_name }}</p>
                    <span
                      v-if="m.is_me"
                      class="text-[10px] px-1.5 py-0.5 rounded bg-[#F4F5F8] text-[#64748B]"
                    >
                      我
                    </span>
                    <span
                      v-if="m.is_virtual"
                      class="text-[10px] px-1.5 py-0.5 rounded bg-[#FFF6E8] text-[#DE8A1E]"
                    >
                      虚拟成员
                    </span>
                  </div>
                  <p class="text-[10px] text-[#94A3B8]">
                    {{ getMemberRoleLabel(m.role) }}
                    {{ m.status === 'invited' ? '· 待加入' : '' }}
                  </p>
                </div>
              </div>

              <button
                v-if="isOwnerOfSelected && !m.is_me && m.role !== 'owner'"
                type="button"
                class="text-[11px] font-semibold text-[#94A3B8] hover:text-[#D6553E] shrink-0"
                @click="handleRemoveMember(m)"
              >
                移除
              </button>
            </div>
          </div>

          <!-- 邮箱邀请 -->
          <form
            class="p-4 rounded-2xl bg-[#FAFBFD] border border-[#EAECEF] space-y-2.5 text-xs"
            @submit.prevent="handleInviteByEmail"
          >
            <p class="font-bold text-[#18181B]">发送邮箱邀请</p>
            <div class="flex flex-wrap gap-2">
              <input
                v-model="inviteEmail"
                type="email"
                required
                placeholder="输入对方邮箱发送账本邀请…"
                class="flex-1 min-w-[180px] h-9 bg-white border border-[#E6E9EF] rounded-xl px-3.5 outline-none focus:border-[#DE8A1E]"
              />
              <CustomDropdown
                v-model="inviteRole"
                :options="ROLE_OPTIONS"
                width-class="w-32"
              />
              <button
                type="submit"
                :disabled="invitingEmail"
                class="h-9 px-4 rounded-xl bg-[#DE8A1E] hover:bg-[#C67612] text-white font-semibold shrink-0 disabled:opacity-60 transition"
              >
                {{ invitingEmail ? '发送中…' : '发送邀请' }}
              </button>
            </div>
          </form>

          <!-- 创建虚拟成员 -->
          <form
            class="p-4 rounded-2xl bg-[#FAFBFD] border border-[#EAECEF] space-y-2.5 text-xs"
            @submit.prevent="handleCreateVirtualMember"
          >
            <p class="font-bold text-[#18181B]">添加虚拟分摊成员（无需对方注册）</p>
            <div class="flex flex-wrap items-center gap-2">
              <input
                v-model="virtualName"
                type="text"
                required
                maxlength="16"
                placeholder="输入成员昵称，如：家里老人、室友"
                class="flex-1 min-w-[180px] h-9 bg-white border border-[#E6E9EF] rounded-xl px-3.5 outline-none focus:border-[#DE8A1E]"
              />
              <div class="flex items-center gap-1">
                <button
                  v-for="c in LEDGER_COLOR_PRESETS.slice(0, 5)"
                  :key="c"
                  type="button"
                  class="w-6 h-6 rounded-full"
                  :class="virtualColor === c ? 'ring-2 ring-offset-1 ring-[#18181B]' : ''"
                  :style="{ backgroundColor: c }"
                  @click="virtualColor = c"
                />
              </div>
              <button
                type="submit"
                :disabled="creatingVirtual"
                class="h-9 px-4 rounded-xl bg-[#18181B] hover:bg-[#27272A] text-white font-semibold shrink-0 disabled:opacity-60 transition"
              >
                {{ creatingVirtual ? '添加中…' : '添加虚拟成员' }}
              </button>
            </div>
          </form>
        </div>
      </template>

      <div v-else class="pc-card py-16 text-center text-xs text-[#94A3B8]">
        请在左侧选择或创建一个账本
      </div>
    </section>
  </div>
</template>
