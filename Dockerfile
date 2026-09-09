FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 go build -o /out/en16931 ./cmd/en16931

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/en16931 /usr/local/bin/en16931
EXPOSE 8080
ENTRYPOINT ["en16931"]
CMD ["serve", "-addr", ":8080"]
