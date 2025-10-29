package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	InitDB()
	defer DB.Close()

	http.HandleFunc("/register", RegisterHandler)
    http.HandleFunc("/login", LoginHandler)
    http.HandleFunc("/rooms", GetRoomsHandler)
	http.HandleFunc("/pemasukan", Pemasukan)
    http.HandleFunc("/pengeluaran", Pengeluaran)
	http.HandleFunc("/transaksi-lainnya", TambahTransaksi)
	http.HandleFunc("/get-transaksi", GetTransaksi)

	port := 8080
	fmt.Printf("🚀 Server berjalan di http://localhost:%d\n", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}
