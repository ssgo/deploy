ssgo/deploy是一套简单易用的构建部署工具。

# 快速使用

首先宿主机上要安装docker。

然后运行：

```shell
docker run -d --restart=always --network=host --name deploy -v /opt/deploy:/opt/deploy -v /var/run/docker.sock:/var/run/docker.sock {deploy-image}
```

容器运行起来后，就可以使用deploy了，直接使用宿主机的docker资源。

docker run使用 -e 'deploy_manageToken=xxxx' 可以给deploy提供登录密码，如果没有设置默认密码为91deploy。

deploy的启动也可以在ssgo/hub中配置开启。

## 访问

hub访问通过：http://xx.xx.xx.xx:7777/

默认使用 8888 端口，可以使用 -p xxxx:7777 来改变端口。

或者启动容器指定 service_listen=":xxxx" 来改变端口。

## 容器网络模式

容器启动需要使用主机网络模式，使用宿主机网络和端口，通讯效率高。

## 存储依赖

数据会存储在 /opt/data 下，可以使用 -v /opt/hub:/opt/data 来挂载外部磁盘。

/var/run/docker.sock代表容器内部使用宿主机的docker。

# 自定义deploy镜像

镜像也可以自己根据ssgo/deploy的代码进行构建。

编译deploy：

```shell
mkdir -p dist
sed -i 's/__TAG__/$TAG/g' www/index.html www/views/Deploy.html
go mod tidy
go build -ldflags -w -o dist/server *.go
cp *.yml dist/
cp -ra www dist/
```

Dockerfile:

```shell
FROM alpine
ADD zoneinfo/PRC /etc/localtime
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/' /etc/apk/repositories \
    && apk add tzdata git openssh-client docker \
    && rm -f /var/cache/apk/* /usr/bin/dockerd /usr/bin/containerd* \
    && rm -f /usr/bin/ctr /usr/bin/runc /usr/bin/docker-proxy \
    && echo -e "Host *\n  StrictHostKeyChecking no\n  UserKnownHostsFile=/dev/null" >> /etc/ssh/ssh_config
ADD dist/ /opt/
ENTRYPOINT /opt/server
HEALTHCHECK --interval=10s --timeout=3s CMD /opt/server check
```

构建镜像：
```shell
docker build . -t $REGISTRY$CONTEXT/$PROJECT:$TAG
//docker login  $REGISTRY
docker push $REGISTRY$CONTEXT/$PROJECT:$TAG
//……
```

# deploy平台

## 基本配置

可在项目根目录放置一个 deploy.json：

```json
{
  "dataPath": "",
  "manageToken": "91deploy"
}
```
dataPath        代表hub的配置持久化存储的路径，不填写默认为/opt/deploy。

manageToken     代表hub的登录密码，以读写方式查看节点和应用的运行状态，不填写默认为91deploy。

可以使用 -e 'deploy_xxxxxx=xxxx' 进行配置。


启动容器：

使用 -e hub_managerToken=91hub 配置查看和管理口令进行登录授权。

使用 -e service_xxxx 来配置 http 相关参数，例如可以配置为基于 https 访问，具体配置请参考 https://github.com/ssgo/s。

## global

### Global Vars

全局变量，可以全局所有项目使用。

deploy过程中被配置成为运行范围内有效的环境变量，对每一个项目都起效果。

### Cache

构建缓存，尤其是依赖包，下次构建无更新可以不用在线拉取。

### SSKey Sync Token 

可以在这里配置token，提供api，相应主机调用这个api以后，可以同步sskey的指定key到目标节点。

复制以下command到sskey服务器执行：

```
sskey sync keyName url/${token}
```

即可将key以加密形式同步到当前机器

sskey sync的使用具体可以参考：[sskey详细使用手册](https://github.com/ssgo/tool/blob/master/sskey/sskey.md)。

### Public key

deploy的公钥，添加公钥到目标服务器中，放入~/.ssh/authorized_keys中。

可以用作远程部署。

也可以添加到私有代码仓库的SSH Keys，才能用SSH的方式无密拉取代码。

## 自定义context

将Repository、build、deploy归类管理。

### Projects

可以在这里设定项目名称、仓库地址(Repository)，Deploy Token，备注，build and deploy脚本（script），按照标签deploy，查看build历史记录。

镜像仓库地址是私有仓库的时候，请使用ssh方式，将公钥加入gitlab的用户setting中，直接clone代码，具体类似：

```shell
ssh://git@xxxx.xxxx.xxxx:port/group/xxx.git
```

Deploy Token 是给deploy开放api使用的。外部使用api使用token后，可以直接调用来完成deploy，一个项目一个Deploy Token。

Script包含build和deploy两大部分，用于项目的build与deploy。

### Vars

build与deploy脚本可以使用变量，变量就在Vars设定，仅仅在当前context下有效。

deploy过程中被配置成为运行范围内有效的环境变量，对整个context中每一个项目都起效果。

### Manage Token

当前context的管理token，使用这个token可以调用api设定当前context的项目仓库和deploy脚本，实际build与deploy。

也可以直接登录到deploy平台管理context与project。

### Memo

当前context的备注。

# 怎么编写Deploy CI脚本

自定义项目的build和deploy脚本，如下：

```
cachetag: $CONTEXT
cache: abc cache node_modules
build:
 - from: local
   script:
     - mkdir -p dist
     - mkdir -p cache
     - cp abc.txt dist/
     - sskey-go:aaa echo 'go build'
     - echo -n "$globalTitle" >> cache/stars
     - echo $(cat cache/stars) = $check1
     - test "$(cat cache/stars)" = $check1
     - cp cache/stars dist/stars

 - from: local # docker@192.168.0.61
   script:
     - echo -n "$flag" >> cache/stars
     - test "$(cat cache/stars)" = $check2
     - cp cache/stars dist/stars

deploy:
 from: local
 dockerfile:
   - FROM alpine:latest
   - ADD dist/ /opt/
   - ENTRYPOINT /opt/server
   - RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/' /etc/apk/repositories
     && apk add openssh-client
     && rm -f /var/cache/apk/*
   - HEALTHCHECK --interval=10s --timeout=3s CMD /opt/server check
 script:
   - docker build . -t $REGISTRY$CONTEXT/$PROJECT:$TAG
   - docker push $REGISTRY$CONTEXT/$PROJECT:$TAG
```

## cacheTag

build缓存，主要存放依赖包，每更改一次cacheTag重新拉取一次依赖包。

如果没有填写，默认为：

```
contextName + "-" + projectName
```

## cache

依赖包的实际存放目录：/${DataPath}/_cache/${cacheTag}/${cache}

例如：

```
/opt/deploy/_caches/ssgo-gateway-1.1/go
```

cache名字建议不要更改。

## build

当前project编译打包build的过程。

编译打包，提供项目部署文件、文件夹、压缩包、打包文件等。

#### from

from:local 代表本地构建。

from:docker@192.168.0.61 ssh无密登录到指定机器进行构建。

from:ssgo/hub 开启容器构建：

```
docker run -rm -v cachePath:cachePath -v buildPath:/root 指定参数 sh /root/buildFile
```

其中指定参数的第一个参数是镜像地址

运行容器，把后续命令放入构建文件中，执行。

#### script

执行的linux脚本。

#### SSkey

ssgo/deploy的构建支持：

```shell
sskey-go:aaa echo 'go build'
```

代表根据aaa密钥提供go语言版本sskey aes加密密钥生成文件，将生成文件放在项目根目录进行构建。

go语言的文件名是：``UniqueId.go``，文件名是随机的。

对于php和java来说sskey生成文件分别为sskeyStarter.php与SSKeyStarter.java。

构建后执行 ```echo 'go build'```

也可以自己提供生成文件名。

```shell
sskey-go:aaa:ghjasd.go echo 'go build'
```

代表指定密钥生成文件为ghjasd.go。

使用后这个文件会被系统删除，php是直接在项目中使用，不用删除。

注意：

sskey-go 后半部分定义的shell不可以使用cp||scp||mv命令。

#### SSkey流程

deploy结合SSkey使用流程：

![](deploy-sskey-flow.png?v=1.1)

sskey管理员定制编译deploy：

```shell
mkdir -p dist
sed -i 's/__TAG__/$TAG/g' www/index.html www/views/Deploy.html
sskey -go sync > _.go
export GOPROXY=https://goproxy.io
export CGO_ENABLED=0
go mod tidy
go build -ldflags -w -o dist/server
rm -f _.go
cp *.yml dist/
cp -ra www dist/
```

将编译文件写入镜像，服务运行时，管理员在sskey的机器上运行sskey -sync指定秘钥传递给deploy服务，保障sskey秘钥的安全。

## deploy

经过deploy操作提供成品的过程，产品为镜像或压缩包等。

#### from

指定本地或远程服务器执行deploy操作。

#### Dockerfile

与docker的Dockerfile编写语法一致。

注意：换行的时候不一致，这里不要使用反斜线换行，换行后多一个制表符。

#### script

执行的linux脚本，制造镜像或其他成品。

到这里完成deploy的工作。

注意：

deploy是生产成品的过程，实际部署可以借助其他工具。

如果部署使用的是docker容器，推荐使用ssgo/hub来做轻量级的容器编排，具体可以查看[ssgo/hub](https://github.com/ssgo/hub)。

# Api

deploy的开放api可以参考：[api文档](api.json)。

其中字段的具体含义：

- Type有Web,WebSocket,Action,Proxy,Rewrite
- Path是url路径
- AuthLevel是授权等级(authLevel=0表示不需要授权)
- Method就是restful api中的method方法
- In代表入参
- Out代表出参

login api可以直接调用。

其他api需要使用鉴权。使用相关权限的token才可以正常调用api。

token在api请求的header头Access-Token中设置具体值。