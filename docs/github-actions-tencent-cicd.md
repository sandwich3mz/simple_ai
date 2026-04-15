# GitHub Actions 到腾讯云自动部署（前后端双 Dockerfile，含失败自动回滚）

完成一次性配置后，你日常发布只需要：

```bash
git push origin main
```

## 1）服务器一次性初始化

登录腾讯云服务器后执行：

```bash
sudo useradd -m -s /bin/bash deploy || true
sudo mkdir -p /srv/simple_ai
sudo chown -R deploy:deploy /srv/simple_ai
sudo usermod -aG docker deploy
```

切换到 `deploy` 用户并拉代码：

```bash
sudo su - deploy
cd /srv
git clone <你的仓库SSH地址> simple_ai
cd /srv/simple_ai
cp config/config.toml.example config/config.toml
vi config/config.toml
```

说明：`config/config.toml` 已在 `.gitignore`，应只保留在服务器本地。

## 2）配置 GitHub Actions 连接服务器的 SSH 密钥

在你本地机器生成一对密钥：

```bash
ssh-keygen -t ed25519 -f deploy_key -C "github-actions-deploy"
```

把 `deploy_key.pub` 加到服务器 `deploy` 用户的 `authorized_keys`：

```bash
mkdir -p ~/.ssh && chmod 700 ~/.ssh
cat deploy_key.pub >> ~/.ssh/authorized_keys
chmod 600 ~/.ssh/authorized_keys
```

## 3）私有仓库场景：允许服务器拉取 GitHub 代码

在服务器（`deploy` 用户）再生成一对仅用于 `git pull` 的密钥：

```bash
ssh-keygen -t ed25519 -f ~/.ssh/github_pull -C "server-pull"
cat ~/.ssh/github_pull.pub
```

把公钥添加到 GitHub 仓库 Deploy Keys（只读即可），然后创建 `~/.ssh/config`：

```text
Host github.com
  HostName github.com
  User git
  IdentityFile ~/.ssh/github_pull
  IdentitiesOnly yes
```

测试：

```bash
ssh -T git@github.com
```

## 4）配置 GitHub Secrets

路径：`Settings -> Secrets and variables -> Actions -> Secrets`

- `SERVER_HOST`：服务器公网 IP
- `SERVER_USER`：`deploy`
- `SERVER_PORT`：`22`
- `SSH_PRIVATE_KEY`：本地 `deploy_key` 私钥完整内容

## 5）配置 GitHub Variables（可选，但推荐）

路径：`Settings -> Secrets and variables -> Actions -> Variables`

- `APP_DIR`：`/srv/simple_ai`
- `DEPLOY_BRANCH`：`main`
- `DEPLOY_NETWORK`：`simple_ai_net`
- `STARTUP_WAIT_SECONDS`：`8`（两个新容器启动后等待秒数）

后端相关：

- `BACKEND_DOCKERFILE`：`/srv/simple_ai/Dockerfile`
- `BACKEND_CONTEXT_DIR`：`/srv/simple_ai`
- `BACKEND_IMAGE_NAME`：`simple_ai_backend`
- `BACKEND_CONTAINER_NAME`：`backend`
- `BACKEND_HOST_PORT`：`9090`
- `BACKEND_CONTAINER_PORT`：`9090`
- `BACKEND_CONFIG_FILE`：`/srv/simple_ai/config/config.toml`
- `BACKEND_HEALTHCHECK_URL`：例如 `http://127.0.0.1:9090/health`（可留空）

前端相关：

- `FRONTEND_DOCKERFILE`：`/srv/simple_ai/vue-frontend/Dockerfile`
- `FRONTEND_CONTEXT_DIR`：`/srv/simple_ai/vue-frontend`
- `FRONTEND_IMAGE_NAME`：`simple_ai_frontend`
- `FRONTEND_CONTAINER_NAME`：`frontend`
- `FRONTEND_HOST_PORT`：`80`
- `FRONTEND_CONTAINER_PORT`：`80`
- `FRONTEND_API_UPSTREAM`：`backend:9090`（必须与后端容器名和端口匹配）
- `FRONTEND_HEALTHCHECK_URL`：例如 `http://127.0.0.1/`（可留空）

说明：如果变量未设置，部署脚本会使用默认值。默认情况下前端和后端会挂到同一个 Docker 网络里，前端通过容器名访问后端。

## 6）失败自动回滚机制

当前部署脚本已内置自动回滚，流程如下（前后端一起）：

1. 分别构建后端镜像和前端镜像（带时间戳 tag）
2. 将旧后端容器、旧前端容器重命名并停止，作为回滚快照
3. 先启动新后端，再启动新前端
4. 等待 `STARTUP_WAIT_SECONDS`，并可选执行前后端健康检查 URL
5. 任一步失败，自动删除新容器并恢复旧容器

## 7）仓库中的 CI/CD 文件

- `.github/workflows/deploy.yml`
- `scripts/deploy.sh`

整体链路：

1. 你 `git push origin main`
2. GitHub Actions 自动 SSH 到腾讯云
3. 服务器自动 `git pull`
4. 自动分别构建前后端镜像并发布两个容器
5. 失败则自动回滚上一版本（前后端一起回滚）
