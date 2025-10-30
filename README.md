# CryptaChat Server

CryptaChat Server is a simple, secure backend for an End-to-End Encrypted (E2EE) chat application. It is built with Python and Flask.

The server's primary role is *not* to see unencrypted messages, but to manage user accounts, store public keys, and securely relay encrypted message blobs between authenticated clients.

## Features

* **User Authentication**: Secure user registration and login using JWT (JSON Web Tokens).
* **Public Key Storage**: Users can upload their public keys, which other users can fetch to initiate an E2EE session.
* **Contact Management**: A chat request system (`pending`, `accepted`) ensures users must mutually agree to communicate.
* **Secure Message Relay**: The server stores encrypted blobs for both the sender and recipient, but never has access to the plaintext keys or messages.
* **Message Polling**: Clients can fetch new messages since their last poll using a `since_id` parameter.
* **Rate Limiting**: Basic per-IP rate limiting is enforced on sensitive endpoints (like login, register, and send_message) to prevent spam and abuse.

## Technology Stack

* **Backend**: Python 3, Flask
* **Database**: SQLite
* **Authentication**: PyJWT
* **Password Hashing**: Werkzeug
* **Security**: `flask-limiter` for rate limiting
* **Environment**: Conda
* **Containerization**: Docker

## How to Run (Docker - Recommended)

This is the easiest way to get the server running with a persistent database.

1.  **Save the Code**: Make sure all the updated files (`Dockerfile`, `requirements.txt`, `server/server.py`, `docker-compose.yml`) are saved in your project directory.
2.  **Start Docker Desktop**: Ensure the Docker Desktop application is running.
3.  **Build and Run**: From your terminal, run:
    ```bash
    docker compose up --build
    ```
4.  The server will build the image, start, initialize the `chat_server.db` file within the Docker volume, and be accessible at `http://localhost:5000`.

## How to Run (Manual/Local Development)

1.  **Create Conda Environment**:
    ```bash
    conda env create -f environment.yml
    ```
2.  **Activate Environment**:
    ```bash
    conda activate venv
    ```
3.  **Install Pip Dependencies**:
    ```bash
    pip install -r requirements.txt
    ```
4.  **Run the Server**:
    ```bash
    python server/server.py
    ```
    The server will create a local `chat_server.db` file and start on `http://127.0.0.1:5000`.

## API Endpoints

All protected routes require an `Authorization: Bearer <token>` header. Rate limits are applied per IP.

* `POST /register`: Register a new user. (Strictly limited)
* `POST /login`: Log in and receive a JWT. (Strictly limited)
* `POST /upload_key` (Protected): Upload/update your public key.
* `GET /get_key` (Protected): Get the public key for a specified username.
* `POST /request_chat` (Protected): Send a chat request to another user. (Limited)
* `GET /get_chat_requests` (Protected): Get your pending incoming chat requests.
* `POST /accept_chat` (Protected): Accept a pending chat request. (Limited)
* `GET /get_contacts` (Protected): Get a list of all accepted chat partners.
* `POST /send_message` (Protected): Send an encrypted message blob to a user. (Limited)
* `GET /get_messages` (Protected): Fetch messages from a user, with an optional `since_id` query param.

## Admin Client

The repository includes a Tkinter-based GUI admin tool (`server/admin_client.py`). This is a separate desktop application you can run locally to directly inspect and manage the `chat_server.db` database file, whether it's running locally or in the Docker volume.