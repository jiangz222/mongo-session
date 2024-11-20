# mongo-session

Implement MongoDB-session-store of [gorilla/sessions](https://github.com/gorilla/sessions). 
Replace the mgo in  [mongostore](https://github.com/kidstuff/mongostore) by [qmgo](https://github.com/qiniu/qmgo).

# example
```
    func foo(rw http.ResponseWriter, req *http.Request) {
        // Fetch new store.
        ctx := context.Background()
	    qc, err := qmgo.Open(ctx, &qmgo.Config{Uri: "mongodb://localhost:27017", Database: "test", Coll: "session"})
        if err != nil {
            panic(err)
        }

        store := mongostore.NewMongoStore(qc, 3600, true,
            []byte("secret-key"))

        // Get a session.
        session, err := store.Get(req, "session-key")
        if err != nil {
            log.Println(err.Error())
        }

        // Add a value.
        session.Values["foo"] = "bar"

        // Save.
        if err = sessions.Save(req, rw); err != nil {
            log.Printf("Error saving session: %v", err)
        }

        fmt.Fprintln(rw, "ok")
    }
```

# 原理
## 基本逻辑
- Mongo-DB 里会存储: _id, data, modified 3个字段
- 初次请求，会将登陆用户的定制信息存入db，具体的定制内容放到data
- 返回响应头为： Set-Cookie: session-key=fadsifadjsifasdiofasdfads
  - fadsifadjsifasdiofasdfads里包含了db里的_id的信息
- 下次请求头为: Cookie: session-key=fadsifadjsifasdiofasdfads, 
  - 从fadsifadjsifasdiofasdfads反解出_id, 然后从DB里得到data，完成验证和信息读取

## 方法意义
- `store.Get`
  - 如果没有cookie或者cookie无效，则此方法会返回一个新的session
  - 如果是有效的cookie，此方法会返回这个请求的session，每个请求都有独立的session，从cookie里解出_id, 然后用_id拿到db里存的data(`session.Values`)
- `sessions.Save`
  - 将新的`session.Values`写入到db里的data里
  - 将_id编码到cookie头里，返回

# 账户系统的使用姿势
一个正常的账户系统，应该如此使用
- 初始化 `store := mongostore.NewMongoStore(qc, 3600, true,
  []byte("secret-key"))`, 可以考虑全局使用
- 注册 或者 登陆请求
  - 先处理完各自的业务逻辑
  - 然后调用`store.Get`、设置`session.Values`、调用`store.Save`(Save会写cookie头)
- 登陆后，其他的业务请求
  - 需要先调用`store.Get`，成功调用且能得到之前设置的Values，说明session有效
  - 处理业务逻辑
  - 设置`session.Values`、调用`store.Save`(Save会写cookie头)