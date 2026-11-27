import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// A interface fica em 127.0.0.1:5180 e repassa /api para a API em 4000. O
// Omni observa a porta 4000: é por ela que passa todo o tráfego da loja.
export default defineConfig({
  plugins: [react()],
  server: {
    host: '127.0.0.1',
    port: 5180,
    strictPort: true,
    proxy: { '/api': 'http://127.0.0.1:4000' },
  },
})
