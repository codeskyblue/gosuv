
##概述
GoSuv是GO语言重写的类supervisor的一个进程管理程序，简单易用，界面美感十足且对用户友好 
##中文文档编写者
[Docking](http://miaomia.com)

##安装GoSuv
####在你使用前请确认已经安装好GO 并配置好
    go get -d github.com/codeskyblue/gosuv
    cd $GOPATH/src/github.com/codeskyblue/gosuv
    go build

##使用GoSuv
###启动
    gosuv start-server
###显示服务状态
    $ gosuv status
    Server is running
### 默认端口：11113  本机测试请使用[http://localhost:11313](http://localhost:11313)
![RunImage](https://github.com/codeskyblue/gosuv/blob/master/docs/gosuv.gif)  
###配置
####默认配置文件位置    $HOME/.gosuv/
    * 项目文件 ：     programs.yml
    * 服务器配置文件：    config.yml
####验证信息配置
    server:
        httpauth:
        enabled: false
        username: uu
        password: pp
      addr: :11313
    client:
      server_url: http://localhost:11313
####默认日志文件位置$HOME/.gosuv/log/
##待续
