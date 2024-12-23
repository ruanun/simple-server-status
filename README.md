## SimpleServerStatus

一款`极简探针` 云探针、多服务器探针。基于Golang + Vue实现。

演示地址：[https://sssd.ions.top/](https://sssd.ions.top/)

### 部署

到`Releases`按照平台下载对应文件，并解压缩

#### agent

```shell
mkdir /etc/sssa/
cp sssa /etc/sssa/sssa
chmod +x /etc/sssa/sssa
cp sss-agent.yaml.example /etc/sssa/sss-agent.yaml
#修改 /etc/sssa/sss-agent.yaml里面的相关配置参数。

cp sssa.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable sssa
#启动
systemctl start sssa
```
其他命令（停止、启动、查看状态、查看日志）
```shell
#停止
systemctl stop sssa
#查看状态
systemctl status sssa
#查看日志
journalctl -f -u sssa
```

agnet的参数可以使用配置`sss-agent.yaml`，也可以命令行直接指定。 参数必须跟dashboard的`sss-dashboard.yaml`里面配置的对应

#### dashboard

参照[dashboard/sss-dashboard.yaml.example](dashboard/sss-dashboard.yaml.example) 配置好`sss-dashboard.yaml`

docker部署

```shell
docker run --name sssd  --restart=unless-stopped -d -v ./sss-dashboard.yaml:/app/sss-dashboard.yaml -p 8900:8900 ruanun/sssd
```

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

### 本地构建

- 前端

```shell
npm run build:prod
```

- 后端

    - 构建dashboard

      因为需要内嵌web页面，所以需要把前端`dist`目录下的文件复制到`dashboard/public/dist`目录下面
    
      ```shell
      cd dashboard && goreleaser release --snapshot --clean
      ```

    - 构建anent

      ```shell
      cd agent && goreleaser release --snapshot --clean
      ```

构建完成，查看dist目录下的文件
