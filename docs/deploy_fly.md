# TrueLine Backend — Fly.io Deployment Guide

This guide details how to deploy the **TrueLine Backend** to **Fly.io** in the Mumbai (`bom`) region.

---

## 1. Step 1: Launch Application on Fly.io

Before setting secrets or deploying, initialize the app registration with Fly.io:

```bash
fly launch --no-deploy
```

- When prompted to overwrite `fly.toml`, select **`N` (No)** so it retains our pre-configured [`fly.toml`](file:///home/cosmic/Projects/TrueLine_Backend/fly.toml).
- If `trueline-backend` is already taken as an app name on Fly.io, `fly launch` will let you specify a unique name (e.g. `trueline-backend-prod`).

---

## 2. Step 2: Set Environment Secrets

Set your database and auth secrets using `fly secrets set`:

```bash
fly secrets set \
  DATABASE_URL="postgres://postgres.[ref]:[PASSWORD]@aws-0-ap-south-1.pooler.supabase.com:6543/postgres?sslmode=require" \
  JWT_SECRET="your_production_secure_jwt_secret_key" \
  OTP_MOCK_MODE="false" \
  OTP_PROVIDER="mock" \
  MSG91_SERVER_AUTH_KEY="your-msg91-server-auth-key"
```

The Android widget ID and mobile integration tokens stay in each app's ignored
`local.properties`. The server auth key is the only MSG91 credential required
by the backend; never place it in `fly.toml`.

---

## 3. Step 3: Deploy Application

Run the deploy command from the repository root:
```bash
fly deploy
```

Fly.io will build the container using [`Dockerfile`](file:///home/cosmic/Projects/TrueLine_Backend/Dockerfile), launch the machine in the Mumbai (`bom`) region, and perform health checks on `/healthz`.

---

## 4. Useful Operations Commands

- **View Live Logs:**
  ```bash
  fly logs
  ```
- **Check Application Health:**
  ```bash
  curl https://trueline-backend.fly.dev/healthz
  ```
