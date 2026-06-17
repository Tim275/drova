FROM nginxinc/nginx-unprivileged:stable-alpine3.23@sha256:de3e40ec8b7debd7194fc798d4bbfb102c7f8b012b2c73032816b5f72393acdd

USER root
RUN apk upgrade --no-cache
USER nginx

COPY --chown=nginx:nginx nginx.conf /etc/nginx/conf.d/default.conf
COPY --chown=nginx:nginx index.html /usr/share/nginx/html/index.html

EXPOSE 8080
