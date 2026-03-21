# Сборка и тесты в Linux-окружении (аналог CI): docker build -t smartroute-ci .
FROM golang:1.21-bookworm
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN make test && make build
