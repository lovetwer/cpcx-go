# 多阶段构建：纯 API 后端，不嵌入前端静态资源
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/lottery-manager .

FROM alpine:3.19
WORKDIR /app
COPY --from=build /out/lottery-manager /app/lottery-manager
EXPOSE 8080
CMD ["/app/lottery-manager"]
