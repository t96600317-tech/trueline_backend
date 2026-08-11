package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"trueline-backend/internal/auth"
	"trueline-backend/internal/chat"
	"trueline-backend/internal/config"
	"trueline-backend/internal/db"
	"trueline-backend/internal/httpx"
	"trueline-backend/internal/user"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	log.Println("Starting TrueLine Backend Service (User App Mode)...")

	// 1. Load Configuration
	cfg := config.LoadConfig()

	// 2. Connect to Supabase Postgres DB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	database, err := db.ConnectSupabaseDB(ctx, cfg.DatabaseURL)
	var dbPool *pgxpool.Pool
	if err != nil {
		log.Printf("Warning: Failed to connect to Supabase DB: %v (continuing with server init)", err)
	} else {
		dbPool = database.Pool
		defer database.Close()
	}

	// 3. Initialize Auth & OTP Provider
	tokenManager := auth.NewTokenManager(cfg.JWTSecret)

	var otpProvider auth.OTPProvider
	switch cfg.OTPProvider {
	case "twilio":
		otpProvider = auth.NewTwilioOTPProvider(cfg.TwilioAccountSID, cfg.TwilioAuthToken, cfg.TwilioFromPhone)
		log.Println("Using Twilio SMS OTP Provider")
	case "msg91":
		otpProvider = auth.NewMSG91OTPProvider(cfg.MSG91AuthKey, cfg.MSG91TemplateID)
		log.Println("Using MSG91 SMS OTP Provider (India Market)")
	default:
		otpProvider = auth.NewMockOTPProvider()
		log.Println("Using Mock SMS OTP Provider (Development)")
	}

	authService := auth.NewAuthService(dbPool, tokenManager, otpProvider, cfg)
	authHandler := auth.NewAuthHandler(authService)

	// 4. Initialize User & Chat Services
	userService := user.NewUserService(dbPool)
	userHandler := user.NewUserHandler(userService)

	chatService := chat.NewChatService(dbPool)
	chatHandler := chat.NewChatHandler(chatService)

	// 5. Build HTTP Router
	router := httpx.NewRouter(authHandler, userHandler, chatHandler, tokenManager)

	// 6. Start Server
	serverAddr := fmt.Sprintf(":%s", cfg.Port)
	srv := &http.Server{
		Addr:         serverAddr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("TrueLine Backend listening on HTTP port %s [ENV: %s]", cfg.Port, cfg.Env)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down TrueLine Backend service gracefully...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("TrueLine Backend stopped cleanly.")
}
