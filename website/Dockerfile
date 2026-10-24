FROM golang:1.26.2-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /postika-website .

FROM alpine:3.23
RUN addgroup -S postika && adduser -S postika -G postika
COPY --from=build /postika-website /usr/local/bin/postika-website
USER postika
ENV ADDR=0.0.0.0:8080
EXPOSE 8080
ENTRYPOINT ["postika-website"]
