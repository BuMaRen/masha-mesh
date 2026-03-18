# 📋 项目信息

这个目录包含项目的概览、资源清单和总体信息。

## 📖 内容指南

### 项目文件清单
**文件**: [`项目清单.md`](项目清单.md)  
**用时**: 10 分钟  
**内容**: 项目的完整文件结构、组织方式、文件说明

### 项目总结
**文件**: [`SUMMARY.md`](SUMMARY.md)  
**用时**: 15 分钟  
**内容**: 项目功能、特性、架构概览、技术栈

---

## 📁 项目结构概览

```
masha-mesh/
├── cmd/                  # 应用程序源码
│   ├── cli/             # CLI 工具
│   └── ctrl/            # 控制器
├── pkg/                 # 公共包
│   ├── api/             # API 定义
│   ├── cli/             # CLI 实现
│   └── ctrl/            # 控制器实现
├── build/               # 构建相关
├── docs/                # 文档（本目录）
├── deploy.sh            # 自动化部署脚本
├── Dockerfile           # Docker 镜像定义
├── k8s-webhook.yaml     # Kubernetes 配置
└── go.mod               # Go 模块定义
```

---

## ⚡ 快速查找

### 我想了解项目包含哪些文件
→ [项目清单.md](项目清单.md)

### 我想了解项目能做什么
→ [SUMMARY.md](SUMMARY.md)

### 我想了解项目的架构
→ [SUMMARY.md](SUMMARY.md) 的"架构部分"

### 我想了解技术栈
→ [SUMMARY.md](SUMMARY.md) 的"技术栈"

---

## 📊 项目信息速览

| 项目 | 说明 |
|------|------|
| 名称 | masha-mesh |
| 类型 | Kubernetes Service Mesh (Webhook Sidecar Injector) |
| 语言 | Go |
| Go 版本 | 1.26 |
| 部署目标 | Kubernetes (Minikube) |
| 主要功能 | 自动注入 Sidecar 代理 |

---

## 🎯 按需求查找

### 我需要项目的文件清单
→ [项目清单.md](项目清单.md) 
- 包含所有文件的详细说明
- 组织结构
- 各模块功能

### 我需要了解项目的功能和特性
→ [SUMMARY.md](SUMMARY.md)
- 功能列表
- 架构说明
- 技术栈
- 设计思路

### 我想知道哪些文件是我需要关注的
→ [项目清单.md](项目清单.md) 的"重要文件"部分

### 我想了解项目的打包部署方式
→ [SUMMARY.md](SUMMARY.md) 的"部署"部分

---

## 🔗 相关链接

- [快速开始](../getting-started/) - 项目使用指南
- [部署指南](../deployment/) - 部署流程
- [调试报告](../debugging/) - 问题解决
- [返回导航](../DOCS_INDEX.md) - 返回文档索引

---

## 📌 关键文件

| 文件 | 用途 | 重要性 |
|------|------|--------|
| deploy.sh | 自动化部署脚本 | ⭐⭐⭐ |
| Dockerfile | Docker 镜像定义 | ⭐⭐⭐ |
| k8s-webhook.yaml | Kubernetes 配置 | ⭐⭐⭐ |
| main.go | 应用入口 | ⭐⭐ |
| go.mod | 依赖管理 | ⭐⭐ |

---

**最后更新**: 2026-03-19

