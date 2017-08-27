FROM nginx:latest

# Install root certificates
RUN apt-get -y -qq update \
 && apt-get -y -qq install --no-install-recommends ca-certificates \
 && apt-get -y -qq clean \
 && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

ENTRYPOINT ["/bin/bash", "/start.sh"]

ENV AILOG_VERSION 0.1.0
RUN apt-get -y -qq update \
 && apt-get -y -qq install curl ca-certificates gpg dirmngr --no-install-recommends \
 && curl -L -o /ailognginx https://github.com/jjjordanmsft/ApplicationInsights-logforward/releases/download/$AILOG_VERSION/ailognginx-$AILOG_VERSION-linux-amd64 \
 && curl -L -o /ailogtrace https://github.com/jjjordanmsft/ApplicationInsights-logforward/releases/download/$AILOG_VERSION/ailogtrace-$AILOG_VERSION-linux-amd64 \
 && curl -L -o /ailognginx.asc https://github.com/jjjordanmsft/ApplicationInsights-logforward/releases/download/$AILOG_VERSION/ailognginx-$AILOG_VERSION-linux-amd64.asc \
 && curl -L -o /ailogtrace.asc https://github.com/jjjordanmsft/ApplicationInsights-logforward/releases/download/$AILOG_VERSION/ailogtrace-$AILOG_VERSION-linux-amd64.asc \
 && gpg --keyserver ha.pool.sks-keyservers.net --recv-keys 0257888EF7FABDE4 \
 && gpg --verify /ailognginx.asc \
 && gpg --verify /ailogtrace.asc \
 && rm /ailog*.asc \
 && chmod 755 /ailognginx /ailogtrace \
 && apt-get -y -qq purge curl gpg dirmngr \
 && apt-get -y -qq autoremove

# Or, for a less secure but less complicated example:
#ADD https://github.com/jjjordanmsft/ApplicationInsights-logforward/releases/download/$AILOG_VERSION/ailognginx-$AILOG_VERSION-linux-amd64 \
#    https://github.com/jjjordanmsft/ApplicationInsights-logforward/releases/download/$AILOG_VERSION/ailogtrace-$AILOG_VERSION-linux-amd64 /
#RUN mv /ailognginx-$AILOG_VERSION-linux-amd64 /ailognginx \
# && mv /ailogtrace-$AILOG_VERSION-linux-amd64 /ailogtrace \
# && chmod 755 /ailognginx /ailogtrace
# ...but really, you should use the above.

COPY nginx.conf.template /etc/nginx/
COPY start.sh /
