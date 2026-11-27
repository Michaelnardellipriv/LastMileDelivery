# Immagine di uno dei due microservizi: SERVIZIO=utenti oppure SERVIZIO=spedizioni
# (cmd/utenti, cmd/spedizioni). Stesso codice, due programmi, due immagini:
#   docker build --build-arg SERVIZIO=utenti     -t lastmile/utenti:2.0.0     backend
#   docker build --build-arg SERVIZIO=spedizioni -t lastmile/spedizioni:2.0.0 backend
# Con SERVIZIO=server si ottiene il server di sviluppo (i due insieme, due database).

# fase 1: compila
FROM golang:1.26-alpine AS build
ARG SERVIZIO=utenti
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /servizio ./cmd/${SERVIZIO}

# fase 2: esegue (immagine minima, utente non root)
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /servizio /servizio
USER nonroot
EXPOSE 8080
CMD ["/servizio"]
