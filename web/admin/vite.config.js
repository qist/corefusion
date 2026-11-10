import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// 面板可能部署在隐藏子路径（webPath）下，而这个路径是**服务端运行时配置**，
// 构建时并不知道。Vite 默认把懒加载 chunk / 预加载 CSS 拼成根绝对地址
// （"/assets/OnlineUsers-xxx.css"）→ 子路径部署时会被 404（真机现象：
// online-users 页 CSS 以 text/plain 被拒、chunk 404）。
//
// 这里改成运行时表达式：window.__CF_ASSET_BASE__（服务端注入，根部署为空）
// + 构建时文件名。两种部署（根 / 隐藏路径）都能解析到正确位置。
function runtimeAssetUrl(filename) {
  return { runtime: `((window.__CF_ASSET_BASE__||"")+"/"+${JSON.stringify(filename)})` }
}

export default defineConfig({
  plugins: [vue()],
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
  },
  experimental: {
    // hostType 说明该 URL 出现在哪种文件里：
    //   js   → chunk/预加载资源，必须带运行时 base（见上）；
    //   css  → url() 引用，保持相对 CSS 文件（CSS 已在 <webPath>/assets/ 下）；
    //   html → index.html 的入口引用，由服务端按 webPath 改写（rewriteIndexAssetRefs）。
    renderBuiltUrl(filename, { hostType }) {
      if (hostType === 'js') return runtimeAssetUrl(filename)
      if (hostType === 'css') return { relative: true }
      return undefined
    },
  },
})
