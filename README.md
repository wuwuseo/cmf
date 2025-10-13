# CMF Framework

CMF (Custom Modular Framework) 是一个基于 Go 语言开发的模块化 Web 应用框架，提供了配置管理、缓存、数据库、文件系统、JWT 认证、RBAC 权限控制等核心功能。

## 特性

- **模块化设计**：采用模块化架构，易于扩展和维护
- **配置管理**：支持多种配置源（.env 文件、环境变量等）
- **多存储支持**：支持本地存储、S3 等多种文件存储方式
- **缓存系统**：支持 Redis 和内存缓存
- **数据库支持**：支持多种数据库（MySQL、PostgreSQL、SQLite 等）
- **JWT 认证**：内置 JWT 认证机制
- **RBAC 权限控制**：基于 Casbin 的角色访问控制
- **日志系统**：集成 Zap 日志库，支持文件和控制台输出

## 目录结构

```
cmf/
├── bootstrap/          # 应用启动引导
├── cache/              # 缓存模块
├── casbin/             # RBAC 权限控制
├── config/             # 配置管理
├── data/               # 数据目录
│   ├── logs/           # 日志文件
│   └── storage/        # 本地存储文件
├── docs/               # 文档
├── examples/           # 示例代码
├── filesystem/         # 文件系统模块
├── http/               # HTTP 相关
├── jwt/                # JWT 认证
├── log/                # 日志模块
├── orm/                # ORM 模块
├── redis/              # Redis 客户端
├── storage/            # 本地存储实现
└── validate/           # 数据验证
```

## 安装


## 配置

CMF 框架支持多种配置格式，包括 `.env` 文件和 YAML 格式。默认配置项如下：

### ENV 格式

```env
# 应用配置
APP_NAME=app
APP_DEBUG=false
APP_PORT=3000
APP_IDLE_TIMEOUT=60
APP_PREFORK=false
APP_SWAGGER=false
APP_SECRET=secret

# 日志配置
LOG_FILE_PATH=./data/logs/app.log
LOG_CONSOLE_OUTPUT=true
LOG_FILE_OUTPUT=true
LOG_MAX_SIZE=10
LOG_MAX_BACKUPS=10
LOG_MAX_AGE=180

# 数据库配置
DATABASE_DEFAULT=default
DATABASE_CONNECTIONS_DEFAULT_DRIVER=mysql
DATABASE_CONNECTIONS_DEFAULT_HOST=localhost
DATABASE_CONNECTIONS_DEFAULT_PORT=3306
DATABASE_CONNECTIONS_DEFAULT_USER=root
DATABASE_CONNECTIONS_DEFAULT_PASSWORD=123456
DATABASE_CONNECTIONS_DEFAULT_NAME=cmf
DATABASE_CONNECTIONS_DEFAULT_SSL_MODE=false
DATABASE_CONNECTIONS_DEFAULT_TABLE_PREFIX=cmf_

# 缓存配置
CACHE_DEFAULT=memory
CACHE_STORES_MEMORY_DRIVER=memory
CACHE_STORES_MEMORY_DEFAULT_TTL=3600
CACHE_STORES_REDIS_DRIVER=redis
CACHE_STORES_REDIS_DEFAULT_TTL=3600

# Redis 配置
REDIS_DEFAULT=redis
REDIS_CONNECTIONS_REDIS_ADDR=localhost:6379
REDIS_CONNECTIONS_REDIS_USERNAME=
REDIS_CONNECTIONS_REDIS_PASSWORD=
REDIS_CONNECTIONS_REDIS_DB=0
REDIS_CONNECTIONS_REDIS_DIAL_TIMEOUT=5
REDIS_CONNECTIONS_REDIS_READ_TIMEOUT=3
REDIS_CONNECTIONS_REDIS_WRITE_TIMEOUT=3
REDIS_CONNECTIONS_REDIS_POOL_SIZE=10
REDIS_CONNECTIONS_REDIS_MIN_IDLE_CONNS=5
REDIS_CONNECTIONS_REDIS_MAX_IDLE_CONNS=10
REDIS_CONNECTIONS_REDIS_CONN_MAX_IDLE_TIME=30
REDIS_CONNECTIONS_REDIS_CONN_MAX_LIFETIME=24
REDIS_CONNECTIONS_REDIS_USE_TLS=false

# 文件系统配置
FILESYSTEM_DEFAULT=local
FILESYSTEM_IS_AND_LOCAL=false
FILESYSTEM_DISKS_LOCAL_DRIVER=local
FILESYSTEM_DISKS_LOCAL_OPTIONS_ROOT=./data/storage
FILESYSTEM_DISKS_S3_DRIVER=s3
FILESYSTEM_DISKS_S3_OPTIONS_ACCESS_KEY=
FILESYSTEM_DISKS_S3_OPTIONS_SECRET_KEY=
FILESYSTEM_DISKS_S3_OPTIONS_REGION=
FILESYSTEM_DISKS_S3_OPTIONS_BUCKET=
FILESYSTEM_DISKS_S3_OPTIONS_ENDPOINT=
```

### YAML 格式

```yaml
app:
  name: app
  port: 3000
  debug: false
  idle_timeout: 60
  prefork: false
  swagger: false
  secret: secret

log:
  file_path: ./data/logs/app.log
  console_output: true
  file_output: true
  max_size: 10
  max_backups: 10
  max_age: 180

database:
  default: default
  connections:
    default:
      driver: mysql
      host: localhost
      port: 3306
      user: root
      password: 123456
      name: cmf
      ssl_mode: false
      table_prefix: cmf_

cache:
  default: memory
  stores:
    memory:
      driver: memory
      default_ttl: 3600
    redis:
      driver: redis
      default_ttl: 3600

redis:
  default: redis
  connections:
    redis:
      addr: localhost:6379
      username: ""
      password: ""
      db: 0
      dial_timeout: 5
      read_timeout: 3
      write_timeout: 3
      pool_size: 10
      min_idle_conns: 5
      max_idle_conns: 10
      conn_max_idle_time: 30
      conn_max_lifetime: 24
      use_tls: false

filesystem:
  default: local
  is_and_local: false
  disks:
    local:
      driver: local
      options:
        root: ./data/storage
    s3:
      driver: s3
      options:
        access_key: ""
        secret_key: ""
        region: ""
        bucket: ""
        endpoint: ""
```

## 核心模块

### 文件系统

支持多种存储驱动，包括本地存储和 S3。可以通过配置 `FILESYSTEM_IS_AND_LOCAL=true` 实现数据双重存储。

```go
// 创建文件系统实例
fs, err := filesystem.NewFilesystemFromConfig(cfg)
if err != nil {
    log.Fatal(err)
}

// 存储文件
err = fs.Set("key", []byte("Hello, World!"), 0)
```

### 缓存系统

支持 Redis 和内存缓存两种驱动。

```go
// 创建缓存实例
cache := cache.NewCache(context.Background(), cfg)

// 设置缓存
err := cache.Set(context.Background(), "key", []byte("value"))
if err != nil {
    log.Fatal(err)
}

// 获取缓存
value, err := cache.Get(context.Background(), "key")
```


### JWT 认证

提供 JWT Token 的生成和验证功能。

```go
// 生成 Token
token, err := jwt.GenerateToken(userID, username)
if err != nil {
    log.Fatal(err)
}

// 验证 Token
claims, err := jwt.ParseToken(token)
```

### RBAC 权限控制

基于 Casbin 实现的角色访问控制。

```go
// 检查权限
allowed, err := casbin.Enforce(sub, obj, act)
if err != nil {
    log.Fatal(err)
}
```

## 使用示例

请查看 `examples/` 目录下的示例代码了解如何使用 CMF 框架的各种功能。

## 许可证

MIT License