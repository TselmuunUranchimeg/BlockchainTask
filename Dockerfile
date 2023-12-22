FROM golang:1.21

LABEL maintainer="Tselmuun"

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . ./

RUN CGO_ENABLED=0 GOOS=linux go build -o /docker_golang_service

CMD ["/docker_golang_service"]