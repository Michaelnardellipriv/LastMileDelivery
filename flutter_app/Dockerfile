# Frontend web (area riservata: operatori, corrieri, amministratore).
#   docker build -t lastmile/frontend:1.0.0 flutter_app
#   docker run --rm -p 8081:8080 lastmile/frontend:1.0.0
# Senza API_BASE l'app chiama l'API sullo stesso indirizzo della pagina: dietro l'Ingress
# "/" va al frontend e "/api" al backend, quindi la stessa immagine va bene in ogni ambiente.

# fase 1: compila l'app Flutter per il web (versione di Flutter fissata)
FROM ghcr.io/cirruslabs/flutter:3.27.0 AS build
WORKDIR /app
COPY pubspec.yaml pubspec.lock ./
RUN flutter pub get
COPY . .
RUN flutter build web --release -t lib/main_operatore.dart

# fase 2: serve solo i file compilati, con nginx senza permessi di root
FROM nginxinc/nginx-unprivileged:1.27-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/build/web /usr/share/nginx/html
USER nginx
EXPOSE 8080
CMD ["nginx", "-g", "daemon off;"]
