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
- `store.Get`
  - 如果请求没有cookie头或者cookie头无效(比如登陆/注册)，则此方法会返回一个新的"空的"session
  - 如果是有效的cookie头，此方法会返回这个请求的session(每个请求都有独立的session)
    - 方法会从cookie头里解出_id, 然后用_id拿到db里存的data, 放到这个session的Values里
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
  - 设置`session.Values`、调用`store.Save`(Save会写cookie头)
  - 处理业务逻辑
