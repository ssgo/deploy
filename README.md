ssgo/deploy是一套简单易用的构建部署工具

# 快速使用

首先宿主机上要安装docker

然后运行：

```shell
docker run -d --restart=always --network=host -v /opt/deploy:/opt/deploy -v /var/run/docker.sock:/var/run/docker.sock {deploy-image}
```

deploy运行起来之后就可以使用了

镜像也可以自己根据ssgo/deploy的代码进行构建

deploy的启动也可以在ssgo/hub中配置开启

## 容器网络模式

容器启动需要使用主机网络模式，使用宿主机网络和端口，通讯效率高

## 存储依赖

数据会存储在 /opt/data 下，可以使用 -v /opt/hub:/opt/data 来挂载外部磁盘

/var/run/docker.sock代表容器内部使用宿主机的docker

# 自定义deploy镜像

编译deploy：

```shell
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
    && apk add openssh-client && rm -f /var/cache/apk/*
ADD dist/ /opt/
ENTRYPOINT /opt/server
HEALTHCHECK --interval=10s --timeout=3s CMD /opt/server check
```

构建镜像：
```shell
docker build . -t $REGISTRY$CONTEXT/$PROJECT:$TAG
docker push $REGISTRY$CONTEXT/$PROJECT:$TAG
```

# deploy平台

## 基本配置

可在项目根目录放置一个 deploy.json

```json
{
  "dataPath": "",
  "manageToken": "91deploy"
}
```
dataPath        代表hub的配置持久化存储的路径，不填写默认为/opt/deploy

manageToken     代表hub的登录密码，以读写方式查看节点和应用的运行状态，不填写默认为91deploy

可以使用 -e 'deploy_xxxxxx=xxxx' 进行配置


启动容器：

使用 -e hub_managerToken=91hub 配置查看和管理口令进行登录授权

使用 -e service_xxxx 来配置 http 相关参数，例如可以配置为基于 https 访问，具体配置请参考 https://github.com/ssgo/s

## 访问

hub访问通过：http://xx.xx.xx.xx:7777/

默认使用 8888 端口，可以使用 -p xxxx:7777 来改变端口

或者启动容器指定 service_listen=":xxxx" 来改变端口

## global

### Global Vars

全局变量，可以全局所有项目使用

deploy过程中被配置成为运行范围内有效的环境变量，对每一个项目都起效果

### Cache

构建缓存，尤其是依赖包，下次构建无更新可以不用在线拉取

### Public key

deploy的公钥，添加公钥到目标服务器中，放入~/.ssh/authorized_keys中

可以用作远程部署

也可以添加到私有代码仓库的SSH Keys，才能用SSH的方式无密拉取代码

## 自定义context

将Repository、build、deploy归类管理

### Projects

可以在这里设定项目名称、仓库地址(Repository)，Deploy Token，备注，build and deploy脚本（script），按照标签deploy，查看build历史记录

镜像仓库地址是私有仓库的时候，请使用ssh方式，将公钥加入gitlab的用户setting中，直接clone代码，具体类似：

```shell
ssh://git@xxxx.xxxx.xxxx:port/group/xxx.git
```

Deploy Token 是给deploy开放api使用的。外部使用api使用token后，可以直接调用来完成deploy，一个项目一个Deploy Token

Script包含build和deploy两大部分，用于项目的build与deploy

### Vars

build与deploy脚本可以使用变量，变量就在Vars设定，仅仅在当前context下有效

deploy过程中被配置成为运行范围内有效的环境变量，对整个context中每一个项目都起效果

### Manage Token

当前context的管理token，使用这个token可以调用api设定当前context的项目仓库和deploy脚本，实际build与deploy

也可以直接登录到deploy平台管理context与project

### Memo

当前context的备注

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
   - cp dist/stars dist/stars2
   - echo -n "!!!" >> cache/stars2
   - echo "$(cat dist/abc.txt)" = {$checkABC}
   - test "$(cat dist/abc.txt)" = $checkABC
```

### cacheTag

build缓存，主要存放依赖包，每更改一次cacheTag重新拉取一次依赖包

如果没有填写，默认为：

```
contextName + "-" + projectName
```

### cache

依赖包的实际存放目录：/${DataPath}/_cache/${cacheTag}/${cache}

例如：

```
/opt/deploy/_caches/ssgo-gateway-1.1/go
```

cache名字建议不要更改

### build

当前project编译打包build的过程

编译打包，提供项目部署文件、文件夹、压缩包、打包文件等

#### from

from:local 代表本地构建

from docker@192.168.0.61 ssh无密登录到指定机器进行构建

#### script

执行的linux脚本

### deploy

deploy为成品的过程，镜像或压缩包

#### from

指定本地或远程服务器执行deploy操作

#### Dockerfile

与docker的Dockerfile编写语法一直

#### script

执行的linux脚本

注意：换行的时候不一致，这里不要使用反斜线换行，换行后多一个制表符