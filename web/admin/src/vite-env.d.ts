/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly BASE_URL: string
  // 添加其他环境变量声明
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
