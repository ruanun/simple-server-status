import {fileURLToPath, URL} from 'node:url'

import {defineConfig} from 'vite'
import vue from '@vitejs/plugin-vue'
// @ts-ignore
import Components from 'unplugin-vue-components/vite';
// @ts-ignore
import {AntDesignVueResolver} from 'unplugin-vue-components/resolvers';

// https://vitejs.dev/config/
export default defineConfig({
    plugins: [
        vue(),
        Components({
            resolvers: [AntDesignVueResolver({ importStyle: "css" })],
        }),],
    resolve: {
        alias: {
            '@': fileURLToPath(new URL('./src', import.meta.url))
        }
    },
    server: {
        proxy: {
            "/api": {
                target: "http://localhost:8900",
                changeOrigin: true,  //允许跨域
                ws: true,  // 开启 websockets 代理
                secure: false, // 验证 SSL 证书
                rewrite: (path) => path,
            },
        }
    },
    build: {
        outDir: "dist", // 指定打包文件的输出目录
        emptyOutDir: true,  // 打包时先清空上一次构建生成的目录
        rollupOptions: {
            output: {
                manualChunks(id) {
                    if (id.includes('node_modules')) {
                        return id.toString().split('node_modules/')[1].split('/')[0].toString();
                    }
                }
            }
        }
    },
})
