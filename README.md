# masha-mesh
for service mesh learning

## 快速部署

使用 `deploy.sh` 脚本可以一键完成编译、推送镜像和部署的全部流程：

```bash
# 自动递增版本号并部署
./deploy.sh

# 指定版本号部署
./deploy.sh v0.1.30
# 或者
./deploy.sh 0.1.30

# 先预览将要执行的步骤（推荐）
./deploy.sh --dry-run
./deploy.sh --dry-run v0.1.30
```

该脚本会自动完成以下步骤：
1. 在根目录执行 `make push VERSION=vX.X.XX` 编译并推送镜像到DockerHub
2. 自动更新 `build/ctrl/deployment.yml` 和 `build/cli/deployment.yml` 中的镜像版本
3. 在 build 目录执行 `make all` 部署到本地Kubernetes集群

查看帮助信息：
```bash
./deploy.sh --help
```

## 手动部署

如果需要手动执行各个步骤：

1. 编译并推送镜像：
```bash
make push VERSION=v0.1.xx
```

2. 修改 build 目录中的 deployment.yml 文件，更新镜像版本号

3. 部署到Kubernetes：
```bash
cd build
make all
```
