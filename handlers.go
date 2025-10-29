package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Hapus struct RegisterRequest di sini (pakai dari models.go)

func hashPassword(pw string) string {
	hash := sha256.Sum256([]byte(pw))
	return hex.EncodeToString(hash[:])
}

func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Hanya POST method yang diizinkan", http.StatusMethodNotAllowed)
		return
	}

	var req RegisterRequest // ← gunakan struct dari models.go
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "JSON tidak valid", http.StatusBadRequest)
		return
	}

	if req.Password != req.ConfirmPassword {
		http.Error(w, "Konfirmasi password tidak cocok", http.StatusBadRequest)
		return
	}

	var count int
	err = DB.QueryRow(context.Background(), "SELECT COUNT(*) FROM users WHERE room_id=$1", req.RoomID).Scan(&count)
	if err != nil {
		http.Error(w, "Gagal memeriksa room", http.StatusInternalServerError)
		return
	}

	if count >= 2 {
		http.Error(w, fmt.Sprintf("Room %d sudah penuh", req.RoomID), http.StatusBadRequest)
		return
	}

	passwordHash := hashPassword(req.Password)
	_, err = DB.Exec(context.Background(),
		`INSERT INTO users (full_name, email, password_hash, room_id, created_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		strings.TrimSpace(req.FullName),
		strings.ToLower(req.Email),
		passwordHash,
		req.RoomID,
		time.Now(),
	)

	if err != nil {
		http.Error(w, "Gagal menyimpan data: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, `{"message": "Registrasi berhasil untuk %s di Room %d"}`, req.FullName, req.RoomID)
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Hanya POST method yang diizinkan", http.StatusMethodNotAllowed)
		return
	}

	type LoginRequest struct {
		Identifier string `json:"identifier"`
		Password   string `json:"password"`
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON tidak valid", http.StatusBadRequest)
		return
	}

	if req.Identifier == "" || req.Password == "" {
		http.Error(w, "Email/Nama dan password wajib diisi", http.StatusBadRequest)
		return
	}

	// 🔹 Ambil user
	var storedHash, fullName, email string
	var roomID, userID int
	queryUser := `
		SELECT id, password_hash, full_name, email, room_id
		FROM users
		WHERE LOWER(email) = LOWER($1) OR LOWER(full_name) = LOWER($1)
	`
	err := DB.QueryRow(context.Background(), queryUser, req.Identifier).Scan(
		&userID, &storedHash, &fullName, &email, &roomID,
	)
	if err != nil {
		http.Error(w, "User tidak ditemukan", http.StatusUnauthorized)
		return
	}

	// 🔹 Verifikasi password
	inputHash := hashPassword(req.Password)
	if inputHash != storedHash {
		http.Error(w, "Password salah", http.StatusUnauthorized)
		return
	}

	// 🔹 Ambil info room
	var (
		roomName             string
		tanggalBuatRoom      time.Time
		totalPemasukanRoom   float64
		totalPengeluaranRoom float64
		totalSaldoRoom       float64
		membersStr           string
	)

	queryRoom := `
    SELECT 
        r.room_name,
        r.created_at,
        COALESCE(rt.total_pemasukan_room, 0),
        COALESCE(rt.total_pengeluaran_room, 0),
        COALESCE(rb.total_saldo, 0),
        COALESCE(m.members, '')
    FROM rooms r
    LEFT JOIN LATERAL (
        SELECT 
            SUM(pemasukan) AS total_pemasukan_room,
            SUM(pengeluaran) AS total_pengeluaran_room
        FROM user_transactions ut
        WHERE ut.room_id = r.id
    ) rt ON TRUE
    LEFT JOIN room_balance rb ON rb.room_id = r.id
    LEFT JOIN LATERAL (
        SELECT STRING_AGG(full_name, ', ') AS members
        FROM users u 
        WHERE u.room_id = r.id
    ) m ON TRUE
    WHERE r.id = $1
`
	err = DB.QueryRow(context.Background(), queryRoom, roomID).Scan(
		&roomName, &tanggalBuatRoom,
		&totalPemasukanRoom, &totalPengeluaranRoom, &totalSaldoRoom, &membersStr,
	)
	if err != nil {
		http.Error(w, fmt.Sprintf("Gagal ambil data room: %v", err), http.StatusInternalServerError)
		return
	}

	// 🔹 Ambil semua pemasukan user + other_transaction
	pemasukanMap := make(map[string]float64)

	// dari user_transactions
	pemasukanRows, err := DB.Query(context.Background(), `
		SELECT tanggal_update::date AS tanggal_update, pemasukan
		FROM user_transactions
		WHERE user_id = $1 AND room_id = $2 AND pemasukan > 0
		ORDER BY tanggal_update ASC
	`, userID, roomID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Gagal ambil pemasukan: %v", err), http.StatusInternalServerError)
		return
	}
	defer pemasukanRows.Close()

	for pemasukanRows.Next() {
		var tanggal time.Time
		var amount float64
		if err := pemasukanRows.Scan(&tanggal, &amount); err == nil {
			key := tanggal.Format("2006-01-02")
			pemasukanMap[key] += amount
		}
	}

	// dari other_transaction
	otherPemasukanRows, err := DB.Query(context.Background(), `
		SELECT tanggal_update::date AS tanggal_update, nominal
		FROM other_transaction
		WHERE user_id = $1 AND room_id = $2 AND jenis = 'Pemasukan'
		ORDER BY tanggal_update ASC
	`, userID, roomID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Gagal ambil pemasukan lain: %v", err), http.StatusInternalServerError)
		return
	}
	defer otherPemasukanRows.Close()

	for otherPemasukanRows.Next() {
		var tanggal time.Time
		var nominal float64
		if err := otherPemasukanRows.Scan(&tanggal, &nominal); err == nil {
			key := tanggal.Format("2006-01-02")
			pemasukanMap[key] += nominal
		}
	}

	// ubah map jadi slice
	var pemasukanHarian []map[string]interface{}
	for tanggal, total := range pemasukanMap {
		pemasukanHarian = append(pemasukanHarian, map[string]interface{}{
			"tanggal":   tanggal,
			"pemasukan": total,
		})
	}

	// 🔹 Ambil semua pengeluaran user + other_transaction
	pengeluaranMap := make(map[string]float64)

	// dari user_transactions
	pengeluaranRows, err := DB.Query(context.Background(), `
		SELECT tanggal_update::date AS tanggal_update, pengeluaran
		FROM user_transactions
		WHERE user_id = $1 AND room_id = $2 AND pengeluaran > 0
		ORDER BY tanggal_update ASC
	`, userID, roomID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Gagal ambil pengeluaran: %v", err), http.StatusInternalServerError)
		return
	}
	defer pengeluaranRows.Close()

	for pengeluaranRows.Next() {
		var tanggal time.Time
		var amount float64
		if err := pengeluaranRows.Scan(&tanggal, &amount); err == nil {
			key := tanggal.Format("2006-01-02")
			pengeluaranMap[key] += amount
		}
	}

	// dari other_transaction
	otherPengeluaranRows, err := DB.Query(context.Background(), `
		SELECT tanggal_update::date AS tanggal_update, nominal
		FROM other_transaction
		WHERE user_id = $1 AND room_id = $2 AND jenis = 'Pengeluaran'
		ORDER BY tanggal_update ASC
	`, userID, roomID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Gagal ambil pengeluaran lain: %v", err), http.StatusInternalServerError)
		return
	}
	defer otherPengeluaranRows.Close()

	for otherPengeluaranRows.Next() {
		var tanggal time.Time
		var nominal float64
		if err := otherPengeluaranRows.Scan(&tanggal, &nominal); err == nil {
			key := tanggal.Format("2006-01-02")
			pengeluaranMap[key] += nominal
		}
	}

	// ubah map jadi slice
	var pengeluaranHarian []map[string]interface{}
	for tanggal, total := range pengeluaranMap {
		pengeluaranHarian = append(pengeluaranHarian, map[string]interface{}{
			"tanggal":     tanggal,
			"pengeluaran": total,
		})
	}

	// 🔹 Susun respon JSON
	response := map[string]interface{}{
		"message":                 "Login berhasil",
		"user_id":                 userID,
		"user_name":               fullName,
		"email":                   email,
		"room_id":                 roomID,
		"room_name":               roomName,
		"tanggal_buat_room":       tanggalBuatRoom.Format("2006-01-02"),
		"members":                 strings.Split(membersStr, ", "),
		"time":                    time.Now(),
		"pemasukan_harian":        pemasukanHarian,
		"pengeluaran_harian":      pengeluaranHarian,
		"total_pemasukan_room":    totalPemasukanRoom,
		"total_pengeluaran_room":  totalPengeluaranRoom,
		"total_room_saldo":        totalSaldoRoom,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}




func GetRoomsHandler(w http.ResponseWriter, r *http.Request) {
	query := `
	SELECT 
		r.id, 
		r.room_name, 
		r.created_at,
		COUNT(u.id) AS user_count
	FROM rooms r
	LEFT JOIN users u ON u.room_id = r.id
	GROUP BY r.id, r.room_name, r.created_at
	ORDER BY r.id;
	`

	rows, err := DB.Query(context.Background(), query)
	if err != nil {
		http.Error(w, "Gagal mengambil data room: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var rooms []map[string]interface{}

	for rows.Next() {
		var (
			id         int
			roomName   string
			createdAt  time.Time
			userCount  int
		)

		err := rows.Scan(&id, &roomName, &createdAt, &userCount)
		if err != nil {
			http.Error(w, "Gagal membaca data: "+err.Error(), http.StatusInternalServerError)
			return
		}

		status := "empty"
		if userCount >= 2 {
			status = "max"
		}

		rooms = append(rooms, map[string]interface{}{
			"id":          id,
			"room_name":   roomName,
			"created_at":  createdAt,
			"user_count":  userCount,
			"status":      status,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rooms)
}
