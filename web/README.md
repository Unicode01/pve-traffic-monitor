# PVE Traffic Monitor - Web Frontend

现代化的 Web 前端界面，基于 Vue 3 + Element Plus 构建。

## 功能特性

- ✅ 现代化 UI 设计
- ✅ 响应式布局
- ✅ 明暗主题切换
- ✅ 中英文国际化
- ✅ ECharts 图表可视化
- ✅ 实时数据刷新

## 开发

```bash
# 安装依赖
npm install

# 开发模式（支持热重载）
npm run dev

# 构建生产版本
npm run build

# 预览构建结果
npm run preview
```

## 构建说明

构建后的文件会输出到 `dist/` 目录，后端API服务器会自动检测并使用构建后的文件。

如果 `dist/` 目录不存在，API服务器会退回到使用内嵌的简化HTML版本。

## 配色方案

前后端使用统一的配色方案（定义在 `pkg/models/colors.go`）：

**亮色主题：**
- 下载：#36a2eb（蓝色）
- 上传：#4bc0c0（青色）
- 总计：#ff6384（红色）

**暗色主题：**
- 下载：#5cb3ff（亮蓝色）
- 上传：#5cd3d3（亮青色）
- 总计：#ff8fa3（亮红色）

## 技术栈

- Vue 3 - 渐进式JavaScript框架
- Element Plus - Vue 3 组件库
- ECharts - 强大的图表库
- Vue Router - 路由管理
- Pinia - 状态管理
- Vue I18n - 国际化
- Vite - 下一代前端构建工具
