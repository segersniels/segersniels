FROM golang:1.22.2-alpine AS builder

WORKDIR /app

COPY . .

RUN apk add --no-cache make && make build

FROM golang:1.22.2-alpine

WORKDIR /app

COPY --from=builder /app/bin/ssh ./ssh
COPY ./README.md .

EXPOSE 22
CMD ["./ssh"]