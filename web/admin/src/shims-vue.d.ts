/* eslint-disable */
/// <reference types="vite/client" />

declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<{}, {}, any>
  export default component
}

declare module '@element-plus/icons-vue' {
  import type { DefineComponent } from 'vue'
  type IconComponent = DefineComponent<{}, {}, any>
  export const Connection: IconComponent
  export const Timer: IconComponent
  export const Upload: IconComponent
  export const Download: IconComponent
  export const Refresh: IconComponent
  export const RefreshRight: IconComponent
  export const Search: IconComponent
  export const CopyDocument: IconComponent
  export const Delete: IconComponent
  export const Plus: IconComponent
  export const Edit: IconComponent
  export const View: IconComponent
  export const Check: IconComponent
  export const Lock: IconComponent
  export const Message: IconComponent
  export const Position: IconComponent
  export const ChatDotRound: IconComponent
  export const Bell: IconComponent
  export const HomeFilled: IconComponent
  export const DataAnalysis: IconComponent
  export const Share: IconComponent
  export const List: IconComponent
  export const Document: IconComponent
  export const UserFilled: IconComponent
  export const Setting: IconComponent
  export const SwitchButton: IconComponent
  export const Menu: IconComponent
  export const InfoFilled: IconComponent
  export const User: IconComponent
  export const Moon: IconComponent
  export const Sunny: IconComponent
}

declare module 'element-plus' {
  export const ElMessage: any
  export const ElMessageBox: any
  export const ElNotification: any
  const ElementPlus: any
  export default ElementPlus
}

declare module 'vue-echarts' {
  import type { DefineComponent } from 'vue'
  const VChart: DefineComponent<{}, {}, any>
  export default VChart
}

declare module 'echarts/core' {
  export const use: (...args: any[]) => void
}

declare module 'echarts/renderers' {
  export const CanvasRenderer: any
  export const SVGRenderer: any
}

declare module 'echarts/charts' {
  export const GraphChart: any
  export const LineChart: any
  export const BarChart: any
  export const PieChart: any
}

declare module 'echarts/components' {
  export const TooltipComponent: any
  export const LegendComponent: any
  export const GridComponent: any
  export const TitleComponent: any
}
