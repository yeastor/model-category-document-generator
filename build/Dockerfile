FROM golang:1.26.0-alpine3.23 AS builder

WORKDIR /build

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o application ./cmd/document-generator

FROM alpine:3.22

RUN apk add --no-cache \
    ca-certificates \
    curl \
    python3 \
    py3-reportlab \
    ttf-dejavu

WORKDIR /app

COPY --from=builder /build/application ./application
COPY data ./data
COPY document-templates ./document-templates
COPY public ./public
COPY scripts ./scripts
COPY integration-example.where_to.json ./integration-example.where_to.json
COPY README.md ./README.md

RUN mkdir -p generated

ENV APP_PORT=4177
ENV PYTHON_BIN=python3
ENV PDF_FONT_PATH=/usr/share/fonts/dejavu/DejaVuSans.ttf

EXPOSE 4177

ENTRYPOINT ["./application"]
