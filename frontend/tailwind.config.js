/** @type {import('tailwindcss').Config} */
export default {
  content: [
    './index.html',
    './src/**/*.{vue,js,ts,jsx,tsx}',
  ],
  theme: {
    extend: {
      colors: {
        // 画布与卡片
        canvas: '#F6F7F8',
        surface: '#FFFFFF',

        // 品牌主色 (深松石绿 - 严格限用于 + 按钮、保存按钮、收入、激活态)
        brand: {
          DEFAULT: '#0E8C7F',
          dark: '#0B6E64',
          tint: '#E4F3F1',
        },

        // 支出强调色 (陶土橘 - 仅用于支出金额与图表图形)
        expense: {
          DEFAULT: '#E5654E',
          tint: '#FDF1EE',
        },

        // 墨色文字色阶
        text: {
          ink: '#171A20',
          muted: '#5D6470',
          subtle: '#9AA0AB',
        },

        // 边框与分割线
        border: {
          DEFAULT: '#EDEEF1',
        },

        // 莫兰迪低饱和色系矩阵 (背景色与描边色)
        morandi: {
          food: { bg: '#F9EFE9', stroke: '#9E6748' },
          shop: { bg: '#EAF0F6', stroke: '#4C6E8D' },
          transport: { bg: '#E8F4F2', stroke: '#2E7E73' },
          housing: { bg: '#F2EFE9', stroke: '#7D6F56' },
          entertainment: { bg: '#F4EEF7', stroke: '#7E5B8E' },
          tech: { bg: '#E9EDF5', stroke: '#506385' },
          health: { bg: '#FAECEB', stroke: '#A85854' },
          edu: { bg: '#EBF5F0', stroke: '#447C5E' },
          cat: { bg: '#FDF2E9', stroke: '#B36B39' },
          social: { bg: '#F7EBEF', stroke: '#945369' },
          salary: { bg: '#E4F3F1', stroke: '#0E8C7F' },
          invest: { bg: '#FEF4E8', stroke: '#B87528' },
          bonus: { bg: '#EAF2E8', stroke: '#4D7D48' },
          transfer: { bg: '#EEF2F6', stroke: '#5A6B82' },
          other: { bg: '#F0F1F3', stroke: '#666F7D' },
        },
      },
      borderRadius: {
        card: '20px',       // 主卡片 squircle
        secondary: '14px',  // 次级卡片 / 模态框
        small: '11px',      // 分类块 / 小徽章
      },
      boxShadow: {
        card: '0 1px 2px rgba(23, 26, 32, 0.03), 0 10px 30px -4px rgba(23, 26, 32, 0.05)',
        'card-hover': '0 2px 6px rgba(23, 26, 32, 0.04), 0 14px 36px -4px rgba(23, 26, 32, 0.07)',
      },
      fontFamily: {
        sans: [
          '"PingFang SC"',
          '-apple-system',
          'BlinkMacSystemFont',
          '"Hiragino Sans GB"',
          '"Microsoft YaHei"',
          'sans-serif',
        ],
      },
      letterSpacing: {
        tightest: '-0.02em',
      },
    },
  },
  plugins: [],
};
