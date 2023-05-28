## SimpleServerStatus

一款`极简探针` 云探针、多服务器探针

### 本地构建

* 前端

```
npm run build:prod
```

* 后端

因为需要内嵌web页面，所以需要把前端`dist`目录下的文件复制到`dashboard/public/dist`目录下面

```
goreleaser release --snapshot --clean
```

### 运行

#### agent

```shell
nohup ./sssa -s ws://127.0.0.1:8900/ws-report -i test-server -a 123456 > sssa.log 2>&1 &
```

* `-s` 服务器地址
* `-i` 服务器id
* `-a` 授权密钥

agnet的参数可以使用配置`sss-agent.yaml`，也可以命令行直接指定；
以上参数必须跟服务端的`sss-dashboard.yaml`里面配置的对应

#### dashboard

参照[sss-dashboard.yaml.example](sss-dashboard.yaml.example) 配置好`sss-dashboard.yaml` 直接运行即可

```shell
nohup ./sssd > sssd.log 2>&1 &
```

### 停止

```shell
ps -ef | grep sssa # dashboard: sssd；agent: sssa
```

查询到pid后直接kill即可

### 反代

**nginx**参照下面配置：

以下配置中的端口（8900）和websocket路径请应`sss-dashboard.yaml`中的配置

```
upstream sssd {
  server 127.0.0.1:8900;
}
# map 指令根据客户端请求头中 $http_upgrade 的值构建 $connection_upgrade 的值；如果 $http_upgrade 没有匹配，默认值为 upgrade，如果 $http_upgrade 配置空字符串，值为 close
map $http_upgrade $connection_upgrade {
    default upgrade;
    '' close;
}
```

server块中配置：

```
location / {
    proxy_set_header HOST $host;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_pass http://sssd;
}
 #代理websocket，这里的path请参考sss-dashboard.yaml 中的webSocketPath 
location /ws-report {
    # 代理转发目标
    proxy_pass http://sssd;

    # 请求服务器升级协议为 WebSocket
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection $connection_upgrade;

    # 设置读写超时时间，默认 60s 无数据连接将会断开
    proxy_read_timeout 300s;
    proxy_send_timeout 300s;

    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Host $host:$server_port;
    proxy_set_header X-Forwarded-Server $host;
    proxy_set_header X-Forwarded-Port $server_port;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```


