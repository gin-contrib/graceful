# graceful

[English](README.md) | 繁體中文 | [简体中文](README.zh-cn.md)

![Run Tests](https://github.com/gin-contrib/graceful/actions/workflows/go.yml/badge.svg?branch=master)
[![Trivy Security Scan](https://github.com/gin-contrib/graceful/actions/workflows/trivy-scan.yml/badge.svg)](https://github.com/gin-contrib/graceful/actions/workflows/trivy-scan.yml)
![codecov](https://codecov.io/gh/gin-contrib/graceful/branch/master/graph/badge.svg)
![Go Report Card](https://goreportcard.com/badge/github.com/gin-contrib/graceful)
![GoDoc](https://godoc.org/github.com/gin-contrib/graceful?status.svg)

**graceful** 是 Gin 的一個包裝器，為 HTTP 伺服器提供強大且靈活的優雅關閉能力。它允許你啟動、停止及平滑地關閉伺服器，並支援各種監聽方式，包括 TCP、Unix socket、檔案描述符或自訂 listener。

- [graceful](#graceful)
  - [特色](#特色)
  - [安裝方式](#安裝方式)
  - [使用範例](#使用範例)
  - [API 概覽](#api-概覽)
    - [Graceful 型別](#graceful-型別)
    - [伺服器啟動方法](#伺服器啟動方法)
    - [關閉與清理方法](#關閉與清理方法)
    - [選項](#選項)
  - [授權條款](#授權條款)

## 特色

- 為 Gin HTTP 伺服器提供無縫的優雅關閉功能
- 支援 TCP、TLS、Unix socket、檔案描述符和自訂 net.Listener
- 執行緒安全的啟動/停止程序與基於 context 的取消控制
- 確保所有活動連線都被妥善處理後再終止
- 提供簡潔 API 與可自訂選項

## 安裝方式

```bash
go get github.com/gin-contrib/graceful
```

## 使用範例

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

## API 概覽

### Graceful 型別

主要型別為：

```go
type Graceful struct {
  *gin.Engine
  // 內部同步與 context 欄位
}
```

### 伺服器啟動方法

- **Default(opts ...Option) \*Graceful, error**：  
  建立具預設 Gin middleware 的 `Graceful` 實例。

- **New(router *gin.Engine, opts ...Option) \*Graceful, error**：  
  包裝現有 `gin.Engine`。

- **Run(addr ...string) error**：  
  於一個或多個 TCP 位址啟動 HTTP 伺服器。

- **RunTLS(addr, certFile, keyFile string) error**：  
  於指定位址啟動 HTTPS 伺服器。

- **RunUnix(file string) error**：  
  於指定 Unix socket 啟動伺服器。

- **RunFd(fd uintptr) error**：  
  利用給定檔案描述符啟動伺服器。

- **RunListener(listener net.Listener) error**：  
  於自訂的 `net.Listener` 啟動伺服器。

- **RunWithContext(ctx context.Context) error**：  
  於 context 綁定的生命週期中啟動伺服器。建議信號處理時使用。

### 關閉與清理方法

- **Shutdown(ctx context.Context) error**：  
  優雅地關閉伺服器，並等候所有活動連線終止。

- **Start() error**：  
  於 goroutine 啟動伺服器，你必須呼叫 `Stop()` 來終止。

- **Stop() error**：  
  停止先前以 `Start()` 啟動的 Graceful 實例。

- **Close()**：  
  清理內部狀態並關閉所有執行中伺服器。

### 選項

各種選項（參見程式碼 `Option` 介面 / 實作）允許設定伺服器，包括自訂位址、TLS、Unix socket、檔案描述符與 listener。

---

## 授權條款

本專案依 [MIT License](LICENSE) 授權。
