FROM nginxinc/nginx-unprivileged:stable-alpine3.23@sha256:b3f2436575bd5be7386518084d842dac414ab4962712afa31e99e0942a56e3b2

USER root
RUN apk upgrade --no-cache
USER nginx

COPY --chown=nginx:nginx nginx.conf /etc/nginx/conf.d/default.conf
COPY --chown=nginx:nginx index.html /usr/share/nginx/html/index.html

EXPOSE 8080
