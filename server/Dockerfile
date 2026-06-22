FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /handy-notion .

FROM alpine:3.21
RUN adduser -D app
USER app
COPY --from=build /handy-notion /handy-notion
EXPOSE 8082
ENTRYPOINT ["/handy-notion"]

