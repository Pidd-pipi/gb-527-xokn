package service

import (
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"satellite-contact-window-deconfliction/backend/internal/dto"
	"satellite-contact-window-deconfliction/backend/internal/model"
	"satellite-contact-window-deconfliction/backend/internal/repository"
)

func TestParseTokenUsesCurrentActiveUserAndRole(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:auth-current-user-527?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("migrate user: %v", err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("password-527"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	user := model.User{Username: "scheduler", PasswordHash: string(hash), Role: "scheduler", Active: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	auth := NewAuthService(repository.NewSystemRepository(db), "test-secret", time.Hour)
	login, err := auth.Login(dto.LoginRequest{Username: user.Username, Password: "password-527"})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if err := db.Model(&model.User{}).Where("id = ?", user.ID).Updates(map[string]any{"role": "reviewer", "active": true}).Error; err != nil {
		t.Fatalf("change role: %v", err)
	}
	actor, err := auth.ParseToken(login.Token)
	if err != nil {
		t.Fatalf("parse token after role change: %v", err)
	}
	if actor.Role != "reviewer" {
		t.Fatalf("actor role = %q, want current reviewer role", actor.Role)
	}
	if err := db.Model(&model.User{}).Where("id = ?", user.ID).Update("active", false).Error; err != nil {
		t.Fatalf("disable user: %v", err)
	}
	if _, err := auth.ParseToken(login.Token); err == nil {
		t.Fatal("disabled user token remained valid")
	}
}
