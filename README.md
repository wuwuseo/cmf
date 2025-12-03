# CMF Core

CMF (Core) 是一个基于 Go 语言开发的模块化 Web 应用框架核心，提供了配置管理、缓存、数据库、文件系统、JWT 认证、RBAC 权限控制等核心功能。旨在提供可扩展且易于维护的基础设施。

## 技术栈 (Tech Stack)

- **语言**: Go 1.25+
- **Web 框架**: Fiber v2 (`github.com/gofiber/fiber/v2`)
- **配置管理**: Viper (`github.com/spf13/viper`), godotenv (`github.com/joho/godotenv`)
- **日志**: Zap (`go.uber.org/zap`), Fiber Zap Middleware
- **数据库**: `database/sql` (支持 MySQL, PostgreSQL, SQLite)
- **缓存**: BigCache (`github.com/allegro/bigcache`), GoCache (`github.com/eko/gocache`), Redis (`github.com/redis/go-redis`)
- **认证**: JWT (`github.com/golang-jwt/jwt/v5`, `github.com/gofiber/contrib/jwt`)
- **授权**: Casbin (`github.com/casbin/casbin/v2`) 用于 RBAC
- **存储**: 抽象文件系统 (本地, S3 via `github.com/gofiber/storage/s3`)
- **验证**: Go Playground Validator (`github.com/go-playground/validator/v10`)
- **文档**: Swagger (`github.com/gofiber/swagger`)

## 项目规范 (Project Conventions)

### 代码风格
- **语言**: Go
- **格式化**: 标准 `gofmt`
- **命名**: 变量、函数、类名使用英文
- **注释**: **必须使用中文**
- **日志**: 推荐使用 Zap 结构化日志

### 架构模式
- **模块化设计**: 功能按包分离 (如 `cache`, `config`, `http`, `jwt`)
- **依赖注入**: 使用 `Bootstrap` 结构体和服务注册模式 (`RegisterService`, `GetService`, `MustGetServiceTyped`) 管理依赖
- **中间件**: 大量使用 Fiber 中间件处理横切关注点 (日志, 恢复, 认证等)
- **单例服务**: 核心服务 (Config, Cache, Filesystem) 在启动时注册为单例

### 测试策略
- 使用标准 Go 测试框架 (`testing` package)
- 针对各模块进行单元测试

### Git 工作流
- 标准 Git 工作流

## 领域上下文 (Domain Context)
- **CMF**: 内容管理框架 (Content Management Framework)
- **RBAC**: 基于角色的访问控制 (使用 Casbin 模型)
- **多租户/域**: Casbin 实现支持多域/多租户

## 重要约束
- **注释**: 所有注释和文档应使用中文
- **性能**: 高性能目标 (使用 Fiber, BigCache)

## 外部依赖
- **Redis**: 分布式缓存和 Redis 存储需要
- **数据库**: 持久化需要 MySQL, PostgreSQL 或 SQLite
- **S3 兼容存储**: 可选的文件存储

## 目录结构

```
cmf/
├── bootstrap/          # 应用启动引导
├── cache/              # 缓存模块
├── casbin/             # RBAC 权限控制
├── config/             # 配置管理
├── crypto/             # 加密工具
├── filesystem/         # 文件系统模块
├── http/               # HTTP 相关
├── jwt/                # JWT 认证
├── log/                # 日志模块
├── orm/                # ORM 模块
├── redis/              # Redis 客户端
├── storage/            # 本地存储实现
├── validate/           # 数据验证
├── README.md           # 项目说明
├── go.mod              # Go 模块定义
└── go.sum              # Go 模块校验和
```

## 配置

CMF 框架支持多种配置格式，包括 `.env` 文件和 YAML 格式。请参考 `config/` 目录下的相关代码了解详细配置项。
