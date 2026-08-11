# TrueLine — User App API Integration Guide (v1)

This document is the official API specification for the Mobile App Engineering Team building the **TrueLine User App** (iOS + Android).

---

## 1. Base Conventions

- **Production Base URL:** `https://api.truelineapp.in/api/v1`
- **Development Base URL:** `http://localhost:8080/api/v1`
- **Protocol:** HTTPS REST (JSON body)
- **Authentication Header:** `Authorization: Bearer <JWT_TOKEN>`
- **Standard Response Format:**
```json
{
  "success": true,
  "data": { ... },
  "error": null
}
```
- **Standard Error Response Format:**
```json
{
  "success": false,
  "data": null,
  "error": {
    "code": "INVALID_TOKEN",
    "message": "Authorization token is invalid or expired"
  }
}
```

---

## 2. Authentication & User Onboarding Flow

Users log in with **Phone Number + OTP** only. **No name, email, or password is required.**

### 2.1 Request OTP
Send a 6-digit OTP code to the provided phone number.

- **Endpoint:** `POST /api/v1/auth/otp/request`
- **Auth:** Public
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
    "mock_otp": "123456" // Returned in development mode for easy app testing!
  }
}
```

### 2.2 Verify OTP
Verify the OTP code and receive a JWT Session Token.

- **Endpoint:** `POST /api/v1/auth/otp/verify`
- **Auth:** Public
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
      "id": "a3b1c2d3-4567-890a-bcde-f12345678901",
      "phone": "+919876543210",
      "language_pref": "hi",
      "status": "active",
      "created_at": "2026-08-11T20:00:00Z"
    }
  }
}
```

---

## 3. User Profile & Wallet Header

### 3.1 Get Profile & Coin Balance Header
Fetch current profile info and coin balance (e.g. `260` coins).

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
    "balance": 260.00
  }
}
```

### 3.2 Update Language Preference
- **Endpoint:** `PATCH /api/v1/user/language`
- **Auth:** `Bearer <USER_JWT_TOKEN>`
- **Request Body:** `{ "language": "hi" }`

---

## 4. Home Screen (Discover Listeners)

Fetch active listeners matching the app UI layout (Name, Title tag, Profile Photo, Voice Audio Intro, Languages, Rate/min, Star Rating, Online/Offline badge, and Favourites status).

- **Endpoint:** `GET /api/v1/partners`
- **Auth:** `Bearer <USER_JWT_TOKEN>`
- **Query Parameters:**
  - `language` (Optional): Filter by language chip (e.g. `Hindi`, `Bhojpuri`, `Bengali`, `Tamil`, `Urdu`, `English`) or `All`.
  - `search` (Optional): Search listener by name.
- **Example Request:** `GET https://api.truelineapp.in/api/v1/partners?language=Hindi&search=Afreen`
- **Response (200 OK):**
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
      "availability": "online", // "online" (green badge) or "offline"
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

## 5. 1-on-1 Chat System (Chat Tab & Chat Room)

### 5.1 List All Chat Conversations Screen
Populates the Chat Tab screen listing all active listener conversations with partner avatar, name, title, online badge, last message snippet, timestamp, and unread badge count.

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

### 5.2 Get 1-on-1 Chat Room Messages
Fetches message history for a specific listener chat room. **Automatically marks unread messages from the partner as read.**

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
      "sender_type": "user",
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

### 5.3 Send Message in 1-on-1 Chat Room
Send a text message to a listener.

- **Endpoint:** `POST /api/v1/chats/{partner_id}/messages`
- **Auth:** `Bearer <USER_JWT_TOKEN>`
- **Request Body:**
```json
{
  "content": "Hello Afreen, I will call you in 5 minutes!"
}
```
- **Response (200 OK):**
```json
{
  "success": true,
  "data": {
    "id": "e3c4d5e6-7890-123a-bcde-f33333333333",
    "user_id": "11111111-1111-1111-1111-111111111111",
    "partner_id": "a0000000-0000-0000-0000-000000000001",
    "sender_type": "user",
    "content": "Hello Afreen, I will call you in 5 minutes!",
    "read_at": null,
    "created_at": "2026-08-11T20:50:00Z"
  }
}
```

### 5.4 Mark Conversation as Read
- **Endpoint:** `POST /api/v1/chats/{partner_id}/read`
- **Auth:** `Bearer <USER_JWT_TOKEN>`
- **Response (200 OK):**
```json
{
  "success": true,
  "data": {
    "message": "Messages marked as read"
  }
}
```
