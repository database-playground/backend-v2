# httputils

`httputils` 提供一些用來傳給 resolver 的 HTTP 特有參數，如 User-Agent。

## Machine Name

gqlgen 預設不會帶入 HTTP 的 `User-Agent`，但我們有在請求情境鏈 (context) 中注入 Gin 取得的 User-Agent，並將其稱之為 Machine Name。

你可以使用 `GetMachineName` 取回這個 Machine Name，並用於如 token 簽發的場合。
