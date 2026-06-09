FROM nginxinc/nginx-unprivileged:stable-alpine3.23@sha256:37f356a5eba5d187365b4f59cd6cc29f1f922ad18146d554b576a80983377e6a

USER root
RUN apk upgrade --no-cache
USER nginx

COPY --chown=nginx:nginx nginx.conf /etc/nginx/conf.d/default.conf
COPY --chown=nginx:nginx index.html /usr/share/nginx/html/index.html

EXPOSE 8080
