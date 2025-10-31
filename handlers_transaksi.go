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

// ==========================
// 🔹 API: Hapus Data user_transactions berdasarkan id
// ==========================
func HapusTransaksiByID(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodDelete {
        http.Error(w, "Hanya DELETE method yang diizinkan", http.StatusMethodNotAllowed)
        return
    }

    type DeleteRequest struct {
        ID int `json:"id"`
    }

    var req DeleteRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "JSON tidak valid", http.StatusBadRequest)
        return
    }

    if req.ID == 0 {
        http.Error(w, "id wajib diisi", http.StatusBadRequest)
        return
    }

    // 🔹 Ambil total pemasukan dan pengeluaran sebelum dihapus
    var pemasukan, pengeluaran float64
    var roomID int
    err := DB.QueryRow(context.Background(),
        `SELECT pemasukan, pengeluaran, room_id 
         FROM user_transactions WHERE id = $1`, req.ID).
        Scan(&pemasukan, &pengeluaran, &roomID)
    if err != nil {
        http.Error(w, "Data transaksi tidak ditemukan: "+err.Error(), http.StatusNotFound)
        return
    }

    // 🔹 Hapus transaksi
    _, err = DB.Exec(context.Background(), `DELETE FROM user_transactions WHERE id = $1`, req.ID)
    if err != nil {
        http.Error(w, "Gagal menghapus data: "+err.Error(), http.StatusInternalServerError)
        return
    }

    // 🔹 Hitung penyesuaian saldo
    saldoPenyesuaian := pemasukan - pengeluaran

    // 🔹 Update saldo di room_balance
    _, err = DB.Exec(context.Background(),
        `UPDATE room_balance 
         SET total_saldo = total_saldo - $1, tanggal_update = NOW()
         WHERE room_id = $2`, saldoPenyesuaian, roomID)
    if err != nil {
        http.Error(w, "Gagal memperbarui total saldo: "+err.Error(), http.StatusInternalServerError)
        return
    }

    // 🔹 Respon sukses
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "status":           "success",
        "message":          "Transaksi berhasil dihapus",
        "id":               req.ID,
        "pemasukan":        pemasukan,
        "pengeluaran":      pengeluaran,
        "saldo_dikurangi":  saldoPenyesuaian,
    })
}


// ==========================
// 🔹 API: Edit Data user_transactions berdasarkan id
// ==========================
func EditTransaksiByID(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPut {
        http.Error(w, "Hanya PUT method yang diizinkan", http.StatusMethodNotAllowed)
        return
    }

    type EditRequest struct {
        ID          int      `json:"id"`
        Pemasukan   *float64 `json:"pemasukan,omitempty"`
        Pengeluaran *float64 `json:"pengeluaran,omitempty"`
    }

    var req EditRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "JSON tidak valid", http.StatusBadRequest)
        return
    }

    // 🔹 Validasi input wajib
    if req.ID == 0 {
        http.Error(w, "id wajib diisi", http.StatusBadRequest)
        return
    }

    // 🔹 Pastikan hanya salah satu dari pemasukan atau pengeluaran yang diisi
    if (req.Pemasukan == nil && req.Pengeluaran == nil) || (req.Pemasukan != nil && req.Pengeluaran != nil) {
        http.Error(w, "Hanya boleh mengisi salah satu field: pemasukan ATAU pengeluaran", http.StatusBadRequest)
        return
    }

    // 🔹 Ambil data lama
    var oldPemasukan, oldPengeluaran float64
    var roomID int
    err := DB.QueryRow(context.Background(),
        `SELECT pemasukan, pengeluaran, room_id FROM user_transactions WHERE id = $1`, req.ID).
        Scan(&oldPemasukan, &oldPengeluaran, &roomID)
    if err != nil {
        http.Error(w, "Transaksi tidak ditemukan: "+err.Error(), http.StatusNotFound)
        return
    }

    // 🔹 Validasi: pastikan field yang mau diubah sudah ada datanya sebelumnya
    if req.Pemasukan != nil && oldPemasukan == 0 {
        http.Error(w, "Tidak bisa melakukan perubahan karena data pemasukan belum ada transaksi sebelumnya", http.StatusBadRequest)
        return
    }
    if req.Pengeluaran != nil && oldPengeluaran == 0 {
        http.Error(w, "Tidak bisa melakukan perubahan karena data pengeluaran belum ada transaksi sebelumnya", http.StatusBadRequest)
        return
    }

    // 🔹 Buat query update dinamis
    setClauses := []string{}
    args := []interface{}{}
    argIdx := 1

    if req.Pemasukan != nil {
        setClauses = append(setClauses, fmt.Sprintf("pemasukan = $%d", argIdx))
        args = append(args, *req.Pemasukan)
        argIdx++
    }
    if req.Pengeluaran != nil {
        setClauses = append(setClauses, fmt.Sprintf("pengeluaran = $%d", argIdx))
        args = append(args, *req.Pengeluaran)
        argIdx++
    }

    setClauses = append(setClauses, "tanggal_update = NOW()")
    args = append(args, req.ID)

    query := fmt.Sprintf(`UPDATE user_transactions SET %s WHERE id = $%d`,
        strings.Join(setClauses, ", "), argIdx)

    // 🔹 Jalankan update transaksi
    _, err = DB.Exec(context.Background(), query, args...)
    if err != nil {
        http.Error(w, "Gagal memperbarui transaksi: "+err.Error(), http.StatusInternalServerError)
        return
    }

    // 🔹 Hitung perubahan saldo
    var newPemasukan = oldPemasukan
    var newPengeluaran = oldPengeluaran
    if req.Pemasukan != nil {
        newPemasukan = *req.Pemasukan
    }
    if req.Pengeluaran != nil {
        newPengeluaran = *req.Pengeluaran
    }

    perubahanSaldo := (newPemasukan - newPengeluaran) - (oldPemasukan - oldPengeluaran)

    // 🔹 Update saldo di tabel room_balance
    _, err = DB.Exec(context.Background(),
        `UPDATE room_balance 
         SET total_saldo = total_saldo + $1, tanggal_update = NOW()
         WHERE room_id = $2`, perubahanSaldo, roomID)
    if err != nil {
        http.Error(w, "Gagal memperbarui saldo: "+err.Error(), http.StatusInternalServerError)
        return
    }

    // 🔹 Kirim respon sukses
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "status":           "success",
        "message":          "Transaksi berhasil diperbarui",
        "id":               req.ID,
        "pemasukan_lama":   oldPemasukan,
        "pengeluaran_lama": oldPengeluaran,
        "pemasukan_baru":   newPemasukan,
        "pengeluaran_baru": newPengeluaran,
        "perubahan_saldo":  perubahanSaldo,
    })
}

// ==========================
// 🔹 API: Ambil Data other_transaction berdasarkan room_id
// ==========================
func GetOtherTransactionsByRoom(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Hanya GET method yang diizinkan", http.StatusMethodNotAllowed)
		return
	}

	roomID := r.URL.Query().Get("room_id")
	if roomID == "" {
		http.Error(w, "room_id wajib diisi", http.StatusBadRequest)
		return
	}

	query := `SELECT id, user_id, room_id, jenis, kategori, nominal, keterangan, tanggal_update 
			  FROM other_transaction 
			  WHERE room_id = $1 
			  ORDER BY tanggal_update DESC`
	rows, err := DB.Query(context.Background(), query, roomID)
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
		if err := rows.Scan(&t.ID, &t.UserID, &t.RoomID, &t.Jenis, &t.Kategori, &t.Nominal, &t.Keterangan, &t.Tanggal); err != nil {
			http.Error(w, "Gagal membaca data: "+err.Error(), http.StatusInternalServerError)
			return
		}
		transaksiList = append(transaksiList, t)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "success",
		"total_data": len(transaksiList),
		"data":       transaksiList,
	})
}

func GetAllTransactionsByRoom(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Hanya GET method yang diizinkan", http.StatusMethodNotAllowed)
		return
	}

	roomID := r.URL.Query().Get("room_id")
	if roomID == "" {
		http.Error(w, "room_id wajib diisi", http.StatusBadRequest)
		return
	}

	type Transaction struct {
		ID          int       `json:"id"`
		UserID      int       `json:"user_id"`
		RoomID      int       `json:"room_id"`
		Pemasukan   float64   `json:"pemasukan,omitempty"`
		Pengeluaran float64   `json:"pengeluaran,omitempty"`
		Jenis       string    `json:"jenis,omitempty"`
		Kategori    string    `json:"kategori,omitempty"`
		Nominal     float64   `json:"nominal,omitempty"`
		Keterangan  string    `json:"keterangan,omitempty"`
		Tanggal     time.Time `json:"tanggal_update"`
		Source      string    `json:"source"` // new field: "user" or "other"
	}

	var transaksiList []Transaction

	// --- Query user_transactions ---
	rows, err := DB.Query(context.Background(), `
		SELECT id, user_id, room_id, pemasukan, pengeluaran, tanggal_update 
		FROM user_transactions WHERE room_id=$1 ORDER BY tanggal_update DESC
	`, roomID)
	if err != nil {
		http.Error(w, "Gagal mengambil user_transactions: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var t Transaction
		if err := rows.Scan(&t.ID, &t.UserID, &t.RoomID, &t.Pemasukan, &t.Pengeluaran, &t.Tanggal); err != nil {
			http.Error(w, "Gagal membaca user_transactions: "+err.Error(), http.StatusInternalServerError)
			return
		}
		t.Source = "user"
		transaksiList = append(transaksiList, t)
	}

	// --- Query other_transaction ---
	rows2, err := DB.Query(context.Background(), `
		SELECT id, user_id, room_id, jenis, kategori, nominal, keterangan, tanggal_update
		FROM other_transaction WHERE room_id=$1 ORDER BY tanggal_update DESC
	`, roomID)
	if err != nil {
		http.Error(w, "Gagal mengambil other_transaction: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows2.Close()

	for rows2.Next() {
		var t Transaction
		if err := rows2.Scan(&t.ID, &t.UserID, &t.RoomID, &t.Jenis, &t.Kategori, &t.Nominal, &t.Keterangan, &t.Tanggal); err != nil {
			http.Error(w, "Gagal membaca other_transaction: "+err.Error(), http.StatusInternalServerError)
			return
		}
		t.Source = "other"
		transaksiList = append(transaksiList, t)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "success",
		"total_data": len(transaksiList),
		"data":       transaksiList,
	})
}

