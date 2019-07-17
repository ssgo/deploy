FROM alpine
ADD ./www server /opt/
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/' /etc/apk/repositories \
  && apk add tzdata git openssh-client docker && rm -f /var/cache/apk/* /usr/bin/dockerd /usr/bin/containerd* /usr/bin/ctr /usr/bin/runc /usr/bin/docker-proxy \
  && cp -f /usr/share/zoneinfo/PRC /etc/localtime \
  && echo "\nHost *\nStrictHostKeyChecking no\nUserKnownHostsFile=/dev/null" >> /etc/ssh/ssh_config
ENTRYPOINT /opt/server
HEALTHCHECK --interval=10s --timeout=3s CMD /opt/server check