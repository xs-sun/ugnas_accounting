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
        canvas: '#F5F6F8',
        'canvas-pc': '#F4F5F8',
        surface: '#FFFFFF',

        // 品牌主色 (暖琥珀金)
        brand: {
          DEFAULT: '#DE8A1E',
          600: '#EC9B2E',
          700: '#DE8A1E',
          dark: '#C67612',
          deep: '#B8680E',
          tint: '#FFF6E8',
          light: '#FFF7EB',
          warm: '#FFF3DC',
        },

        // 收入色 (鼠尾草绿)
        income: {
          DEFAULT: '#3E8E63',
          tint: '#ECF6F0',
        },

        // 支出强调色 (赤陶红)
        expense: {
          DEFAULT: '#D6553E',
          tint: '#FDF0EE',
        },
        terracotta: {
          DEFAULT: '#D6553E',
          tint: '#FDF0EE',
        },

        // 墨色文字色阶
        ink: {
          primary: '#18181B',
          secondary: '#64748B',
          muted: '#94A3B8',
        },

        // 边框与分割线
        border: {
          DEFAULT: '#ECEEF2',
          pc: '#EAECEF',
          warm: '#ECEEF2',
        },
      },
      borderRadius: {
        card: '22px',
        'pc-card': '20px',
        secondary: '14px',
        small: '11px',
      },
      boxShadow: {
        xs: '0 1px 2px rgba(15, 23, 42, 0.04)',
        card: '0 2px 10px rgba(28, 37, 32, 0.025)',
        'card-hover': '0 8px 24px -10px rgba(15, 23, 42, 0.08)',
      },
      ringWidth: {
        '1.5': '1.5px',
      },
      fontFamily: {
        sans: [
          '"Inter"',
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
