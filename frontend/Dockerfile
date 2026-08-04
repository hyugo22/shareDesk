FROM node:20-alpine AS build
WORKDIR /app
COPY package.json ./
RUN npm install --no-audit --no-fund
COPY . .
ARG VITE_API_BASE_URL
ARG VITE_WS_BASE_URL
ENV VITE_API_BASE_URL=$VITE_API_BASE_URL
ENV VITE_WS_BASE_URL=$VITE_WS_BASE_URL
RUN npm run build

# Simple serveur de fichiers statiques pour la SPA compilée — ce n'est pas un
# reverse proxy applicatif : le frontend appelle directement le backend via
# VITE_API_BASE_URL/VITE_WS_BASE_URL (TLS/exposition hors périmètre de ce projet).
FROM nginx:alpine
COPY --from=build /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
