# xhtml

Copia del parser HTML5 `golang.org/x/net/html` (con `atom`), versione
**v0.40.0**, licenza BSD (vedi `LICENSE`). Esclusi test, dati di test e il
generatore di `atom`; l'unica modifica è il percorso di import di `atom`.

È inclusa nel repository perché gli ambienti di sviluppo usati non sempre
raggiungono il proxy dei moduli Go. Per aggiornarla: copiare i file dalla
nuova versione, ripetere la modifica dell'import, eseguire i test del gateway.
