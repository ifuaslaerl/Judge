package main

import (
    "log"
    "net/http"
)

func main() {
    // Simple health check handler
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("Judge Platform Online"))
    })

    // Port 8443 as specified in system_design.txt 
    port := ":8443"
    certFile := "certs/server.crt"
    keyFile := "certs/server.key"

    log.Printf("Starting secure server on https://localhost%s", port)
    
    // ListenAndServeTLS requires the cert and key generated above
    err := http.ListenAndServeTLS(port, certFile, keyFile, nil)
    if err != nil {
        log.Fatalf("Failed to start server: %v", err)
    }
}
