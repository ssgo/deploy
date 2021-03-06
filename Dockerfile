FROM alpine
ADD ./dist /opt
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/' /etc/apk/repositories \
  && apk add tzdata git openssh-client docker rsync && rm -f /var/cache/apk/* /usr/bin/dockerd /usr/bin/containerd* /usr/bin/ctr /usr/bin/runc /usr/bin/docker-proxy \
  && cp -f /usr/share/zoneinfo/PRC /etc/localtime \
  && echo -e "Host *\n  StrictHostKeyChecking no\n  UserKnownHostsFile=/dev/null" >> /etc/ssh/ssh_config
ENTRYPOINT /opt/deploy.server
HEALTHCHECK --interval=10s --timeout=3s CMD /opt/deploy.server check