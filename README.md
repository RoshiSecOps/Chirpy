# Chirpy

Chirpy is a simple social media backend server written in Go. It allows users to create accounts, authenticate, and share "chirps" (short posts).

## Features

- **User Management**: Create and update user profiles.
- **Authentication**: Secure login with JWT access tokens and refresh tokens.
- **Chirps**: Create, retrieve, and delete chirps.
- **Admin Tools**: Monitor server metrics and reset the database during development.
- **Webhooks**: Integration for Polka premium upgrades.

## Getting Started

1. **Clone the repository**
   ```bash git clone https://github.com/RoshiSecOps/chirpy.git```
2. **Run the server**
   ```bash go build -o chirpy && ./chirpy```


### API Documentation

| Category | Method | Endpoint | Description | Auth |
| :--- | :--- | :--- | :--- | :--- |
| **Users** | `POST` | `/api/users` | Create a new user account | No |
| | `POST` | `/api/login` | Authenticate user and receive tokens | No |
| | `PUT` | `/api/users` | Update authenticated user info | **Yes** |
| | `POST` | `/api/refresh` | Refresh an expired access token | **Yes** |
| | `POST` | `/api/revoke` | Revoke a refresh token | **Yes** |
| **Chirps** | `GET` | `/api/chirps` | Retrieve all chirps | No |
| | `GET` | `/api/chirps/{id}` | Retrieve a specific chirp by ID | No |
| | `POST` | `/api/chirps` | Create a new chirp | **Yes** |
| | `DELETE` | `/api/chirps/{id}` | Delete a specific chirp | **Yes** |
| **Admin** | `GET` | `/admin/metrics` | View server usage statistics | No |
| | `POST` | `/admin/reset` | Reset database state (Dev only) | No |
| **Webhooks**| `POST` | `/api/polka/webhooks` | Handle Polka premium upgrades | **Yes** |


```bash

curl -X POST http://localhost:8080/api/users \
  -H "Content-Type: application/json" \
  -d '{
    "email": "wizard@boot.dev",
    "password": "password123"
  }'

Login

curl -X POST http://localhost:8080/api/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "wizard@boot.dev",
    "password": "password123"
  }'

Refresh Token

curl -X POST http://localhost:8080/api/refresh \
  -H "Authorization: Bearer <REFRESH_TOKEN>"

Chirps
Create Chirp

curl -X POST http://localhost:8080/api/chirps \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <ACCESS_TOKEN>" \
  -d '{
    "body": "This is a great day to learn Go!"
  }'

Get Specific Chirp

curl -X GET http://localhost:8080/api/chirps/123e4567-e89b-12d3-a456-426614174000

Delete Chirp

curl -X DELETE http://localhost:8080/api/chirps/123e4567-e89b-12d3-a456-426614174000 \
  -H "Authorization: Bearer <ACCESS_TOKEN>"

Admin & Webhooks
Reset Database

curl -X POST http://localhost:8080/admin/reset

Polka Webhook (Upgrade User)

curl -X POST http://localhost:8080/api/polka/webhooks \
  -H "Content-Type: application/json" \
  -H "Authorization: ApiKey <YOUR_POLKA_KEY>" \
  -d '{
    "event": "user.upgraded",
    "data": {
      "user_id": "123e4567-e89b-12d3-a456-426614174000"
    }
  }'

  ```