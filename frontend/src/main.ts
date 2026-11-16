import { createApp } from 'vue';
import './style.css';
import App from './App.vue';
import UGOSCore from '@ugreen-nas/core';

try {
  UGOSCore.init();
} catch (err) {
  console.warn('[UGOS] Core init warning:', err);
}

createApp(App).mount('#app');
