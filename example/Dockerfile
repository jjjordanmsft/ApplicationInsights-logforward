FROM nginx:latest

ENTRYPOINT ["/bin/bash", "/start.sh"]
COPY nginx.conf.template /etc/nginx/
COPY ailognginx ailogtrace start.sh /
RUN chmod 755 /ailognginx /ailogtrace
