FROM golang:1.25-alpine AS build
WORKDIR /src
RUN apk add --no-cache git
COPY . .
RUN go mod download && CGO_ENABLED=0 go build -o /out/sharedesk-server ./cmd/server
# Répertoire de données de la CA interne (montée en volume), pré-créé avec les
# droits de l'utilisateur non-root : l'image distroless finale n'a pas de
# shell pour faire un chown, et Docker initialise un volume nommé vide avec le
# contenu/les droits du répertoire correspondant de l'image au premier montage.
RUN mkdir -p /out/data/ca && chown -R 65532:65532 /out/data

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/sharedesk-server /sharedesk-server
COPY --from=build --chown=65532:65532 /out/data /data
USER nonroot:nonroot
EXPOSE 8080 8443
ENTRYPOINT ["/sharedesk-server"]
