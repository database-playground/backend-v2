# Admin CLI

Admin CLI 提供用來執行初始化、晉升等網頁管理介面不適合直接操作的功能，使用 [`urfave/cli`](https://github.com/urfave/cli) 實作互動。

## 指令

- `migrate`：執行資料庫遷移
- `setup`：執行資料庫遷移和基礎結構的建立
- `promote-admin`：將一個使用者晉升為管理員

## 依賴

這個 CLI 依賴 Config 和 Ent。因此即使他不依賴 Redis、Google OAuth 等參數，您依然需要保持環境變數和 [backend](../backend) 一致。

## 方法撰寫

- 您應該在 [cli 目錄](../../cli) 定義每個 CLI 方法的實作。
- 然後在 [`commands.go`](./commands.go) 定義這個 CLI 方法的呼叫形式。
- 最後在 [`cli.go`](./cli.go) 註冊您撰寫的 CLI 方法。
