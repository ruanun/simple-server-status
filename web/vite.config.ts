import {defineConfig} from 'vite'
import vue from '@vitejs/plugin-vue'
import {fileURLToPath, URL} from 'node:url'
import Components from 'unplugin-vue-components/vite';
import {AntDesignVueResolver} from 'unplugin-vue-components/resolvers';

export default defineConfig({
    build: {
        //分隔多个，防止单文件过大
        rollupOptions: {
            output:{
                manualChunks(id) {
                    if (id.includes('node_modules')) {
                        return id.toString().split('node_modules/')[1].split('/')[0].toString();
                    }
                }
            }
        }
    },
    plugins: [
        vue(),
        Components({
            resolvers: [
                AntDesignVueResolver({
                    importStyle: false, // css in js
                }),
            ],
        }),
    ],
    resolve: {
        // alias: {
        //   '@': path.resolve(__dirname, 'src'),
        // }
        alias: {
            '@': fileURLToPath(new URL('./src', import.meta.url))
        }
    },
    server: {
        proxy: {
            "/api": {
                target: "http://127.0.0.1:8900",
                changeOrigin: true,  //允许跨域
                ws: true,  // 开启 websockets 代理
                secure: false, // 验证 SSL 证书
                rewrite: (path) => path,
            },
            "/ws-frontend": {
                target: "http://127.0.0.1:8900",
                changeOrigin: true,  //允许跨域
                ws: true,  // 开启 websockets 代理
                secure: false, // 验证 SSL 证书
            },
        }
    },
})
