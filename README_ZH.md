# 概述
GoSuv是GO语言重写的类supervisor的一个进程管理程序，简单易用，界面美感十足且对用户友好 

## 使用
* 启动服务

    ```
    gosuv start-server
    ```

查看服务状态

    ```
    $ gosuv status
    Server is running
    ```

默认端口 11113  本机测试请使用[http://localhost:11313](http://localhost:11313)

![RunImage](docs/gosuv.gif)

## 配置
默认配置文件都放在 `$HOME/.gosuv/`
    
* 项目文件名 ：     programs.yml
* 服务器配置文件名：    config.yml

验证信息配置

```yml
server:
    httpauth:
    enabled: false
    username: uu
    password: pp
  addr: :11313
client:
  server_url: http://localhost:11313
```

## 默认日志文件位置
`$HOME/.gosuv/log/`

## 待续
内容不是很多，还是推荐能看懂英语的去看[英文的README](README.md)

## 贡献人
- [Docking](http://miaomia.com)