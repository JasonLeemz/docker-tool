# Docker Tool 管理脚本

这个目录包含了Docker Tool的管理脚本，用于简化程序的构建、启动和停止操作。

## 脚本说明

### build.sh - 构建脚本
用于编译docker-tool程序。

**功能：**
- 检查Go环境
- 清理旧的构建文件
- 下载Go模块依赖
- 编译程序
- 显示构建结果

**使用方法：**
```bash
./bin/build.sh
```

### start.sh - 启动脚本
用于启动docker-tool程序。

**功能：**
- 检查可执行文件和配置文件
- 检查程序是否已在运行
- 创建必要的目录
- 后台启动程序
- 保存进程ID到PID文件

**使用方法：**
```bash
./bin/start.sh
```

### stop.sh - 停止脚本
用于停止docker-tool程序。

**功能：**
- 读取PID文件
- 优雅停止程序（发送TERM信号）
- 如果程序无响应，强制停止（发送KILL信号）
- 清理PID文件
- 显示最近的日志

**使用方法：**
```bash
./bin/stop.sh
```

## 日志系统

程序启动后会自动创建日志文件：

- **日志目录：** `logs/`
- **日志文件格式：** `docker-tool-YYYY-MM-DD.log`
- **日志内容：** 同时输出到控制台和文件
- **日志格式：** 包含时间戳和文件名行号

## 使用流程

### 1. 首次使用
```bash
# 构建程序
./bin/build.sh

# 启动程序
./bin/start.sh
```

### 2. 日常使用
```bash
# 启动程序
./bin/start.sh

# 查看日志
tail -f logs/docker-tool-$(date +%Y-%m-%d).log

# 停止程序
./bin/stop.sh
```

### 3. 更新程序
```bash
# 停止程序
./bin/stop.sh

# 重新构建
./bin/build.sh

# 启动程序
./bin/start.sh
```

## 文件结构

```
docker-tool/
├── docker-tool          # 可执行文件
├── config.yaml          # 配置文件
├── docker-tool.pid      # PID文件（运行时生成）
├── logs/                # 日志目录
│   └── docker-tool-2025-09-18.log
└── bin/                 # 管理脚本目录
    ├── build.sh
    ├── start.sh
    ├── stop.sh
    └── README.md
```

## 注意事项

1. **权限：** 确保脚本有执行权限
2. **配置文件：** 确保 `config.yaml` 存在且配置正确
3. **Docker：** 确保Docker daemon正在运行
4. **日志轮转：** 日志文件按日期自动创建，建议定期清理旧日志
5. **PID文件：** 程序停止时会自动清理PID文件

## 故障排除

### 程序无法启动
1. 检查配置文件是否存在
2. 检查Docker daemon是否运行
3. 查看日志文件中的错误信息

### 程序无法停止
1. 检查PID文件是否存在
2. 手动杀死进程：`kill -9 <PID>`
3. 删除PID文件：`rm docker-tool.pid`

### 日志文件过大
1. 定期清理旧日志文件
2. 考虑使用日志轮转工具
3. 调整日志级别（需要修改代码）
