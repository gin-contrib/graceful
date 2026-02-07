# graceful

[English](README.md) | [繁體中文](README.zh-tw.md) | 简体中文

![Run Tests](https://github.com/gin-contrib/graceful/actions/workflows/go.yml/badge.svg?branch=master)
[![Trivy Security Scan](https://github.com/gin-contrib/graceful/actions/workflows/trivy-scan.yml/badge.svg)](https://github.com/gin-contrib/graceful/actions/workflows/trivy-scan.yml)
![codecov](https://codecov.io/gh/gin-contrib/graceful/branch/master/graph/badge.svg)
![Go Report Card](https://goreportcard.com/badge/github.com/gin-contrib/graceful)
![GoDoc](https://godoc.org/github.com/gin-contrib/graceful?status.svg)

**graceful** 是 Gin 的一个封装器，为 HTTP 服务器提供强大且灵活的优雅关闭能力。它允许你启动、停止及平滑关闭服务器，并支持多种监听机制，包括 TCP、Unix socket、文件描述符或自定义 listener。

- [graceful](#graceful)
  - [特性](#特性)
  - [安装方法](#安装方法)
  - [使用示例](#使用示例)
  - [API 概览](#api-概览)
    - [Graceful 类型](#graceful-类型)
    - [服务器启动方法](#服务器启动方法)
    - [关闭与清理方法](#关闭与清理方法)
    - [选项](#选项)
  - [许可证](#许可证)

## 特性

- 为 Gin HTTP 服务器无缝实现优雅关闭功能
- 支持 TCP、TLS、Unix socket、文件描述符和自定义 net.Listener
- 线程安全的启动/停止流程和基于 context 的取消
- 保证所有活动连接都被妥善处理后再终止
- 简洁 API 并支持自定义选项

## 安装方法

```bash
go get github.com/gin-contrib/graceful
```

## 使用示例

```go
package main

import (
  "context"
  "net/http"
  "os/signal"
  "syscall"

  "github.com/gin-contrib/graceful"
  "github.com/gin-gonic/gin"
)

func main() {
  ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
  defer stop()

  router, err := graceful.Default()
  if err != nil {
    panic(err)
  }
  defer router.Close()

  router.GET("/", func(c *gin.Context) {
    c.String(http.StatusOK, "Welcome Gin Server")
  })

  if err := router.RunWithContext(ctx); err != nil && err != context.Canceled {
    panic(err)
  }
}
```

## API 概览

### Graceful 类型

主要类型为：

```go
type Graceful struct {
  *gin.Engine
  // 内部同步/context 字段
}
```

### 服务器启动方法

- **Default(opts ...Option) \*Graceful, error**：  
  创建一个带默认 Gin 中间件的 `Graceful` 实例。

- **New(router \*gin.Engine, opts ...Option) \*Graceful, error**：  
  封装已有 `gin.Engine`。

- **Run(addr ...string) error**：  
  在一个或多个 TCP 地址启动 HTTP 服务器。

- **RunTLS(addr, certFile, keyFile string) error**：  
  在指定地址启动 HTTPS 服务器。

- **RunUnix(file string) error**：  
  在指定 Unix socket 启动服务器。

- **RunFd(fd uintptr) error**：  
  使用给定文件描述符启动服务器。

- **RunListener(listener net.Listener) error**：  
  在自定义的 `net.Listener` 启动服务器。

- **RunWithContext(ctx context.Context) error**：  
  在绑定 context 生命周期下启动服务器。推荐配合信号处理关闭使用。

### 关闭与清理方法

- **Shutdown(ctx context.Context) error**：  
  优雅关闭服务器，并等待所有活动连接断开。

- **Start() error**：  
  在 goroutine 中启动服务器，需自行调用 `Stop()` 终止。

- **Stop() error**：  
  停止由 `Start()` 启动的 Graceful 实例。

- **Close()**：  
  清理内部状态并关闭正在运行的服务器。

### 选项

多种选项可配置服务器：

- **WithAddr(addr string)**：
  配置 HTTP 服务器监听指定 TCP 地址。

- **WithTLS(addr, certFile, keyFile string)**：
  配置 HTTPS 服务器与 TLS 证书。

- **WithUnix(file string)**：
  配置服务器监听 Unix socket。

- **WithFd(fd uintptr)**：
  配置服务器监听文件描述符。

- **WithListener(listener net.Listener)**：
  配置服务器使用自定义 listener。

- **WithServer(srv \*http.Server)**：
  使用现有 `http.Server` 进行完整自定义。

- **WithShutdownTimeout(timeout time.Duration)**：
  配置优雅关闭超时时间（默认：30 秒）。

- **WithServerTimeouts(readTimeout, writeTimeout, idleTimeout time.Duration)**：
  配置 HTTP 服务器超时时间：
  - `ReadTimeout`：完整请求读取超时（包含请求体，默认：15 秒）
  - `WriteTimeout`：响应写入超时（默认：30 秒）
  - `IdleTimeout`：Keep-Alive 空闲连接超时（默认：60 秒）

自定义超时设置示例：

```go
router, err := graceful.Default(
  graceful.WithShutdownTimeout(10 * time.Second),
  graceful.WithServerTimeouts(10*time.Second, 15*time.Second, 30*time.Second),
)
```

---

## 许可证

本项目基于 [MIT License](LICENSE) 授权。
