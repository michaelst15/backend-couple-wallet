package main

type RegisterRequest struct {
	FullName        string `json:"full_name"`
	Email           string `json:"email"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
	RoomID          int    `json:"room_id"`
}

type User struct {
	ID        int    `json:"id"`
	FullName  string `json:"full_name"`
	Email     string `json:"email"`
	RoomID    int    `json:"room_id"`
	CreatedAt string `json:"created_at"`
}
