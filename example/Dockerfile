FROM nginx:latest

# Install root certificates
RUN apt-get -y -qq update \
 && apt-get -y -qq install --no-install-recommends ca-certificates \
 && apt-get -y -qq clean \
 && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

ENTRYPOINT ["/bin/bash", "/start.sh"]
COPY nginx.conf.template /etc/nginx/
COPY ailognginx ailogtrace start.sh /
RUN chmod 755 /ailognginx /ailogtrace
