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
  - [生命週期 Hooks 範例](#生命週期-hooks-範例)
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

- **New(router \*gin.Engine, opts ...Option) \*Graceful, error**：  
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

各種選項允許設定伺服器：

- **WithAddr(addr string)**：
  設定 HTTP 伺服器監聽指定 TCP 位址。

- **WithTLS(addr, certFile, keyFile string)**：
  設定 HTTPS 伺服器與 TLS 憑證。

- **WithUnix(file string)**：
  設定伺服器監聽 Unix socket。

- **WithFd(fd uintptr)**：
  設定伺服器監聽檔案描述符。

- **WithListener(listener net.Listener)**：
  設定伺服器使用自訂 listener。

- **WithServer(srv \*http.Server)**：
  使用現有 `http.Server` 進行完整自訂。

- **WithShutdownTimeout(timeout time.Duration)**：
  設定優雅關閉逾時時間（預設：30 秒）。

- **WithServerTimeouts(readTimeout, writeTimeout, idleTimeout time.Duration)**：
  設定 HTTP 伺服器逾時時間：
  - `ReadTimeout`：完整請求讀取逾時（包含請求主體，預設：15 秒）
  - `WriteTimeout`：回應寫入逾時（預設：30 秒）
  - `IdleTimeout`：Keep-Alive 閒置連線逾時（預設：60 秒）

- **WithBeforeShutdown(hook Hook)**：
  註冊一個在伺服器開始關閉前呼叫的 hook。可註冊多個 hooks，將按註冊順序執行。Hooks 會接收到 shutdown context，可回傳錯誤，這些錯誤會被收集但不會阻止關閉流程繼續執行。

- **WithAfterShutdown(hook Hook)**：
  註冊一個在所有伺服器關閉後呼叫的 hook。可註冊多個 hooks，將按註冊順序執行。Hooks 會接收到 shutdown context，可回傳錯誤，這些錯誤會被收集並回傳，但不影響伺服器關閉流程。

自訂逾時設定範例：

```go
router, err := graceful.Default(
  graceful.WithShutdownTimeout(10 * time.Second),
  graceful.WithServerTimeouts(10*time.Second, 15*time.Second, 30*time.Second),
)
```

## 生命週期 Hooks 範例

生命週期 hooks 允許你在優雅關閉過程中執行自訂邏輯。使用 `WithBeforeShutdown` 進行應在伺服器停止前的清理工作（如從服務發現中註銷），使用 `WithAfterShutdown` 進行伺服器停止後的清理工作（如關閉資料庫連線）。

```go
package main

import (
  "context"
  "log"
  "net/http"
  "os/signal"
  "syscall"

  "github.com/gin-contrib/graceful"
  "github.com/gin-gonic/gin"
)

func main() {
  ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
  defer stop()

  router, err := graceful.Default(
    // BeforeShutdown：在伺服器開始關閉前呼叫
    graceful.WithBeforeShutdown(func(ctx context.Context) error {
      log.Println("通知負載平衡器...")
      // 從負載平衡器註銷
      return nil
    }),
    graceful.WithBeforeShutdown(func(ctx context.Context) error {
      log.Println("停止背景工作程序...")
      // 停止接收新的背景任務
      return nil
    }),

    // AfterShutdown：在所有伺服器關閉後呼叫
    graceful.WithAfterShutdown(func(ctx context.Context) error {
      log.Println("關閉資料庫連線...")
      // 關閉資料庫連線池
      return nil
    }),
    graceful.WithAfterShutdown(func(ctx context.Context) error {
      log.Println("推送指標資料...")
      // 傳送最終指標
      return nil
    }),
  )
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

當觸發關閉時，hooks 會按以下順序執行：

1. 所有 `BeforeShutdown` hooks（按註冊順序）
2. 伺服器優雅關閉
3. 所有 `AfterShutdown` hooks（按註冊順序）

完整可執行範例請參閱 [hooks 範例](_examples/hooks)。

---

## 授權條款

本專案依 [MIT License](LICENSE) 授權。
