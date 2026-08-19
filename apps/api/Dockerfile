FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY apps/api ./apps/api
COPY packages/scenario ./packages/scenario
RUN CGO_ENABLED=0 go build -o /out/server ./apps/api/cmd/server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates wget
WORKDIR /app
COPY --from=build /out/server /app/server
EXPOSE 8012
ENV RELIABILITY_HTTP_PORT=8012
CMD ["/app/server"]
