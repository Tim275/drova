FROM nginxinc/nginx-unprivileged:stable-alpine3.23@sha256:0a1e718ff1e1a22fc519d0c2e5b6872681f01e37c8a2817ec43ce6e716103929

USER root
RUN apk upgrade --no-cache
USER nginx

COPY --chown=nginx:nginx nginx.conf /etc/nginx/conf.d/default.conf
COPY --chown=nginx:nginx index.html /usr/share/nginx/html/index.html

EXPOSE 8080
