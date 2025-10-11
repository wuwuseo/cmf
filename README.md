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

# 数据库配置
DATABASE_DEFAULT=default
DATABASE_CONNECTIONS_DEFAULT_DRIVER=mysql
DATABASE_CONNECTIONS_DEFAULT_HOST=localhost
DATABASE_CONNECTIONS_DEFAULT_PORT=3306
DATABASE_CONNECTIONS_DEFAULT_USER=root
DATABASE_CONNECTIONS_DEFAULT_PASSWORD=123456
DATABASE_CONNECTIONS_DEFAULT_NAME=cmf

# Redis 配置
REDIS_DEFAULT=redis
REDIS_CONNECTIONS_REDIS_ADDR=localhost:6379

# 文件系统配置
FILESYSTEM_DEFAULT=local
FILESYSTEM_IS_AND_LOCAL=false
```

### YAML 格式

```yaml
app:
  name: app
  debug: false
  port: 3000

database:
  default: default
  connections:
    default:
      driver: mysql
      host: localhost
      port: 3306
      user: root
      password: "123456"
      name: cmf

redis:
  default: redis
  connections:
    redis:
      addr: localhost:6379

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

### 缓存存储切换示例

请查看 [examples/cache_store_example.go](examples/cache_store_example.go) 了解如何使用新的 `Store` 方法在不同的缓存存储之间切换。

`Store` 方法利用 `sync.Map` 来缓存已创建的存储实例，确保多次切换到同一存储时返回相同的实例，避免重复创建。这提高了性能并保持了一致性。

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