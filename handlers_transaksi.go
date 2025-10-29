package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	"io"
	"bytes"
)

// ===========================
// 🔹 Handler: Tambah Pemasukan
// ===========================
func Pemasukan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Hanya POST method yang diizinkan", http.StatusMethodNotAllowed)
		return
	}

	type IncomeRequest struct {
		UserID int     `json:"user_id"`
		RoomID int     `json:"room_id"`
		Amount float64 `json:"amount"`
	}

	var req IncomeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON tidak valid", http.StatusBadRequest)
		return
	}

	if req.UserID == 0 || req.RoomID == 0 || req.Amount <= 0 {
		http.Error(w, "Data tidak lengkap", http.StatusBadRequest)
		return
	}

	query := `
		INSERT INTO user_transactions (user_id, room_id, pemasukan, tanggal_update)
		VALUES ($1, $2, $3, NOW())
	`
	_, err := DB.Exec(context.Background(), query,
		req.UserID, req.RoomID, req.Amount,
	)
	if err != nil {
		http.Error(w, "Gagal menambah pemasukan: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 🔹 Ambil saldo terkini per user dalam room tsb
	summary := getUserSummary(req.RoomID, req.UserID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "success",
		"message":  "Pemasukan berhasil ditambahkan",
		"room_id":  req.RoomID,
		"user_id":  req.UserID,
		"summary":  summary,
		"datetime": time.Now().Format(time.RFC3339),
	})
}

// ============================
// 🔹 Handler: Tambah Pengeluaran
// ============================
func Pengeluaran(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Hanya POST method yang diizinkan", http.StatusMethodNotAllowed)
		return
	}

	type ExpenseRequest struct {
		UserID int     `json:"user_id"`
		RoomID int     `json:"room_id"`
		Amount float64 `json:"amount"`
	}

	var req ExpenseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON tidak valid", http.StatusBadRequest)
		return
	}

	if req.UserID == 0 || req.RoomID == 0 || req.Amount <= 0 {
		http.Error(w, "Data tidak lengkap", http.StatusBadRequest)
		return
	}

	query := `
		INSERT INTO user_transactions (user_id, room_id, pengeluaran, tanggal_update)
		VALUES ($1, $2, $3, NOW())
	`
	_, err := DB.Exec(context.Background(), query,
		req.UserID, req.RoomID, req.Amount,
	)
	if err != nil {
		http.Error(w, "Gagal menambah pengeluaran: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 🔹 Ambil saldo terkini per user dalam room tsb
	summary := getUserSummary(req.RoomID, req.UserID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "success",
		"message":  "Pengeluaran berhasil ditambahkan",
		"room_id":  req.RoomID,
		"user_id":  req.UserID,
		"summary":  summary,
		"datetime": time.Now().Format(time.RFC3339),
	})
}

// ==========================
// 🔹 Fungsi: Hitung Ringkasan Per User dalam Room
// ==========================
// ==========================
// 🔹 Fungsi: Hitung Ringkasan Room dan User
// ==========================
func getUserSummary(roomID, userID int) map[string]interface{} {
	var (
		totalPemasukanRoom   float64
		totalPengeluaranRoom float64
		totalSaldoRoom       float64
		terakhirUpdateRoom   *time.Time

		totalPemasukanUser   float64
		totalPengeluaranUser float64
		totalSaldoUser       float64
	)

	// 🔹 Ambil total untuk seluruh user di dalam room
	queryRoom := `
		SELECT 
			COALESCE(SUM(pemasukan), 0),
			COALESCE(SUM(pengeluaran), 0),
			(COALESCE(SUM(pemasukan), 0) - COALESCE(SUM(pengeluaran), 0)) AS total_saldo,
			MAX(tanggal_update)
		FROM user_transactions
		WHERE room_id = $1
	`

	err := DB.QueryRow(context.Background(), queryRoom, roomID).Scan(
		&totalPemasukanRoom,
		&totalPengeluaranRoom,
		&totalSaldoRoom,
		&terakhirUpdateRoom,
	)
	if err != nil {
		return map[string]interface{}{
			"error": fmt.Sprintf("Gagal menghitung total room: %v", err),
		}
	}

	// 🔹 Ambil total untuk user spesifik dalam room
	queryUser := `
		SELECT 
			COALESCE(SUM(pemasukan), 0),
			COALESCE(SUM(pengeluaran), 0),
			(COALESCE(SUM(pemasukan), 0) - COALESCE(SUM(pengeluaran), 0)) AS total_saldo
		FROM user_transactions
		WHERE room_id = $1 AND user_id = $2
	`

	err = DB.QueryRow(context.Background(), queryUser, roomID, userID).Scan(
		&totalPemasukanUser,
		&totalPengeluaranUser,
		&totalSaldoUser,
	)
	if err != nil {
		return map[string]interface{}{
			"error": fmt.Sprintf("Gagal menghitung saldo user: %v", err),
		}
	}

	var formattedTime any
	if terakhirUpdateRoom != nil {
		formattedTime = terakhirUpdateRoom.Format("2006-01-02 15:04:05")
	} else {
		formattedTime = nil
	}

	return map[string]interface{}{
		// 🔹 Total untuk seluruh room
		"room_summary": map[string]interface{}{
			"total_pemasukan_room":   totalPemasukanRoom,
			"total_pengeluaran_room": totalPengeluaranRoom,
			"total_saldo_room":       totalSaldoRoom,
			"terakhir_update_room":   formattedTime,
		},

		// 🔹 Total untuk user yang baru input
		"user_summary": map[string]interface{}{
			"total_pemasukan_user":   totalPemasukanUser,
			"total_pengeluaran_user": totalPengeluaranUser,
			"total_saldo_user":       totalSaldoUser,
		},
	}
}

func TambahTransaksi(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Hanya POST method yang diizinkan", http.StatusMethodNotAllowed)
        return
    }

    type TransactionRequest struct {
        UserID     int     `json:"user_id"`
        RoomID     int     `json:"room_id"`
        Jenis      string  `json:"jenis"`
        Kategori   string  `json:"kategori"`
        Nominal    float64 `json:"nominal"`
        Keterangan string  `json:"keterangan"`
    }

    bodyBytes, _ := io.ReadAll(r.Body)
    fmt.Println("RAW BODY:", string(bodyBytes))
    r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

    var req TransactionRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "JSON tidak valid", http.StatusBadRequest)
        return
    }

    jenis := strings.Title(strings.ToLower(strings.TrimSpace(req.Jenis)))
    kategori := strings.Title(strings.ToLower(strings.TrimSpace(req.Kategori)))
    validJenis := map[string]bool{"Pemasukan": true, "Pengeluaran": true}
    validKategori := map[string]bool{"Makanan": true, "Belanja": true, "Hiburan": true, "Tagihan": true, "Lainnya": true}

    if req.UserID == 0 || req.RoomID == 0 || req.Nominal <= 0 || !validJenis[jenis] || !validKategori[kategori] {
        http.Error(w, "Data tidak lengkap atau salah", http.StatusBadRequest)
        return
    }

    tx, err := DB.Begin(context.Background())
    if err != nil {
        http.Error(w, "Gagal memulai transaksi DB: "+err.Error(), http.StatusInternalServerError)
        return
    }
    defer tx.Rollback(context.Background())

    // --- Ambil saldo saat ini, buat jika belum ada ---
    var currentSaldo float64
    err = tx.QueryRow(context.Background(), "SELECT total_saldo FROM room_balance WHERE room_id=$1 FOR UPDATE", req.RoomID).Scan(&currentSaldo)
    if err != nil {
        // Jika tidak ada record → buat baru
        if strings.Contains(err.Error(), "no rows in result set") {
            _, err := tx.Exec(context.Background(), "INSERT INTO room_balance (room_id, total_saldo) VALUES ($1, 0)", req.RoomID)
            if err != nil {
                http.Error(w, "Gagal membuat saldo baru: "+err.Error(), http.StatusInternalServerError)
                return
            }
            currentSaldo = 0
        } else {
            http.Error(w, "Gagal membaca saldo: "+err.Error(), http.StatusInternalServerError)
            return
        }
    }

    // Cek saldo cukup untuk pengeluaran
    if jenis == "Pengeluaran" && currentSaldo < req.Nominal {
        http.Error(w, "Saldo Tidak Cukup", http.StatusBadRequest)
        return
    }

    // Insert transaksi
    insertQuery := `
        INSERT INTO other_transaction (user_id, room_id, jenis, kategori, nominal, keterangan, tanggal_update)
        VALUES ($1, $2, $3, $4, $5, $6, NOW())
    `
    _, err = tx.Exec(context.Background(), insertQuery, req.UserID, req.RoomID, jenis, kategori, req.Nominal, req.Keterangan)
    if err != nil {
        http.Error(w, "Gagal menambah transaksi: "+err.Error(), http.StatusInternalServerError)
        return
    }

    // Update saldo
    if jenis == "Pemasukan" {
        currentSaldo += req.Nominal
    } else if jenis == "Pengeluaran" {
        currentSaldo -= req.Nominal
    }

    _, err = tx.Exec(context.Background(), "UPDATE room_balance SET total_saldo=$1 WHERE room_id=$2", currentSaldo, req.RoomID)
    if err != nil {
        http.Error(w, "Gagal update saldo: "+err.Error(), http.StatusInternalServerError)
        return
    }

    if err := tx.Commit(context.Background()); err != nil {
        http.Error(w, "Gagal commit transaksi: "+err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "status":      "success",
        "message":     "Transaksi berhasil ditambahkan",
        "user_id":     req.UserID,
        "room_id":     req.RoomID,
        "jenis":       jenis,
        "kategori":    kategori,
        "nominal":     req.Nominal,
        "keterangan":  req.Keterangan,
        "total_saldo": currentSaldo,
        "datetime":    time.Now().Format(time.RFC3339),
    })
}



func GetTransaksi(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "Hanya GET method yang diizinkan", http.StatusMethodNotAllowed)
        return
    }

    // Optional: filter by query param
    userID := r.URL.Query().Get("user_id")
    roomID := r.URL.Query().Get("room_id")

    query := "SELECT id, user_id, room_id, jenis, kategori, nominal, keterangan, tanggal_update FROM other_transaction"
    var args []interface{}
    conditions := []string{}

    if userID != "" {
        conditions = append(conditions, "user_id = $"+fmt.Sprint(len(args)+1))
        args = append(args, userID)
    }
    if roomID != "" {
        conditions = append(conditions, "room_id = $"+fmt.Sprint(len(args)+1))
        args = append(args, roomID)
    }

    if len(conditions) > 0 {
        query += " WHERE " + strings.Join(conditions, " AND ")
    }

    query += " ORDER BY tanggal_update DESC"

    rows, err := DB.Query(context.Background(), query, args...)
    if err != nil {
        http.Error(w, "Gagal mengambil data: "+err.Error(), http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    type Transaction struct {
        ID         int       `json:"id"`
        UserID     int       `json:"user_id"`
        RoomID     int       `json:"room_id"`
        Jenis      string    `json:"jenis"`
        Kategori   string    `json:"kategori"`
        Nominal    float64   `json:"nominal"`
        Keterangan string    `json:"keterangan"`
        Tanggal    time.Time `json:"tanggal_update"`
    }

    var transaksiList []Transaction
    for rows.Next() {
        var t Transaction
        err := rows.Scan(&t.ID, &t.UserID, &t.RoomID, &t.Jenis, &t.Kategori, &t.Nominal, &t.Keterangan, &t.Tanggal)
        if err != nil {
            http.Error(w, "Gagal membaca data: "+err.Error(), http.StatusInternalServerError)
            return
        }
        transaksiList = append(transaksiList, t)
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "status":      "success",
        "total_data":  len(transaksiList),
        "transaksi":   transaksiList,
    })
}

