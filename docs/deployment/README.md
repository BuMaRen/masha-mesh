# 🔧 部署与运维

这个目录包含所有与部署、验证和维护系统相关的文档。

## 📖 使用指南

### 想快速查看部署要点？
**文件**: [`README_DEPLOYMENT.md`](README_DEPLOYMENT.md) ⭐  
**用时**: 5 分钟  
**内容**: 部署概览、常用命令、快速参考

### 想了解部署的完整细节？
**文件**: [`DEPLOYMENT_SUMMARY.md`](DEPLOYMENT_SUMMARY.md)  
**用时**: 15 分钟  
**内容**: 工作总结、修改内容、后续建议

### 想验证部署是否成功？
**文件**: [`DEPLOYMENT_SUCCESS.md`](DEPLOYMENT_SUCCESS.md) ⭐  
**用时**: 10 分钟  
**内容**: 成功标志、验证清单、故障排查

### 想查看完整的部署日志？
**文件**: [`deployment.log`](deployment.log)  
**用时**: 按需查看  
**内容**: 完整的部署执行日志、时间戳、输出信息

---

## ⚡ 快速导航

### 快速部署
```bash
./deploy.sh deploy
```
然后查看 [DEPLOYMENT_SUCCESS.md](DEPLOYMENT_SUCCESS.md) 验证

### 遇到问题
→ 查看 [../debugging/DEBUG_REPORT.md](../debugging/DEBUG_REPORT.md)

### 查看详细日志
→ 查看 [deployment.log](deployment.log)

### 快速参考
→ 查看 [README_DEPLOYMENT.md](README_DEPLOYMENT.md)

---

## 📋 文件说明

| 文件 | 用途 | 阅读时长 |
|------|------|---------|
| README_DEPLOYMENT.md | 部署快速参考 | 5 分钟 |
| DEPLOYMENT_SUMMARY.md | 工作总结、学习资源 | 15 分钟 |
| DEPLOYMENT_SUCCESS.md | 验证清单、故障排查 | 10 分钟 |
| deployment.log | 完整部署日志 | 按需 |

---

## 🎯 按需求查找

### 我想快速查看部署状态
→ [README_DEPLOYMENT.md](README_DEPLOYMENT.md) 的"验证清单"部分

### 我想知道做了什么改动
→ [DEPLOYMENT_SUMMARY.md](DEPLOYMENT_SUMMARY.md) 的"修改文件列表"

### 我想验证系统是否正常工作
→ [DEPLOYMENT_SUCCESS.md](DEPLOYMENT_SUCCESS.md) 的"验证清单"

### 我想查看执行过程中的所有输出
→ [deployment.log](deployment.log)

### 我想学习系统架构和最佳实践
→ [DEPLOYMENT_SUMMARY.md](DEPLOYMENT_SUMMARY.md) 的"社区最佳实践"和"学习价值"

---

## 📊 部署速度参考

- 首次部署: ~60-90 秒
- 重新部署: ~30-40 秒（镜像已缓存）
- 清理 + 重来: ~5 分钟

---

## 🔗 相关链接

- [快速开始](../getting-started/) - 快速上手指南
- [调试报告](../debugging/) - 常见问题解决
- [项目信息](../project/) - 项目详情
- [返回导航](../DOCS_INDEX.md) - 返回文档索引

