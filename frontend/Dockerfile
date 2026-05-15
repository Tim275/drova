FROM nginxinc/nginx-unprivileged:stable-alpine3.23@sha256:2c5a98dcd9bfb568fd6a305e0637f0410beafdbf644443112ab47d6a713ba43e

USER root
RUN apk upgrade --no-cache
USER nginx

COPY --chown=nginx:nginx nginx.conf /etc/nginx/conf.d/default.conf
COPY --chown=nginx:nginx index.html /usr/share/nginx/html/index.html

EXPOSE 8080
