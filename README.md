# CryptaChat Server

CryptaChat Server is a simple, secure backend for an End-to-End Encrypted (E2EE) chat application, built in Go.

The server's primary role is *not* to see unencrypted messages, but to manage user accounts, store public keys, and securely relay encrypted message blobs between authenticated clients.

## Features

* **User Authentication**: Secure user registration and login using JWT (JSON Web Tokens).
* **Public Key Storage**: Users can upload their public keys, which other users can fetch to initiate an E2EE session.
* **Contact Management**: A chat request system (`pending`, `accepted`) ensures users must mutually agree to communicate.
* **Secure Message Relay**: The server stores encrypted blobs for both the sender and recipient, but never has access to the plaintext keys or messages.
* **Message Polling**: Clients can fetch new messages since their last poll using a `since_id` parameter.

## Technology Stack

* **Backend**: Go (Golang)
* **Database**: **PostgreSQL**
* **Router**: Standard Library (`net/http`)
* **Authentication**: `github.com/golang-jwt/jwt/v5`
* **Password Hashing**: `golang.org/x/crypto/bcrypt`
* **Containerization**: Docker

## How to Run (Docker - Recommended)

This is the easiest way to get the server running with a persistent database.

1.  **Configure Environment**: The `.config/docker.env` file holds the database credentials. **WARNING:** For production, you must change the default `POSTGRES_USER` and `POSTGRES_PASSWORD`.
2.  **Start Docker Desktop**: Ensure the Docker Desktop application is running.
3.  **Build and Run**: From your terminal, run:
    ```bash
    docker compose up --build
    ```
4.  The server will build the Go binary, start, initialize the **PostgreSQL database** within the persistent Docker volume (`pgdata`), and be accessible at `http://localhost:5000`.

## How to Run (Manual/Local Development)

This method requires you to have **Go (1.21+)** installed and a **PostgreSQL database running** and accessible at the host/port specified in `.config/docker.env`.

1.  **Navigate to Source**:
    ```bash
    cd src
    ```
2.  **Install Dependencies**:
    ```bash
    go mod tidy
    ```
3.  **Run the Server**:
    ```bash
    go run ./main.go
    ```
4.  The server will attempt to connect to the PostgreSQL host specified in `.config/docker.env`, initialize the tables, and then start on `http://127.0.0.1:5000`.

## API Endpoints

All protected routes require an `Authorization: Bearer <token>` header.

* `POST /register`: Register a new user.
* `POST /login`: Log in and receive a JWT.
* `POST /upload_key` (Protected): Upload/update your public key.
* `GET /get_key` (Protected): Get the public key for a specified username.
* `POST /request_chat` (Protected): Send a chat request to another user.
* `GET /get_chat_requests` (Protected): Get your pending incoming chat requests.
* `POST /accept_chat` (Protected): Accept a pending chat request.
* `GET /get_contacts` (Protected): Get a list of all accepted chat partners.
* `POST /send_message` (Protected): Send an encrypted message blob to a user.
* `GET /get_messages` (Protected): Fetch messages from a user, with an optional `since_id` query param.