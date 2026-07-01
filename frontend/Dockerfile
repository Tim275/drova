FROM nginxinc/nginx-unprivileged:stable-alpine3.23@sha256:fd3314e343bad2de4e1127ef58be122abbfa7e09572fa46ae62fcddb6b3f21c5

USER root
RUN apk upgrade --no-cache
USER nginx

COPY --chown=nginx:nginx nginx.conf /etc/nginx/conf.d/default.conf
COPY --chown=nginx:nginx index.html /usr/share/nginx/html/index.html

EXPOSE 8080
