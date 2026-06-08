# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN mkdir -p bin
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/http ./cmd/http
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/scheduled ./cmd/scheduled

# Lambda stage
FROM public.ecr.aws/lambda/provided:al2

COPY --from=builder /app/bin/${HANDLER} ${LAMBDA_TASK_ROOT}/

CMD [ "main" ]
