FROM node:22-alpine AS build

WORKDIR /app

COPY web/package.json web/package-lock.json ./
RUN npm ci

COPY web/ ./

ARG VITE_API_URL=http://localhost:8080
ENV VITE_API_URL=${VITE_API_URL}

RUN npm run build

FROM nginx:1.29-alpine

COPY --from=build /app/dist /usr/share/nginx/html
RUN printf 'server { listen 80; server_name _; root /usr/share/nginx/html; location / { try_files $uri $uri/ /index.html; } }\n' > /etc/nginx/conf.d/default.conf

EXPOSE 80
