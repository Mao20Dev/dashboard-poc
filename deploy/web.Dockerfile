FROM node:22-alpine AS build
WORKDIR /app
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM nginxinc/nginx-unprivileged:alpine
ENV API_UPSTREAM=api:8080
ENV NGINX_ENVSUBST_OUTPUT_DIR=/tmp/nginx-conf
ENV NGINX_ENTRYPOINT_LOCAL_RESOLVERS=1
RUN mkdir -p /tmp/nginx-conf
COPY deploy/nginx.conf /etc/nginx/nginx.conf
COPY deploy/default.conf.template /etc/nginx/templates/default.conf.template
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 8080
