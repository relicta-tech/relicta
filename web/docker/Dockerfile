# Stage 1: Build the Vue frontend
FROM node:24-alpine3.24@sha256:116b800c3754b440ee0aa3ed2f20f5bf3d99a554bfd9adeea09ea8017f6e85f5 AS builder

WORKDIR /app

# Pull patched Alpine packages from the maintained v3.24 repositories.
RUN apk upgrade --no-cache

# Install dependencies first (cache layer)
COPY package.json package-lock.json ./
# The pinned Node image bundles an older npm with vulnerable bundled
# dependencies. Upgrade npm to a fixed, Node 24-compatible release before use.
RUN npm install --global --ignore-scripts npm@12.2.3 && npm ci

# Copy source and build
COPY . .

# VITE_API_URL can be overridden at build time
ARG VITE_API_URL=""
ENV VITE_API_URL=${VITE_API_URL}

RUN npm run build

# Stage 2: Serve with nginx
FROM nginx:1.30.8-alpine3.24@sha256:8eb04fe3b385090e9aa02961f77398ad09b6bf33ab07f6669a5bb7acce22c0d3

# This dashboard only serves static files and proxies API traffic; it does not
# use Nginx's optional image transformation module. Removing it also drops the
# libgd/fontconfig/expat dependency chain from the runtime image.
# Apply security fixes published after the pinned base image was built.
RUN apk del nginx-module-image-filter && \
    if apk info -e libexpat >/dev/null 2>&1; then \
        echo "libexpat remains installed after removing nginx-module-image-filter" >&2; \
        exit 1; \
    fi && \
    apk upgrade --no-cache

# Nginx proxies API and WebSocket requests to this Relicta server. The official
# image entrypoint expands this template from RELICTA_API_URL at container start.
ENV RELICTA_API_URL=http://relicta:8080
COPY docker/nginx.conf /etc/nginx/templates/default.conf.template

# Copy built frontend
COPY --from=builder /app/dist /usr/share/nginx/html

# Run as non-root
RUN chown -R nginx:nginx /usr/share/nginx/html && \
    chown -R nginx:nginx /etc/nginx/conf.d && \
    chown -R nginx:nginx /var/cache/nginx && \
    chown -R nginx:nginx /var/log/nginx && \
    touch /var/run/nginx.pid && \
    chown -R nginx:nginx /var/run/nginx.pid

USER nginx

EXPOSE 80

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost/nginx-health || exit 1

CMD ["nginx", "-g", "daemon off;"]
