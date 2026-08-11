# TrueLine Backend — API Updates & Mobile App Integration Guide

**Document Version:** 1.1 (User App Release)  
**Target Audience:** Mobile App Development Team (Android + iOS)  
**Base Server URL:** `http://localhost:8080/api/v1`

---

## 🚀 What's Included in This Update

This update delivers the complete backend APIs required to power the **TrueLine User App**:

1. **Phone Number + OTP Authentication Flow** (No name or email required for users).
2. **User Profile & Header Balance** (Displaying coin/rupee balance in header e.g. `260` coins).
3. **Home Screen (Discover Active Listeners)** matching your design UI:
   - Profile photo, Listener Name, Title/Tagline (*"Joy Helper"*, *"Calm Friend"*), Languages, Voice Audio Intro preview, Call Rate/min, Star Rating, Online/Offline badge, and Favourites status.
   - Search filter by listener name.
   - Language filter chips (*Hindi*, *Bhojpuri*, *Bengali*, *Tamil*, *Urdu*, *English*).
4. **1-on-1 Chat System**:
   - Chat Tab: List all conversations with partner avatar, name, online badge, last message snippet, timestamp, and unread count badge.
   - Chat Room: 1-on-1 message history, send messages, and auto-read tracking.
5. **Database Pre-seeded Test Data** (Includes pre-seeded test listeners matching UI designs and test coin balance).

---

## 🔑 1. Authentication & Onboarding Integration

All protected endpoints require the HTTP header:
```http
Authorization: Bearer <JWT_TOKEN>
```

### Step 1: Request OTP
- **Endpoint:** `POST /api/v1/auth/otp/request`
- **Request Body:**
```json
{
  "phone": "+919876543210",
  "role": "user"
}
```
- **Response (200 OK):**
```json
{
  "success": true,
  "data": {
    "message": "OTP sent successfully",
    "phone": "+919876543210",
    "expires_in_seconds": 300,
    "mock_otp": "123456" // Use '123456' for development testing!
  }
}
```

### Step 2: Verify OTP & Receive JWT Token
- **Endpoint:** `POST /api/v1/auth/otp/verify`
- **Request Body:**
```json
{
  "phone": "+919876543210",
  "otp": "123456",
  "role": "user"
}
```
- **Response (200 OK):**
```json
{
  "success": true,
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "role": "user",
    "is_new_user": true,
    "user": {
      "id": "11111111-1111-1111-1111-111111111111",
      "phone": "+919876543210",
      "language_pref": "hi",
      "status": "active"
    }
  }
}
```
> **App Action:** Store `token` securely in device Keychain / EncryptedSharedPreferences.

---

## 👤 2. User Profile & Header Coin Balance

### Fetch User Profile & Coin Balance
- **Endpoint:** `GET /api/v1/user/me`
- **Auth:** `Bearer <USER_JWT_TOKEN>`
- **Response (200 OK):**
```json
{
  "success": true,
  "data": {
    "user": {
      "id": "11111111-1111-1111-1111-111111111111",
      "phone": "+919876543210",
      "language_pref": "hi",
      "status": "active"
    },
    "balance": 260.00 // Populate the top-right header coin pill (🪙 260)
  }
}
```

---

## 🏠 3. Home Screen (Discover Active Listeners) Integration

### Fetch Listeners List
- **Endpoint:** `GET /api/v1/partners`
- **Auth:** `Bearer <USER_JWT_TOKEN>`
- **Query Parameters:**
  - `language` (Optional): `All`, `Hindi`, `Bhojpuri`, `Bengali`, `Tamil`, `Urdu`, `English`
  - `search` (Optional): Query text typed in top search bar (e.g. `Afreen`)
- **Example Requests:**
  - All Listeners: `GET /api/v1/partners`
  - Hindi Listeners: `GET /api/v1/partners?language=Hindi`
  - Search Name: `GET /api/v1/partners?search=Ahmedi`

- **Response Payload (200 OK):**
```json
{
  "success": true,
  "data": [
    {
      "id": "a0000000-0000-0000-0000-000000000001",
      "name": "Afreen",
      "title": "Joy Helper",
      "photo_url": "https://images.unsplash.com/photo-1534528741775-53994a69daeb?auto=format&fit=crop&w=400&q=80",
      "audio_sample_url": "https://actions.google.com/sounds/v1/ambiences/rain_heavy.ogg",
      "bio": "Always here to listen and bring joy to your day.",
      "languages": ["Hindi", "Bengali"],
      "rate_per_min": 11.00,
      "rating_avg": 4.50,
      "rating_count": 38,
      "availability": "online", // Show green ONLINE badge when "online"
      "is_favourite": true
    },
    {
      "id": "a0000000-0000-0000-0000-000000000002",
      "name": "Ahmedi",
      "title": "Calm Friend",
      "photo_url": "https://images.unsplash.com/photo-1517841905240-472988babdf9?auto=format&fit=crop&w=400&q=80",
      "audio_sample_url": "https://actions.google.com/sounds/v1/ambiences/rain_heavy.ogg",
      "bio": "A calm and patient listener for work and personal chats.",
      "languages": ["Urdu", "Hindi"],
      "rate_per_min": 11.00,
      "rating_avg": 4.80,
      "rating_count": 54,
      "availability": "online",
      "is_favourite": false
    }
  ]
}
```

---

## 💬 4. 1-on-1 Chat System Integration

### 4.1 Chat Tab (List All Conversations Screen)
Populates the bottom-navigation Chat Tab listing all active chats.

- **Endpoint:** `GET /api/v1/chats`
- **Auth:** `Bearer <USER_JWT_TOKEN>`
- **Response (200 OK):**
```json
{
  "success": true,
  "data": [
    {
      "partner_id": "a0000000-0000-0000-0000-000000000001",
      "partner_name": "Afreen",
      "partner_title": "Joy Helper",
      "partner_photo_url": "https://images.unsplash.com/photo-1534528741775-53994a69daeb?auto=format&fit=crop&w=400&q=80",
      "partner_availability": "online",
      "last_message": "Haan bilkul! Feel free to call anytime.",
      "last_message_sender": "partner",
      "last_message_time": "2026-08-11T20:15:00Z",
      "unread_count": 0
    },
    {
      "partner_id": "a0000000-0000-0000-0000-000000000002",
      "partner_name": "Ahmedi",
      "partner_title": "Calm Friend",
      "partner_photo_url": "https://images.unsplash.com/photo-1517841905240-472988babdf9?auto=format&fit=crop&w=400&q=80",
      "partner_availability": "online",
      "last_message": "Hello! Hope you had a peaceful day.",
      "last_message_sender": "partner",
      "last_message_time": "2026-08-11T20:00:00Z",
      "unread_count": 1
    }
  ]
}
```

### 4.2 1-on-1 Chat Room (Message History)
Fetches messages inside a specific listener's chat room. Automatically marks unread messages from the partner as read.

- **Endpoint:** `GET /api/v1/chats/{partner_id}/messages`
- **Auth:** `Bearer <USER_JWT_TOKEN>`
- **Query Params:** `limit` (default 50), `offset` (default 0)
- **Response (200 OK):**
```json
{
  "success": true,
  "data": [
    {
      "id": "d1a2b3c4-5678-901a-bcde-f11111111111",
      "user_id": "11111111-1111-1111-1111-111111111111",
      "partner_id": "a0000000-0000-0000-0000-000000000001",
      "sender_type": "user", // "user" (bubble right) or "partner" (bubble left)
      "content": "Namaste Afreen! Are you free to talk today?",
      "read_at": "2026-08-11T19:00:00Z",
      "created_at": "2026-08-11T18:45:00Z"
    },
    {
      "id": "d2b3c4d5-6789-012a-bcde-f22222222222",
      "user_id": "11111111-1111-1111-1111-111111111111",
      "partner_id": "a0000000-0000-0000-0000-000000000001",
      "sender_type": "partner",
      "content": "Haan bilkul! Feel free to call anytime.",
      "read_at": "2026-08-11T19:05:00Z",
      "created_at": "2026-08-11T19:00:00Z"
    }
  ]
}
```

### 4.3 Send Message in 1-on-1 Chat Room
Send a text message to a listener.

- **Endpoint:** `POST /api/v1/chats/{partner_id}/messages`
- **Auth:** `Bearer <USER_JWT_TOKEN>`
- **Request Body:**
```json
{
  "content": "Hello Afreen, I am starting the call now!"
}
```
- **Response (200 OK):** Returns created `ChatMessage` object.

---

## 🛠️ 5. Quick Testing cURL Commands

Run backend server:
```bash
go run cmd/server/main.go
```

1. **Request OTP:**
   ```bash
   curl -X POST http://localhost:8080/api/v1/auth/otp/request \
     -H "Content-Type: application/json" \
     -d '{"phone": "+919876543210", "role": "user"}'
   ```

2. **Verify OTP & Get Token:**
   ```bash
   curl -X POST http://localhost:8080/api/v1/auth/otp/verify \
     -H "Content-Type: application/json" \
     -d '{"phone": "+919876543210", "otp": "123456", "role": "user"}'
   ```

3. **Get Home Screen Partners:**
   ```bash
   curl http://localhost:8080/api/v1/partners \
     -H "Authorization: Bearer <TOKEN_FROM_STEP_2>"
   ```

4. **Get Chat Conversations List:**
   ```bash
   curl http://localhost:8080/api/v1/chats \
     -H "Authorization: Bearer <TOKEN_FROM_STEP_2>"
   ```

5. **Send Chat Message:**
   ```bash
   curl -X POST http://localhost:8080/api/v1/chats/a0000000-0000-0000-0000-000000000001/messages \
     -H "Authorization: Bearer <TOKEN_FROM_STEP_2>" \
     -H "Content-Type: application/json" \
     -d '{"content": "Hello Afreen!"}'
   ```
