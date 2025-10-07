# useraccount

useraccount 套件處理使用者的預定義執行流程。

## 註冊

- 使用者使用 Google 帳號或其他 OAuth 方式登入。
- 確認是否已經有這個使用者，如果沒有，建立一個 `unverified` 群組的使用者。
- 可以在後台手動授予使用者更明確的群組（如 `student`）。
- 如果使用者放棄驗證，則直接走登出流程。
- Cronjob 會在每個小時定期清除 unverified users。
