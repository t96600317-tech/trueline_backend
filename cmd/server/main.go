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

	"trueline-backend/internal/admin"
	"trueline-backend/internal/auth"
	"trueline-backend/internal/calls"
	"trueline-backend/internal/chat"
	"trueline-backend/internal/config"
	"trueline-backend/internal/db"
	"trueline-backend/internal/earnings"
	"trueline-backend/internal/httpx"
	"trueline-backend/internal/kyc"
	"trueline-backend/internal/listeners"
	"trueline-backend/internal/payments"
	"trueline-backend/internal/payouts"
	"trueline-backend/internal/user"
	"trueline-backend/internal/wallet"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	log.Println("Starting TrueLine Backend Service (Pilot V1 Mode)...")

	// 1. Load & Validate Configuration
	cfg := config.LoadConfig()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}

	// 2. Connect to Supabase Postgres DB
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	database, err := db.ConnectSupabaseDB(ctx, cfg.DatabaseURL)
	var dbPool *pgxpool.Pool
	if err != nil {
		if cfg.Env != "development" && cfg.Env != "test" {
			log.Fatalf("Fatal: Database connection failed in %s environment: %v", cfg.Env, err)
		}
		log.Printf("Warning: Failed to connect to Supabase DB: %v (continuing in development mode)", err)
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
	if (cfg.MSG91WidgetID != "" && cfg.MSG91WidgetAuthToken != "") ||
		(cfg.MSG91CustomerWidgetID != "" && cfg.MSG91CustomerWidgetAuthToken != "") ||
		(cfg.MSG91ListenerWidgetID != "" && cfg.MSG91ListenerWidgetAuthToken != "") ||
		cfg.MSG91ServerAuthKey != "" ||
		cfg.MSG91CustomerServerAuthKey != "" ||
		cfg.MSG91ListenerServerAuthKey != "" {
		log.Println("Using MSG91 Widget verification for mobile SDK OTP login")
	}

	authService := auth.NewAuthService(dbPool, tokenManager, otpProvider, cfg)
	authHandler := auth.NewAuthHandler(authService)

	// 4. Initialize Core Services
	userService := user.NewUserService(dbPool)
	userHandler := user.NewUserHandler(userService)

	listenerService := listeners.NewListenerService(dbPool)
	listenerHandler := listeners.NewListenerHandler(listenerService)

	var secureIDProvider kyc.SecureIDProvider
	if cfg.CashfreeClientID != "" && cfg.CashfreeClientSecret != "" {
		secureIDProvider = kyc.NewCashfreeSecureID(cfg.CashfreeClientID, cfg.CashfreeClientSecret, cfg.CashfreeSandbox)
		log.Println("Using Cashfree Secure ID verification provider")
	} else {
		secureIDProvider = kyc.NewMockSecureIDProvider()
		log.Println("Using Mock Secure ID verification provider (Dev/Offline)")
	}
	kycService := kyc.NewKYCService(dbPool, secureIDProvider)
	kycHandler := kyc.NewKYCHandler(kycService)

	walletService := wallet.NewWalletService(dbPool)

	cfPGClient := payments.NewCashfreePGClient(cfg.CashfreeClientID, cfg.CashfreeClientSecret, cfg.CashfreeWebhookKey, cfg.CashfreeSandbox)
	paymentService := payments.NewPaymentService(dbPool, walletService, cfPGClient)
	paymentHandler := payments.NewPaymentHandler(paymentService, cfPGClient)

	cfPayoutsClient := payouts.NewCashfreePayoutsClient(cfg.CashfreeClientID, cfg.CashfreeClientSecret, cfg.CashfreeSandbox)
	payoutService := payouts.NewPayoutService(dbPool, cfPayoutsClient)
	payoutHandler := payouts.NewPayoutHandler(payoutService)

	earningsService := earnings.NewEarningsService(dbPool)

	// 5. Calling & Metering
	zegoTokenProvider := calls.NewZegoTokenProvider(cfg.ZegoAppID, cfg.ZegoServerSecret)
	var incomingCallNotifier calls.IncomingCallNotifier
	if cfg.APNsTeamID != "" || cfg.APNsKeyID != "" || cfg.APNsBundleID != "" || cfg.APNsPrivateKey != "" {
		if err := calls.EnsureIOSVoIPDeviceStore(ctx, dbPool); err != nil {
			log.Printf("APNs VoIP notifications disabled: %v", err)
		} else {
			notifier, err := calls.NewAPNsVoIPNotifier(
				dbPool,
				cfg.APNsTeamID,
				cfg.APNsKeyID,
				cfg.APNsBundleID,
				cfg.APNsPrivateKey,
				cfg.APNsSandbox,
			)
			if err != nil {
				log.Printf("APNs VoIP notifications disabled: %v", err)
			} else {
				incomingCallNotifier = notifier
				log.Println("APNs VoIP incoming-call notifications enabled")
			}
		}
	}
	callService := calls.NewCallService(dbPool, zegoTokenProvider, walletService, incomingCallNotifier)
	eventHub := calls.NewEventHub()
	callHandler := calls.NewCallHandler(callService, eventHub, tokenManager)

	meteringEngine := calls.NewMeteringEngine(dbPool, walletService, earningsService, callService, eventHub)
	go meteringEngine.Start(ctx)

	adminService := admin.NewAdminService(dbPool, tokenManager, payoutService)
	adminHandler := admin.NewAdminHandler(adminService)

	chatService := chat.NewChatService(dbPool)
	chatHandler := chat.NewChatHandler(chatService)

	// 6. Build HTTP Router
	router := httpx.NewRouter(
		authHandler,
		userHandler,
		listenerHandler,
		kycHandler,
		paymentHandler,
		payoutHandler,
		adminHandler,
		callHandler,
		chatHandler,
		tokenManager,
	)

	// 7. Start Server
	serverAddr := fmt.Sprintf(":%s", cfg.Port)
	srv := &http.Server{
		Addr:         serverAddr,
		Handler:      httpx.RequestLoggerMiddleware(httpx.CORSMiddleware(router)),
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
	cancel() // Stop metering engine

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("TrueLine Backend stopped cleanly.")
}
