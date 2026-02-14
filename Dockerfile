# Build stage
FROM node:20-alpine AS builder

WORKDIR /build

# Version build arguments
ARG VERSION=dev
ARG BUILD=unknown
ARG GIT_COMMIT=unknown

# Copy package files first for better caching
COPY package.json package-lock.json ./
RUN npm ci

# Copy source code
COPY . .

# Inject version info into the build
ENV VITE_APP_VERSION=${VERSION}
ENV VITE_APP_BUILD=${BUILD}
ENV VITE_APP_COMMIT=${GIT_COMMIT}

# Build static assets
RUN npm run build

# Runtime stage
FROM nginx:1.27-alpine

LABEL org.opencontainers.image.source="https://github.com/bobmcallan/vire-portal"

# Copy built assets from builder
COPY --from=builder /build/dist /usr/share/nginx/html

# Copy nginx config with env substitution template
COPY nginx.conf /etc/nginx/templates/default.conf.template

# Copy version file
COPY .version /usr/share/nginx/html/.version

# nginx:alpine uses envsubst on templates in /etc/nginx/templates/
# and writes output to /etc/nginx/conf.d/ at startup

EXPOSE 8080

# Restrict envsubst to only API_URL and DOMAIN.
# Without this, envsubst replaces nginx's own $uri, $request_uri, etc. with
# empty strings, breaking SPA routing and proxy directives.
ENV NGINX_ENVSUBST_FILTER="API_URL|DOMAIN"

# nginx:alpine default entrypoint handles template substitution
# No custom entrypoint needed
