# Docker Tool

一个基于Go开发的Docker容器监听工具，能够自动监听容器IP变化并更新nginx反向代理配置。

## 功能特性

- 🔄 **实时监听**：使用Docker Events API实时监听容器启动、停止、重命名事件
- 🌐 **自动发现**：自动发现新创建的容器并注册到nginx
- 📝 **配置管理**：基于YAML配置文件管理服务规则
- 🔀 **双协议支持**：支持HTTP和Stream两种nginx配置类型
- ⚡ **自动重载**：容器变化时自动更新nginx配置并重载
- 🎯 **智能匹配**：支持容器名称匹配和网络类型识别
- 🔥 **热重载**：支持配置文件热重载，无需重启程序

## 快速开始

### 1. 编译程序

```bash
go mod tidy
go build -o docker-tool
```

### 2. 配置文件

创建 `config.yaml` 配置文件：

```yaml
# 全局配置
global:
  nginx_config_dir: "/etc/nginx/conf.d"
  stream_config_dir: "/etc/nginx/stream.d"
  nginx_reload_cmd: "docker exec nginx-ui nginx -s reload"
  
  # 默认代理配置
  default_proxy:
    client_max_body_size: "2048M"
    proxy_http_version: "1.1"
    proxy_headers:
      - "Upgrade $http_upgrade"
      - "Connection upgrade"
      - "X-Real-IP $remote_addr"
      - "X-Forwarded-For $proxy_add_x_forwarded_for"
      - "X-Forwarded-Proto $scheme"
      - "X-Forwarded-Host $http_host"
    proxy_redirect: "off"

# 服务配置
services:
  # HTTP服务配置
  - name: "api-service"
    type: "http"
    container_name: "my-api-container"
    domain: "api.example.com"
    path: "/"
    port: 9000
    upstream_name: "api_backend"
    
  # Stream服务配置
  - name: "mysql-service"
    type: "stream"
    container_name: "mysql-db"
    listen_port: 3306
    container_port: 3306
    upstream_name: "mysql_backend"
```

### 3. 运行程序

```bash
# 前台运行
./docker-tool -config config.yaml

# 后台运行
./docker-tool -config config.yaml -daemon
```

## 配置说明

### 全局配置

- `nginx_config_dir`: HTTP配置文件目录
- `stream_config_dir`: Stream配置文件目录  
- `nginx_reload_cmd`: nginx重载命令
- `default_proxy`: 默认代理配置

### 服务配置

#### HTTP服务
- `name`: 服务名称
- `type`: 服务类型，固定为 "http"
- `container_name`: 容器名称
- `domain`: 域名
- `path`: 路径
- `port`: 容器内部端口
- `upstream_name`: 上游服务器组名称

#### Stream服务
- `name`: 服务名称
- `type`: 服务类型，固定为 "stream"
- `container_name`: 容器名称
- `listen_port`: nginx监听端口
- `container_port`: 容器内部端口
- `upstream_name`: 上游服务器组名称

## 工作原理

1. **事件监听**：程序启动后监听Docker容器的启动、停止、重命名事件
2. **配置监听**：每5秒检查一次配置文件是否发生变化
3. **容器匹配**：根据配置文件中的容器名称匹配需要代理的服务
4. **信息获取**：获取容器的IP地址和端口信息
5. **配置生成**：根据服务类型生成对应的nginx配置文件
6. **自动重载**：执行nginx重载命令使配置生效

## 配置文件热重载

程序支持配置文件热重载功能：

- **自动检测**：每5秒检查一次配置文件修改时间
- **自动重载**：检测到变化时自动重新加载配置
- **重新扫描**：重载配置后自动重新扫描所有现有容器
- **无需重启**：整个过程无需重启程序

### 使用场景

1. **添加新服务**：在配置文件中添加新的服务配置，程序会自动发现并注册现有容器
2. **修改配置**：修改现有服务的配置参数，程序会自动更新nginx配置
3. **删除服务**：删除配置文件中的服务，程序会自动清理对应的nginx配置

## 生成的nginx配置示例

### HTTP配置
```nginx
upstream api_backend {
    server 192.168.31.100:9000;
}

server {
    listen 80;
    server_name api.example.com;
    
    location / {
        client_max_body_size 2048M;
        proxy_pass          http://api_backend/;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-Host $http_host;
        proxy_redirect off;
    }
}
```

### Stream配置
```nginx
upstream mysql_backend {
    server 192.168.31.101:3306;
}

server {
    listen 3306;
    proxy_pass mysql_backend;
}
```

## 注意事项

1. 确保Docker daemon正在运行且程序有访问权限
2. nginx容器需要挂载配置文件目录
3. 程序需要执行nginx重载命令的权限
4. 建议在测试环境先验证配置正确性

## 故障排除

### 常见问题

1. **无法连接Docker daemon**
   - 检查Docker是否运行
   - 确认用户权限

2. **nginx重载失败**
   - 检查nginx容器是否运行
   - 验证重载命令是否正确

3. **配置文件生成失败**
   - 检查目录权限
   - 确认配置文件路径正确

## 开发

### 项目结构
```
docker-tool/
├── main.go                 # 主程序入口
├── config.yaml            # 配置文件示例
├── go.mod                 # Go模块文件
├── internal/
│   ├── config/            # 配置管理
│   ├── watcher/           # 容器监听
│   └── nginx/             # nginx配置管理
└── README.md              # 说明文档
```

### 依赖
- `github.com/docker/docker`: Docker API客户端
- `github.com/docker/go-connections`: Docker网络连接处理
- `gopkg.in/yaml.v3`: YAML配置文件解析
