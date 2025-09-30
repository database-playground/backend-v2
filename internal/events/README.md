# Events

負責觸發事件和加減點數的 service。

## 事件表

### 憑證管理

- `login`：登入帳號
- `impersonated`：管理員嘗試取得登入憑證
- `logout`：登出帳號
- `logout_all`：撤銷這個使用者的所有登入憑證

### 作答管理

- `submit_answer`：提交答案
