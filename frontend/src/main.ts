import { createApp } from 'vue';
import './style.css';
import App from './App.vue';
import { initUGOSCoreIfEmbedded } from '@/api/client';

void initUGOSCoreIfEmbedded();

createApp(App).mount('#app');
