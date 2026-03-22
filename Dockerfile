FROM golang:1.25 AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/orbyte-server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/orbyte-migrate ./cmd/migrate

FROM gcr.io/distroless/base-debian12

WORKDIR /app
COPY --from=build /out/orbyte-server /app/orbyte-server
COPY --from=build /out/orbyte-migrate /app/orbyte-migrate

EXPOSE 8080
ENTRYPOINT ["/app/orbyte-server"]
