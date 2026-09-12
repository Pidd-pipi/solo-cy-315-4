# 部署说明

生产部署使用根目录 `docker-compose.yml`，本目录保留同等编排文件作为部署参考。

- 后端服务名：`gbschedule`
- 内部端口：`8080`
- 宿主端口：`${BACKEND_PORT:-19515}`
- 数据卷：`gbschedule_data`
