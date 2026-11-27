if (-not $env:TEST_API_URL) { $env:TEST_API_URL = "http://localhost:8080" }
# i due database dei microservizi: TEST_DATABASE_URL è il globale (DATABASE_URL del
# backend), TEST_DATABASE_SPEDIZIONI_URL quello delle spedizioni (DATABASE_SPEDIZIONI_URL)
if (-not $env:TEST_DATABASE_URL) {
    Write-Error "TEST_DATABASE_URL non impostata: usa la stessa stringa di connessione del backend (DATABASE_URL)"
    exit 1
}
if (-not $env:TEST_DATABASE_SPEDIZIONI_URL) {
    Write-Error "TEST_DATABASE_SPEDIZIONI_URL non impostata: usa la stessa stringa di connessione del backend (DATABASE_SPEDIZIONI_URL)"
    exit 1
}

go test -tags=integration -count=1 -v ./tests
