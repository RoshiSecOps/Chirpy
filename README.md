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
   ```bash
   git clone https://github.com/RoshiSecOps/chirpy.git

Run the server
go build -o chirpy && ./chirpy

API Documentation
Users
POST /api/users - Create a new user account.
POST /api/login - Authenticate a user and receive tokens.
PUT /api/users - Update authenticated user information.
POST /api/refresh - Refresh an expired access token.
POST /api/revoke - Revoke a refresh token.
Chirps
GET /api/chirps - Retrieve all chirps.
GET /api/chirps/{chirpID} - Retrieve a specific chirp by ID.
POST /api/chirps - Create a new chirp (requires authentication).
DELETE /api/chirps/{chirpID} - Delete a chirp (requires authentication).
Administration
GET /admin/metrics - View server usage statistics.
POST /admin/reset - Reset the database state (Development only).
Webhooks
POST /api/polka/webhooks - Handle account upgrades from Polka.
Example API Requests
Users & Authentication
Create User

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