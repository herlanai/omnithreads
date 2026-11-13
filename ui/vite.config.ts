import { defineConfig } from 'vite'
import { tanstackStart } from '@tanstack/react-start/plugin/vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
export default defineConfig({plugins:[tailwindcss(),tanstackStart({spa:{enabled:true,prerender:{outputPath:'/index.html'}}}),react()],server:{proxy:{'/media':{target:'http://127.0.0.1:7331',changeOrigin:true},'/omniweb':{target:'http://127.0.0.1:7331',changeOrigin:true}}}})
