FROM golang:1.25-alpine AS build
WORKDIR /src
RUN apk add --no-cache git
COPY . .
RUN go mod download && CGO_ENABLED=0 go build -o /out/sharedesk-server ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/sharedesk-server /sharedesk-server
USER nonroot:nonroot
EXPOSE 8080 8443
ENTRYPOINT ["/sharedesk-server"]
