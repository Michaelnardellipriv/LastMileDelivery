if (-not $env:TEST_API_URL) { $env:TEST_API_URL = "http://localhost:8080" }
if (-not $env:TEST_DATABASE_URL) {
    Write-Error "TEST_DATABASE_URL non impostata: usa la stessa stringa di connessione del backend (DATABASE_URL)"
    exit 1
}

go test -tags=integration -count=1 -v ./tests
