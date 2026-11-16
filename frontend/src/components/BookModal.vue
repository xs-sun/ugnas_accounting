<script setup lang="ts">
import { ref, watch } from 'vue';
import type { Book, BookMember } from '../types';
import { api } from '../api/client';

const props = defineProps<{
  show: boolean;
  book?: Book | null;
}>();

const emit = defineEmits<{
  (e: 'close'): void;
  (e: 'saved'): void;
}>();

const name = ref('');
const currency = ref('CNY');
const description = ref('');
const members = ref<BookMember[]>([]);
const newMemberId = ref('');
const newMemberName = ref('');
const newMemberRole = ref<'editor' | 'viewer'>('editor');
const submitting = ref(false);
const errorMessage = ref<string | null>(null);

const loadMembers = async () => {
  if (props.book?.id) {
    try {
      members.value = await api.listMembers(props.book.id);
    } catch {
      members.value = [];
    }
  }
};

watch(
  () => props.show,
  (val) => {
    if (val) {
      errorMessage.value = null;
      if (props.book) {
        name.value = props.book.name;
        currency.value = props.book.currency || 'CNY';
        description.value = props.book.description || '';
        loadMembers();
      } else {
        name.value = '';
        currency.value = 'CNY';
        description.value = '';
        members.value = [];
      }
    }
  },
  { immediate: true }
);

const handleSaveBook = async () => {
  errorMessage.value = null;
  const nameTrim = name.value.trim();
  if (!nameTrim) {
    errorMessage.value = '账本名称不能为空';
    return;
  }

  submitting.value = true;
  try {
    if (props.book?.id) {
      await api.updateBook(props.book.id, {
        name: nameTrim,
        currency: currency.value,
        description: description.value,
      });
    } else {
      await api.createBook({
        name: nameTrim,
        currency: currency.value,
        description: description.value,
      });
    }
    emit('saved');
    emit('close');
  } catch (err: any) {
    errorMessage.value = err.message || '保存失败';
  } finally {
    submitting.value = false;
  }
};

const handleAddMember = async () => {
  if (!props.book?.id || !newMemberId.value.trim()) return;
  try {
    await api.addMember(props.book.id, {
      user_id: newMemberId.value.trim(),
      user_name: newMemberName.value.trim() || '协作者',
      role: newMemberRole.value,
    });
    newMemberId.value = '';
    newMemberName.value = '';
    await loadMembers();
  } catch (err: any) {
    errorMessage.value = err.message || '邀请成员失败';
  }
};

const handleRemoveMember = async (userId: string) => {
  if (!props.book?.id) return;
  try {
    await api.removeMember(props.book.id, userId);
    await loadMembers();
  } catch (err: any) {
    errorMessage.value = err.message || '移除成员失败';
  }
};
</script>

<template>
  <div
    v-if="show"
    class="fixed inset-0 z-50 flex items-end sm:items-center justify-center p-0 sm:p-4 bg-black/30 backdrop-blur-[2px] transition-opacity"
  >
    <div
      class="w-full sm:max-w-md bg-[#FFFFFF] rounded-t-[20px] sm:rounded-[20px] shadow-[0_1px_2px_rgba(23,26,32,0.03),0_10px_30px_-4px_rgba(23,26,32,0.05)] border border-[#EDEEF1] flex flex-col overflow-hidden"
    >
      <!-- Header -->
      <div class="px-6 py-4 flex items-center justify-between border-b border-[#EDEEF1]">
        <h3 class="text-base font-semibold text-[#171A20]">
          {{ book ? '编辑账本' : '新建账本' }}
        </h3>
        <button
          type="button"
          class="w-8 h-8 rounded-full flex items-center justify-center text-[#9AA0AB] hover:text-[#171A20] hover:bg-[#F6F7F8] cursor-pointer"
          @click="$emit('close')"
        >
          ✕
        </button>
      </div>

      <!-- Form Body -->
      <div class="p-6 space-y-4 max-h-[80vh] overflow-y-auto">
        <div class="space-y-1">
          <label class="text-xs font-medium text-[#5D6470]">账本名称</label>
          <input
            v-model="name"
            type="text"
            placeholder="例如: 日常生活账本、家庭共用账本..."
            class="w-full h-10 px-3 text-xs bg-white rounded-[11px] border border-[#EDEEF1] text-[#171A20] outline-hidden focus:border-[#0E8C7F]"
          />
        </div>

        <div class="space-y-1">
          <label class="text-xs font-medium text-[#5D6470]">货币符号</label>
          <select
            v-model="currency"
            class="w-full h-10 px-3 text-xs bg-white rounded-[11px] border border-[#EDEEF1] text-[#171A20] outline-hidden focus:border-[#0E8C7F]"
          >
            <option value="CNY">人民币 (CNY ¥)</option>
            <option value="USD">美元 (USD $)</option>
            <option value="EUR">欧元 (EUR €)</option>
            <option value="HKD">港币 (HKD HK$)</option>
            <option value="JPY">日元 (JPY ¥)</option>
          </select>
        </div>

        <div class="space-y-1">
          <label class="text-xs font-medium text-[#5D6470]">备注描述</label>
          <input
            v-model="description"
            type="text"
            placeholder="简短描述账本用途..."
            class="w-full h-10 px-3 text-xs bg-white rounded-[11px] border border-[#EDEEF1] text-[#171A20] outline-hidden focus:border-[#0E8C7F]"
          />
        </div>

        <!-- Members Management (when editing existing book) -->
        <div v-if="book?.id" class="space-y-2 pt-2 border-t border-[#EDEEF1]">
          <div class="flex items-center justify-between">
            <span class="text-xs font-medium text-[#171A20]">账本协作成员</span>
            <span class="text-[11px] text-[#9AA0AB]">{{ members.length }} 人</span>
          </div>

          <!-- Existing Members List -->
          <div class="space-y-1.5 max-h-36 overflow-y-auto">
            <div
              v-for="m in members"
              :key="m.user_id"
              class="flex items-center justify-between p-2 rounded-[9px] bg-[#F6F7F8] text-xs"
            >
              <div>
                <span class="font-medium text-[#171A20]">{{ m.user_name || m.user_id }}</span>
                <span class="text-[10px] ml-1.5 px-1.5 py-0.5 rounded-full"
                  :class="
                    m.role === 'owner'
                      ? 'bg-[#E4F3F1] text-[#0E8C7F]'
                      : m.role === 'editor'
                      ? 'bg-[#FDF2E9] text-[#B36B39]'
                      : 'bg-gray-200 text-gray-600'
                  "
                >
                  {{ m.role === 'owner' ? '所有者' : m.role === 'editor' ? '编辑者' : '只读' }}
                </span>
              </div>
              <button
                v-if="m.role !== 'owner'"
                type="button"
                class="text-[11px] text-[#E5654E] hover:underline cursor-pointer"
                @click="handleRemoveMember(m.user_id)"
              >
                移除
              </button>
            </div>
          </div>

          <!-- Invite New Member Input -->
          <div class="flex items-center space-x-2 pt-2">
            <input
              v-model="newMemberId"
              type="text"
              placeholder="绿联用户ID"
              class="flex-1 h-8 px-2 text-xs bg-white rounded-[8px] border border-[#EDEEF1] outline-hidden focus:border-[#0E8C7F]"
            />
            <select
              v-model="newMemberRole"
              class="h-8 px-2 text-xs bg-white rounded-[8px] border border-[#EDEEF1] text-[#5D6470] outline-hidden"
            >
              <option value="editor">编辑者</option>
              <option value="viewer">只读者</option>
            </select>
            <button
              type="button"
              class="h-8 px-3 rounded-[8px] bg-[#0E8C7F] text-white text-xs font-medium hover:bg-[#0B6E64] cursor-pointer"
              @click="handleAddMember"
            >
              邀请
            </button>
          </div>
        </div>

        <div v-if="errorMessage" class="text-xs text-[#E5654E] bg-[#FDF1EE] p-2.5 rounded-[11px]">
          {{ errorMessage }}
        </div>
      </div>

      <!-- Footer Action -->
      <div class="p-6 pt-3 border-t border-[#EDEEF1]">
        <button
          type="button"
          class="w-full h-11 bg-[#0E8C7F] hover:bg-[#0B6E64] active:scale-[0.98] text-white font-medium text-xs rounded-full transition-all duration-150 flex items-center justify-center space-x-2 cursor-pointer disabled:opacity-50"
          :disabled="submitting"
          @click="handleSaveBook"
        >
          <span v-if="submitting">保存中...</span>
          <span v-else>保存账本</span>
        </button>
      </div>
    </div>
  </div>
</template>
