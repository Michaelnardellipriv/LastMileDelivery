$env:TEST_API_URL = "http://localhost:8080"

$env:TEST_ID_OPERATORE = Read-Host "ID operatore logistico"
$env:TEST_ID_MITTENTE = Read-Host "ID cliente mittente"
$env:TEST_ID_DESTINATARIO = Read-Host "ID cliente destinatario"
$env:TEST_ID_RITIRO = Read-Host "ID indirizzo di ritiro"
$env:TEST_ID_CONSEGNA = Read-Host "ID indirizzo di consegna"

# Posizione dei corrieri (lasciate vuoto TEST_ID_CORRIERE per saltare quel test)
$env:TEST_ID_CORRIERE = Read-Host "ID corriere con spedizioni in consegna (invio per saltare)"
if ($env:TEST_ID_CORRIERE) {
    $env:TEST_ID_CORRIERE_INATTIVO = Read-Host "ID corriere senza spedizioni in consegna (es. solo ritirata)"
    $env:TEST_ID_AMMINISTRATORE = Read-Host "ID amministratore"
}

go test -tags=integration -count=1 -v ./tests