FROM nginxinc/nginx-unprivileged:stable-alpine3.23@sha256:df0e9edf92b8436ff797fe5a2cbfc66be1df775c113d322ccadf5c7f3100eda8

USER root
RUN apk upgrade --no-cache
USER nginx

COPY --chown=nginx:nginx nginx.conf /etc/nginx/conf.d/default.conf
COPY --chown=nginx:nginx index.html /usr/share/nginx/html/index.html

EXPOSE 8080
