# fase 1: compila
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /server .

# fase 2: esegue (immagine minima, utente non root)
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /server /server
USER nonroot
EXPOSE 8080
CMD ["/server"]
