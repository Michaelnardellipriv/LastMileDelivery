$env:TEST_API_URL = "http://localhost:8080"

$env:TEST_ID_OPERATORE = Read-Host "ID operatore logistico"
$env:TEST_ID_MITTENTE = Read-Host "ID cliente mittente"
$env:TEST_ID_DESTINATARIO = Read-Host "ID cliente destinatario"
$env:TEST_ID_RITIRO = Read-Host "ID indirizzo di ritiro"
$env:TEST_ID_CONSEGNA = Read-Host "ID indirizzo di consegna"

go test -tags=integration -count=1 -v ./tests